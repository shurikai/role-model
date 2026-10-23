"""The interview graph: employer -> position (title, start, end) ->
contributions -> tags/skills.

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
from datetime import date

from langgraph.graph import END, START, StateGraph
from langgraph.types import Checkpointer, RunnableConfig, interrupt

from agent import tools
from agent.state import InterviewState
from agent.tags import StructuredModel, extract_tag_candidates

_DONE_WORDS = {"done", "no", "nothing", "nope", "that's it", "thats it", "no more"}
_DONE_LEADING_WORDS = {"done", "no", "nope", "nothing"}


# The first clause break: sentence or clause punctuation, or a dash.
_CLAUSE_BREAK = re.compile(r"\s*(?:[.,;:!?]+|--|\u2014|\u2013)\s*")


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

    # A stop phrase that is its own first clause: "Done. We've covered
    # everything", "done, that is everything", "That's it -- thanks" (#163).
    # The whole-answer check above missed these because only the END of the
    # answer was stripped, and the leading-word check below saw "done." with
    # its period. A clause that goes on with "but" is still continuing:
    # "No, but I also rebuilt the scheduler" is a contribution, not a stop.
    brk = _CLAUSE_BREAK.search(normalized)
    if brk:
        first, rest = normalized[: brk.start()], normalized[brk.end() :]
        if first in _DONE_WORDS and not rest.startswith("but"):
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


# A year is 19xx or 20xx: bounding it keeps "1200 beds" or "4000 patients" in
# a free-text answer from reading as a date.
_ISO_DATE = re.compile(
    r"(?<![\d-])((?:19|20)\d{2})(?:-(\d{1,2})(?:-(\d{1,2}))?)?(?![\d-])"
)
_MONTH_NAME_DATE = re.compile(
    r"\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?,?\s+((?:19|20)\d{2})\b",
    re.IGNORECASE,
)
_MONTHS = {
    name: number
    for number, name in enumerate(
        [
            "jan",
            "feb",
            "mar",
            "apr",
            "may",
            "jun",
            "jul",
            "aug",
            "sep",
            "oct",
            "nov",
            "dec",
        ],
        start=1,
    )
}
# Trailing sentence punctuation is not part of the answer (#159): "2019-03."
# is a date someone typed and then ended their sentence.
_TRAILING_PUNCTUATION = " \t\n.,;:!?"


def _parse_date(answer: str) -> str | None:
    """The first date in the answer, as YYYY-MM-DD, or None if there is none.

    Accepts YYYY-MM-DD, YYYY-MM, a bare YYYY, and a month name with a year
    ("March 2019", "Mar. 2019"), normalizing the shorter forms to the 1st --
    the same tolerance jd_extraction.tmpl documents for dates elsewhere in
    this codebase ("Where the text gives only a year, use \"-01\" and let the
    reviewer correct it").

    It looks anywhere in the answer, not only at an answer that is nothing
    but a date (#158). "I was at Acme from 2015, then Globex 2019 to 2022"
    contains a start date for the job being asked about, and refusing it
    drove a real interview into the placeholder fallback. The first date is
    taken because the question names one job and people answer it first;
    the caller keeps the whole answer on the position whenever it was more
    than a date, so a reviewer can see what the date was read from.

    An impossible date ("2019-13") is skipped, not clamped.
    """
    candidates: list[tuple[int, str]] = []
    for m in _ISO_DATE.finditer(answer):
        year, month, day = m.group(1), m.group(2), m.group(3)
        iso = _valid_iso(int(year), int(month or 1), int(day or 1))
        if iso:
            candidates.append((m.start(), iso))
    for m in _MONTH_NAME_DATE.finditer(answer):
        iso = _valid_iso(int(m.group(2)), _MONTHS[m.group(1).lower()], 1)
        if iso:
            candidates.append((m.start(), iso))
    if not candidates:
        return None
    return min(candidates)[1]


def _valid_iso(year: int, month: int, day: int) -> str | None:
    try:
        return date(year, month, day).isoformat()
    except ValueError:
        return None


def _is_bare_date(answer: str) -> bool:
    """True when the answer is a date and nothing else, so there is nothing
    worth keeping beside the parsed value."""
    text = answer.strip().rstrip(_TRAILING_PUNCTUATION)
    return bool(_ISO_DATE.fullmatch(text) or _MONTH_NAME_DATE.fullmatch(text))


# Used only when a second reprompt still doesn't parse -- see ask_position_start
# and ask_position_end. A recognisably-fake date, not today's date or a null:
# this is what made the original bug (a silent 2000-01-01 for every position,
# always) invisible. For an end date, null is not even available as a
# fallback -- null means "still there", which is exactly the false claim #161
# was about.
_DATE_FALLBACK = "1900-01-01"


def _job_label(state: InterviewState) -> str:
    title = state.get("position_title", "").strip()
    employer = state.get("employer_name", "").strip()
    if title and employer:
        return f"{title} at {employer}"
    return title or employer or "that job"


async def ask_position_start(state: InterviewState) -> dict:
    # _position_start_needs_retry drives routing (route_position_start below)
    # and is set on EVERY return path here, unlike checking for
    # position_started_on's presence in state -- that key, once set for one
    # job, stays set for the rest of the thread, so presence alone can't tell
    # "this job's date is in" from "a PREVIOUS job's date is still sitting
    # there while this job's hasn't been asked yet".
    reprompt = state.get("_position_start_needs_retry", False)
    question = (
        "I couldn't find a date in that. One job at a time: when did you start "
        f"as {_job_label(state)}? (YYYY-MM, like 2019-03)"
        if reprompt
        else "When did you start there (YYYY-MM)?"
    )
    answer = interrupt(question)
    parsed = _parse_date(answer)
    if parsed is not None:
        return {
            "position_started_on": parsed,
            "position_started_on_note": (
                None
                if _is_bare_date(answer)
                else f'Start date read from: "{answer.strip()}"'
            ),
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
            "position_started_on": _DATE_FALLBACK,
            "position_started_on_note": (
                f'Start date as stated: "{answer.strip()}" (could not be read as a date)'
            ),
            "_position_start_needs_retry": False,
        }
    return {"_position_start_needs_retry": True}


def route_position_start(state: InterviewState) -> str:
    if state.get("_position_start_needs_retry"):
        return "ask_position_start"
    return "ask_position_end"


# Whole words, so "currently" counts and "concurrent" does not. Checked only
# when the answer holds no date: "I left in 2019, now at Globex" has a "now"
# in it and is still an end date.
_STILL_THERE = re.compile(
    r"\b(still|current|currently|present|now|ongoing|today)\b", re.IGNORECASE
)


async def ask_position_end(state: InterviewState) -> dict:
    """The end date (#161). The interview used to never ask, and sent every
    position with ended_on null -- which the career view and the renderer
    both read as "still there", so an interviewed career printed every job as
    held at once.

    An end before the start is not accepted: it is either a misread or an
    answer about a different job, and storing it would print a negative
    span. Comparison is at the precision the person gave, so "2019" ends a
    job that started 2019-03 rather than being refused as January.
    """
    reprompt = state.get("_position_end_needs_retry", False)
    started_on = state.get("position_started_on", _DATE_FALLBACK)
    question = (
        f"I couldn't read an end date for {_job_label(state)} in that. When did "
        "you leave? (YYYY-MM, or 'still there' if it's your current job)"
        if reprompt
        else "When did you leave (YYYY-MM)? Say 'still there' if it's your current job."
    )
    answer = interrupt(question)
    parsed = _parse_date(answer)
    if parsed is not None and not _ends_before(parsed, answer, started_on):
        return {
            "position_ended_on": max(parsed, started_on),
            "position_ended_on_note": (
                None
                if _is_bare_date(answer)
                else f'End date read from: "{answer.strip()}"'
            ),
            "_position_end_needs_retry": False,
        }
    if parsed is None and _STILL_THERE.search(answer):
        return {
            "position_ended_on": None,
            "position_ended_on_note": None,
            "_position_end_needs_retry": False,
        }
    if reprompt:
        return {
            "position_ended_on": _DATE_FALLBACK,
            "position_ended_on_note": (
                f'End date as stated: "{answer.strip()}" (could not be read as a date)'
            ),
            "_position_end_needs_retry": False,
        }
    return {"_position_end_needs_retry": True}


def _ends_before(ended_on: str, answer: str, started_on: str) -> bool:
    """Whether the end precedes the start at the precision the answer gave: a
    bare year compares as a year, a month as a month."""
    if started_on == _DATE_FALLBACK:
        return False
    bare = answer.strip().rstrip(_TRAILING_PUNCTUATION)
    if re.fullmatch(r"(?:19|20)\d{2}", bare):
        return ended_on[:4] < started_on[:4]
    if re.fullmatch(r"(?:19|20)\d{2}-\d{1,2}", bare) or _MONTH_NAME_DATE.fullmatch(
        bare
    ):
        return ended_on[:7] < started_on[:7]
    return ended_on < started_on


def route_position_end(state: InterviewState) -> str:
    if state.get("_position_end_needs_retry"):
        return "ask_position_end"
    return "resolve_position"


async def resolve_position(state: InterviewState, config: RunnableConfig) -> dict:
    conf = _configurable(config)
    notes = [
        n
        for n in (
            state.get("position_started_on_note"),
            state.get("position_ended_on_note"),
        )
        if n
    ]
    position, created = await tools.resolve_or_create_position(
        conf["api_client"],
        conf["token"],
        state["employer_id"],
        state["position_title"],
        state["position_started_on"],
        state.get("position_ended_on"),
        "\n".join(notes) or None,
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
            candidate_years = float(cleaned)
        except ValueError:
            continue
        # Found by onboardingeval's first real run: an answer like "I got my
        # ACLS in 2012" parsed "2012" as 2012.0 years of experience, which
        # skills.years_experience (NUMERIC(4,1)) can't even store -- POST
        # /skills 500'd on a numeric field overflow and took the whole
        # interview down with it. A calendar year is not a duration; nobody
        # has anywhere near 76 years of experience either way, so this is a
        # plausibility bound, not a narrow one.
        if not (0 <= candidate_years <= 75):
            continue
        years = candidate_years
        break
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
    builder.add_node("ask_position_end", ask_position_end)
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
        ["ask_position_start", "ask_position_end"],
    )
    builder.add_conditional_edges(
        "ask_position_end",
        route_position_end,
        ["ask_position_end", "resolve_position"],
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
