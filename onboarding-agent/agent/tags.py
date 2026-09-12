"""Turns a contribution's text into tag candidates.

Isolated in its own module, and taking the model as a parameter, for the same
reason internal/intake's Extractor interface exists on the Go side: the half
that talks to a model is untestable without spending money and getting a
different answer each time, and the half that decides what to do with the
response is where the bugs live. Tests inject a fake with a canned response;
nothing here is exercised against the real Anthropic API in CI.
"""

from __future__ import annotations

from typing import Protocol

from pydantic import BaseModel, Field

from agent.state import TagCandidate


class _Tag(BaseModel):
    category: str = Field(
        description="A grouping the person would recognise, e.g. 'Clinical', "
        "'Languages', 'Equipment'. Not one imported from another field."
    )
    name: str = Field(
        description="A specific nameable thing mentioned in the text, e.g. "
        "'ACLS', 'Python', 'sous vide'. Not a quality or a responsibility."
    )
    is_skill_candidate: bool = Field(
        description="True if this reads as a durable skill worth a "
        "proficiency/years follow-up question, false if it is a one-off "
        "mention that is still worth tagging as evidence but not asking "
        "'how many years' about."
    )


class _TagExtraction(BaseModel):
    tags: list[_Tag]


class StructuredModel(Protocol):
    """What extract_tag_candidates needs from an LLM: a chat model already
    bound to _TagExtraction via with_structured_output (see
    build_tag_extraction_model)."""

    async def ainvoke(self, messages: list[dict]) -> _TagExtraction: ...


TAG_EXTRACTION_PROMPT = (
    "You are reading one thing a person just told you they did at work. "
    "Propose the specific, nameable tags this text supports: tools, "
    "methods, systems, certifications, materials, or techniques. Not a "
    "quality ('hard-working'), not a responsibility ('patient care') -- "
    "those aren't tags, they belong in the sentence the person already "
    "said. Categories are groupings the person would recognise, not ones "
    "imported from another field. Return an empty list if the text "
    "supports no specific tag. Claim only what the text supports -- a tag "
    "you invent here is one the person will have to notice and delete."
)


async def extract_tag_candidates(
    model: StructuredModel, contribution_text: str
) -> list[TagCandidate]:
    result = await model.ainvoke(
        [
            {"role": "system", "content": TAG_EXTRACTION_PROMPT},
            {"role": "user", "content": contribution_text},
        ]
    )
    return [
        TagCandidate(
            category=t.category,
            name=t.name,
            is_skill_candidate=t.is_skill_candidate,
        )
        for t in result.tags
    ]


def build_tag_extraction_model(chat_model) -> StructuredModel:
    """Binds a raw ChatAnthropic instance to the structured schema. Kept
    separate from extract_tag_candidates so tests can skip this entirely and
    hand a fake StructuredModel straight to that function instead."""
    return chat_model.with_structured_output(_TagExtraction)
