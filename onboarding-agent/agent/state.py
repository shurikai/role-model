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
    employer_id: NotRequired[str]
    position_title: NotRequired[str]
    position_started_on: NotRequired[str]
    position_id: NotRequired[str]

    # The contribution just recorded, and the tags proposed for it.
    contribution_id: NotRequired[str]
    contribution_text: NotRequired[str]
    tag_candidates: NotRequired[list[TagCandidate]]
    tag_candidate_index: NotRequired[int]
    confirmed_tag_ids: NotRequired[list[str]]
