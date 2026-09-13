"""The interview graph: employer -> position -> contributions -> tags/skills.

One user-visible turn is one `ainvoke` call: LangGraph runs nodes until the
next `interrupt()` (or the graph ends) and returns in one round trip, so a
node that resolves an employer/position/contribution and then asks the next
question happens inside a single turn from the caller's point of view.

Each loop (another contribution? another job? another tag to confirm?) is its
own node with exactly one `interrupt()` call, looped back to itself via a
conditional edge, rather than a Python loop with several `interrupt()` calls
inside one node. `interrupt()`'s own docs are explicit that a node re-runs
from the top on every resume -- looping via edges instead keeps every node
idempotent to re-entry and keeps the API-writing side effects (creating a
tag, attaching it) each running exactly once.

Nothing here talks to Anthropic or the Go API directly except through
`agent.tools` and the `StructuredModel` passed in via config -- see
agent/tags.py for why that boundary exists.
"""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph
from langgraph.types import Checkpointer, RunnableConfig, interrupt

from agent import tools
from agent.state import InterviewState
from agent.tags import StructuredModel, extract_tag_candidates

_DONE_WORDS = {"done", "no", "nothing", "nope", "that's it", "thats it", "no more"}


def _sounds_done(answer: str) -> bool:
    normalized = answer.strip().lower().rstrip(".!")
    return normalized in _DONE_WORDS or "done" in normalized or "next job" in normalized


def _configurable(config: RunnableConfig) -> dict:
    return config["configurable"]


async def ask_employer(state: InterviewState) -> dict:
    answer = interrupt("What company did you work at?")
    return {"employer_name": answer}


async def resolve_employer(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    employer, created = await tools.resolve_or_create_employer(
        conf["api_client"], conf["token"], state["employer_name"]
    )
    note = (
        f"I don't have {employer['name']} on file yet -- adding it."
        if created
        else f"Found {employer['name']} already on file."
    )
    return {"employer_id": employer["id"], "reply": note}


async def ask_position(state: InterviewState) -> dict:
    prior_note = state.get("reply", "")
    answer = interrupt(
        f"{prior_note} What was your title there, and when did you start (YYYY-MM)?"
    )
    return {"position_title": answer}


async def resolve_position(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    # v1 asks title and start date together in one free-text answer; a real
    # parse of "Staff Engineer, 2019-03" belongs in a dedicated parsing pass,
    # not hand-rolled here. For now the whole answer becomes the title and the
    # start date is left for the person to correct later in the UI, the same
    # way a Stage 0 draft is corrected rather than trusted verbatim.
    position = await tools.create_position(
        conf["api_client"],
        conf["token"],
        state["employer_id"],
        state["position_title"],
        conf.get("default_started_on", "2000-01-01"),
    )
    return {"position_id": position["id"]}


async def ask_contribution(state: InterviewState) -> dict:
    answer = interrupt("Tell me about one thing you did in that role.")
    return {"contribution_text": answer}


async def record_contribution(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    contribution = await tools.create_contribution(
        conf["api_client"],
        conf["token"],
        state["position_id"],
        summary=state["contribution_text"][:200],
        full_description=state["contribution_text"],
    )
    return {"contribution_id": contribution["id"]}


async def extract_tags(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    model: StructuredModel = conf["tag_model"]
    candidates = await extract_tag_candidates(model, state["contribution_text"])
    return {
        "tag_candidates": candidates,
        "tag_candidate_index": 0,
        "confirmed_tag_ids": [],
    }


def route_tag_confirmation(state: InterviewState) -> str:
    candidates = state.get("tag_candidates", [])
    index = state.get("tag_candidate_index", 0)
    return "confirm_next_tag" if index < len(candidates) else "ask_more_contributions"


async def confirm_next_tag(state: InterviewState) -> dict:
    candidate = state["tag_candidates"][state["tag_candidate_index"]]
    answer = interrupt(
        f'You mentioned "{candidate["name"]}" ({candidate["category"]}) -- add that?'
    )
    return {"reply": answer}


def route_tag_answer(state: InterviewState) -> str:
    answer = state.get("reply", "").strip().lower()
    said_yes = answer.startswith(("y", "sure", "yes"))
    if not said_yes:
        return "advance_tag_index"
    candidate = state["tag_candidates"][state["tag_candidate_index"]]
    return "ask_skill_depth" if candidate["is_skill_candidate"] else "attach_tag"


async def attach_tag(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    candidate = state["tag_candidates"][state["tag_candidate_index"]]
    # This is the invariant the issue asks for: a tag only ever lands here
    # attached to the contribution that was just recorded, never floating
    # free. v_skill_provenance derives skill evidence from exactly this link.
    tag = await tools.resolve_or_create_tag(
        conf["api_client"], conf["token"], candidate["category"], candidate["name"]
    )
    await tools.attach_tag_to_contribution(
        conf["api_client"], conf["token"], state["contribution_id"], tag["id"]
    )
    return {"confirmed_tag_ids": [*state.get("confirmed_tag_ids", []), tag["id"]]}


async def ask_skill_depth(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    candidate = state["tag_candidates"][state["tag_candidate_index"]]
    levels = await tools.list_proficiency_levels(conf["api_client"], conf["token"])
    values = (
        ", ".join(level["value"] for level in levels) or "novice, proficient, expert"
    )
    answer = interrupt(
        f'How would you rate your depth with "{candidate["name"]}" ({values})? '
        "Years of experience too, if you know."
    )
    return {"reply": answer}


async def record_skill(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    candidate = state["tag_candidates"][state["tag_candidate_index"]]
    tag = await tools.resolve_or_create_tag(
        conf["api_client"], conf["token"], candidate["category"], candidate["name"]
    )
    await tools.attach_tag_to_contribution(
        conf["api_client"], conf["token"], state["contribution_id"], tag["id"]
    )

    levels = await tools.list_proficiency_levels(conf["api_client"], conf["token"])
    proficiency, years = _parse_depth_answer(state.get("reply", ""), levels)
    # Losing the signal is the safe direction here, same rule the fit gate's
    # skill_levels extraction follows: a depth we can't confidently read off
    # free text is not asserted. The tag attachment above still stands as
    # evidence either way.
    if proficiency is not None:
        await tools.create_skill(
            conf["api_client"],
            conf["token"],
            candidate["category"],
            candidate["name"],
            proficiency,
            years,
        )
    return {"confirmed_tag_ids": [*state.get("confirmed_tag_ids", []), tag["id"]]}


def _parse_depth_answer(
    answer: str, levels: list[dict]
) -> tuple[str | None, float | None]:
    normalized = answer.lower()
    values = [level["value"] for level in levels] or ["novice", "proficient", "expert"]
    proficiency = next((v for v in values if v.lower() in normalized), None)

    years: float | None = None
    for token in normalized.replace("+", " ").split():
        cleaned = token.rstrip("years").rstrip("yrs").rstrip("year").rstrip("yr")
        try:
            years = float(cleaned)
            break
        except ValueError:
            continue
    return proficiency, years


async def advance_tag_index(state: InterviewState) -> dict:
    return {"tag_candidate_index": state.get("tag_candidate_index", 0) + 1}


async def ask_more_contributions(state: InterviewState) -> dict:
    answer = interrupt(
        "Anything else you did in that role? Tell me about it, or say "
        "'next job' / 'done' to move on."
    )
    return {"reply": answer}


def route_more_contributions(state: InterviewState) -> str:
    answer = state.get("reply", "")
    if _sounds_done(answer):
        return "ask_more_employers"
    return "record_contribution_again"


async def record_contribution_again(state: InterviewState) -> dict:
    # The free-text answer to "anything else?" IS the next contribution, so
    # this reuses record_contribution's own logic on it directly rather than
    # bouncing back through another "tell me about it" question.
    return {"contribution_text": state["reply"]}


async def ask_more_employers(state: InterviewState) -> dict:
    answer = interrupt(
        "Any other jobs to add? Tell me the company, or say 'done' if that's "
        "everything for now."
    )
    return {"reply": answer}


def route_more_employers(state: InterviewState) -> str:
    return (
        "finish" if _sounds_done(state.get("reply", "")) else "resolve_employer_again"
    )


async def resolve_employer_again(state: InterviewState) -> dict:
    return {"employer_name": state["reply"]}


async def finish(state: InterviewState) -> dict:
    return {
        "reply": "Thanks -- I've got everything for now. You can review what was added on your Career page."
    }


def build_graph(checkpointer: Checkpointer):
    builder = StateGraph(InterviewState)

    builder.add_node("ask_employer", ask_employer)
    builder.add_node("resolve_employer", resolve_employer)
    builder.add_node("ask_position", ask_position)
    builder.add_node("resolve_position", resolve_position)
    builder.add_node("ask_contribution", ask_contribution)
    builder.add_node("record_contribution", record_contribution)
    builder.add_node("extract_tags", extract_tags)
    builder.add_node("confirm_next_tag", confirm_next_tag)
    builder.add_node("attach_tag", attach_tag)
    builder.add_node("ask_skill_depth", ask_skill_depth)
    builder.add_node("record_skill", record_skill)
    builder.add_node("advance_tag_index", advance_tag_index)
    builder.add_node("ask_more_contributions", ask_more_contributions)
    builder.add_node("record_contribution_again", record_contribution_again)
    builder.add_node("ask_more_employers", ask_more_employers)
    builder.add_node("resolve_employer_again", resolve_employer_again)
    builder.add_node("finish", finish)

    builder.add_edge(START, "ask_employer")
    builder.add_edge("ask_employer", "resolve_employer")
    builder.add_edge("resolve_employer", "ask_position")
    builder.add_edge("ask_position", "resolve_position")
    builder.add_edge("resolve_position", "ask_contribution")
    builder.add_edge("ask_contribution", "record_contribution")
    builder.add_edge("record_contribution", "extract_tags")
    builder.add_conditional_edges(
        "extract_tags",
        route_tag_confirmation,
        ["confirm_next_tag", "ask_more_contributions"],
    )
    builder.add_conditional_edges(
        "confirm_next_tag",
        route_tag_answer,
        ["advance_tag_index", "ask_skill_depth", "attach_tag"],
    )
    builder.add_edge("attach_tag", "advance_tag_index")
    builder.add_edge("ask_skill_depth", "record_skill")
    builder.add_edge("record_skill", "advance_tag_index")
    builder.add_conditional_edges(
        "advance_tag_index",
        route_tag_confirmation,
        ["confirm_next_tag", "ask_more_contributions"],
    )
    builder.add_conditional_edges(
        "ask_more_contributions",
        route_more_contributions,
        ["record_contribution_again", "ask_more_employers"],
    )
    builder.add_edge("record_contribution_again", "record_contribution")
    builder.add_conditional_edges(
        "ask_more_employers",
        route_more_employers,
        ["resolve_employer_again", "finish"],
    )
    builder.add_edge("resolve_employer_again", "resolve_employer")
    builder.add_edge("finish", END)

    return builder.compile(checkpointer=checkpointer)
