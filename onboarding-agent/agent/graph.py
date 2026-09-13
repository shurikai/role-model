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

import re

from langgraph.graph import END, START, StateGraph
from langgraph.types import Checkpointer, RunnableConfig, interrupt

from agent import tools
from agent.state import InterviewState
from agent.tags import StructuredModel, extract_tag_candidates

_DONE_WORDS = {"done", "no", "nothing", "nope", "that's it", "thats it", "no more"}
_DONE_LEADING_WORDS = {"done", "no", "nope", "nothing"}


def _sounds_done(answer: str) -> bool:
    """Whole-utterance, not substring -- the same lesson the fit gate's
    matcher already learned (internal/fitgate): a bare `"done" in normalized`
    here misread "I made sure the migration was done before the cutover" as
    "no more contributions" and silently truncated the interview.

    This is a partial fix, not a complete one. "No new features, but I
    optimized X" genuinely starts with "no" and there is no simple heuristic
    that resolves that ambiguity -- what this closes is the common,
    unambiguous case: the stop word appearing mid-answer rather than as the
    answer.
    """
    normalized = answer.strip().lower().rstrip(".!")
    if normalized in _DONE_WORDS or normalized.startswith("next job"):
        return True
    leading_word = normalized.split(" ", 1)[0] if normalized else ""
    return leading_word in _DONE_LEADING_WORDS


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


async def ask_position_title(state: InterviewState) -> dict:
    prior_note = state.get("reply", "")
    answer = interrupt(f"{prior_note} What was your title there?")
    return {"position_title": answer}


_YEAR_MONTH_DAY = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_YEAR_MONTH = re.compile(r"^\d{4}-\d{2}$")
_YEAR_ONLY = re.compile(r"^\d{4}$")


def _parse_started_on(answer: str) -> str | None:
    """Accepts YYYY-MM-DD, YYYY-MM, or a bare YYYY, normalizing the latter two
    to a full date -- the same tolerance jd_extraction.tmpl documents for
    dates elsewhere in this codebase ("Where the text gives only a year, use
    \"-01\" and let the reviewer correct it"). Returns None for anything else,
    rather than guessing; the caller re-asks once on None."""
    text = answer.strip()
    if _YEAR_MONTH_DAY.match(text):
        return text
    if _YEAR_MONTH.match(text):
        return f"{text}-01"
    if _YEAR_ONLY.match(text):
        return f"{text}-01-01"
    return None


# Used only when a second reprompt still doesn't parse -- see ask_position_start.
# A recognisably-fake date, not today's date or a null: this is what made the
# original bug (a silent 2000-01-01 for every position, always) invisible.
_STARTED_ON_FALLBACK = "1900-01-01"


async def ask_position_start(state: InterviewState) -> dict:
    # _position_start_needs_retry drives routing (route_position_start below)
    # and is set on EVERY return path here, unlike checking for
    # position_started_on's presence in state -- that key, once set for one
    # job, stays set for the rest of the thread, so presence alone can't tell
    # "this job's date is in" from "a PREVIOUS job's date is still sitting
    # there while this job's hasn't been asked yet".
    reprompt = state.get("_position_start_needs_retry", False)
    question = (
        "I couldn't read that as a date -- try YYYY-MM, like 2019-03."
        if reprompt
        else "When did you start there (YYYY-MM)?"
    )
    answer = interrupt(question)
    parsed = _parse_started_on(answer)
    if parsed is not None:
        return {
            "position_started_on": parsed,
            "position_started_on_note": None,
            "_position_start_needs_retry": False,
        }
    if reprompt:
        # Second bad answer in a row: fall back rather than loop forever --
        # losing a clean date is the safe direction, the same rule skill-depth
        # parsing already follows. The raw answer is kept in
        # position_started_on_note (-> context_narrative) so it isn't silently
        # discarded the way the original combined-question version discarded
        # the date entirely, with nothing left for a human to correct later.
        return {
            "position_started_on": _STARTED_ON_FALLBACK,
            "position_started_on_note": answer,
            "_position_start_needs_retry": False,
        }
    return {"_position_start_needs_retry": True}


def route_position_start(state: InterviewState) -> str:
    if state.get("_position_start_needs_retry"):
        return "ask_position_start"
    return "resolve_position"


async def resolve_position(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    date_note = state.get("position_started_on_note")
    context_narrative = (
        f'Start date as stated: "{date_note}" (could not be read as a date)'
        if date_note
        else None
    )
    position, created = await tools.resolve_or_create_position(
        conf["api_client"],
        conf["token"],
        state["employer_id"],
        state["position_title"],
        state["position_started_on"],
        context_narrative,
    )
    note = (
        f"Got it -- adding {state['position_title']}."
        if created
        else f"Found {position['title']} at this employer already on file."
    )
    return {"position_id": position["id"], "reply": note}


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
        # removesuffix, not rstrip: rstrip("years") strips any trailing RUN
        # of the characters y/e/a/r/s, not the substring "years" -- it
        # happened to produce right answers here purely because those
        # characters overlap with the unit words, not because it was doing
        # the right thing.
        cleaned = (
            token.removesuffix("years")
            .removesuffix("yrs")
            .removesuffix("year")
            .removesuffix("yr")
        )
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
    builder.add_node("ask_position_title", ask_position_title)
    builder.add_node("ask_position_start", ask_position_start)
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
    builder.add_edge("resolve_employer", "ask_position_title")
    builder.add_edge("ask_position_title", "ask_position_start")
    builder.add_conditional_edges(
        "ask_position_start",
        route_position_start,
        ["ask_position_start", "resolve_position"],
    )
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
