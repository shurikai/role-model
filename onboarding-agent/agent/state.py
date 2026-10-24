"""Graph state for the onboarding interview.

The bearer token is deliberately NOT a field here. LangGraph checkpoints this
dict to SQLite so an interview survives a service restart (see main.py), and a
live credential sitting in plaintext on disk is a problem this design avoids
by construction: the token is threaded per-turn through
``config["configurable"]["token"]`` instead of through state, so nothing
durable ever stores it. See README.md.
"""

from typing import NotRequired, TypedDict


class TagCandidate(TypedDict):
    """One tag `extract_tag_candidates` read out of a contribution, not yet
    confirmed by the person. Nothing is written to the API on the strength of
    this alone -- see confirm_next_tag in graph.py."""

    category: str
    name: str
    # Worth a follow-up proficiency/years question, versus a one-off mention
    # that is still evidence worth tagging but not a durable skill claim.
    is_skill_candidate: bool


class InterviewState(TypedDict):
    # What the current turn says back to the person. Set by whichever node
    # last ran before the graph paused (or finished).
    reply: str

    # The employer/position currently being discussed.
    employer_name: NotRequired[str]
    # Set only when the name answer read as a sentence twice and
    # employer_name fell back to a placeholder: the person's words, carried
    # into the employer's notes. The retry flag routes ask_employer back to
    # itself once and is reset on every path, like the date flags below. See
    # _reads_as_a_sentence in graph.py.
    employer_name_note: NotRequired[str | None]
    _employer_needs_retry: NotRequired[bool]
    employer_id: NotRequired[str]
    position_title: NotRequired[str]
    # The same pair for the title, whose note goes into context_narrative.
    position_title_note: NotRequired[str | None]
    _position_title_needs_retry: NotRequired[bool]
    position_started_on: NotRequired[str]
    # A line for the position's context_narrative, set when the date was
    # read out of a longer answer or couldn't be read at all and fell back
    # to a placeholder -- either way the person's own words, so a reviewer
    # can see what the date came from. See ask_position_start in graph.py.
    position_started_on_note: NotRequired[str | None]
    # Graph-control flag, not a fact about the position: routes
    # ask_position_start back to itself for exactly one reprompt. Reset
    # explicitly on every path through that node, since a NotRequired key
    # once set for one job otherwise stays set into the next.
    _position_start_needs_retry: NotRequired[bool]
    # None means "still there" -- a claim, not an absence, which is why an
    # unreadable end falls back to a placeholder date instead. The note and
    # the retry flag work exactly as the start date's do. See
    # ask_position_end in graph.py.
    position_ended_on: NotRequired[str | None]
    position_ended_on_note: NotRequired[str | None]
    _position_end_needs_retry: NotRequired[bool]
    position_id: NotRequired[str]

    # The contribution just recorded, and the tags proposed for it.
    contribution_id: NotRequired[str]
    contribution_text: NotRequired[str]
    tag_candidates: NotRequired[list[TagCandidate]]
    tag_candidate_index: NotRequired[int]
    confirmed_tag_ids: NotRequired[list[str]]
