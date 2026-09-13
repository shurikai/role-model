"""Node/routing logic, exercised against a fake Go API and a canned LLM
response -- no real Anthropic or Go API calls. The half that talks to a model
is untestable without spending money and getting a different answer each
time (see agent/tags.py's module docstring); this is the half that decides
what to do with the response, which is where the bugs live.
"""

from __future__ import annotations

import uuid

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.types import Command

from agent.graph import build_graph
from agent.tags import _Tag, _TagExtraction
from tests.conftest import FakeTagModel


def _config(thread_id: str, fake_api, tag_model) -> dict:
    return {
        "configurable": {
            "thread_id": thread_id,
            "token": "fake-jwt",
            "api_client": fake_api.client(),
            "tag_model": tag_model,
        }
    }


def _graph():
    return build_graph(InMemorySaver())


async def _drive(graph, config, *answers: str) -> dict:
    """Starts a new interview and resumes it once per answer given, returning
    the result of the LAST invoke -- the state after the final answer."""
    result = await graph.ainvoke({"reply": ""}, config)
    for answer in answers:
        result = await graph.ainvoke(Command(resume=answer), config)
    return result


def _interrupt_text(result: dict) -> str:
    return result["__interrupt__"][0].value


async def test_full_interview_attaches_a_confirmed_skill_to_its_contribution(fake_api):
    graph = _graph()
    tag_model = FakeTagModel(
        [
            _TagExtraction(
                tags=[
                    _Tag(
                        category="Databases", name="PostgreSQL", is_skill_candidate=True
                    )
                ]
            )
        ]
    )
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    r1 = await graph.ainvoke({"reply": ""}, config)
    assert "company" in _interrupt_text(r1).lower()

    r2 = await graph.ainvoke(Command(resume="Acme Corp"), config)
    assert "title" in _interrupt_text(r2).lower()
    assert "Acme Corp" in _interrupt_text(r2)  # the "adding it" note carries the name

    r3 = await graph.ainvoke(Command(resume="Senior Engineer"), config)
    assert "when did you start" in _interrupt_text(r3).lower()

    r3b = await graph.ainvoke(Command(resume="2019-01"), config)
    assert "tell me about" in _interrupt_text(r3b).lower()

    r4 = await graph.ainvoke(
        Command(resume="Migrated the billing system to Postgres"), config
    )
    assert "postgresql" in _interrupt_text(r4).lower()

    r5 = await graph.ainvoke(Command(resume="yes"), config)
    assert "depth" in _interrupt_text(r5).lower()

    r6 = await graph.ainvoke(Command(resume="expert, 5 years"), config)
    assert "anything else" in _interrupt_text(r6).lower()

    r7 = await graph.ainvoke(Command(resume="done"), config)
    assert "other jobs" in _interrupt_text(r7).lower()

    r8 = await graph.ainvoke(Command(resume="done"), config)
    assert "__interrupt__" not in r8
    assert "thanks" in r8["reply"].lower()

    assert len(fake_api.employers) == 1
    assert fake_api.employers[0]["name"] == "Acme Corp"
    assert len(fake_api.positions) == 1
    assert fake_api.positions[0]["employer_id"] == fake_api.employers[0]["id"]
    assert fake_api.positions[0]["title"] == "Senior Engineer"
    # The bug this hardening pass exists for: the date the person actually
    # gave used to be discarded entirely in favor of a hardcoded placeholder.
    assert fake_api.positions[0]["started_on"] == "2019-01-01"
    assert len(fake_api.contributions) == 1
    assert (
        fake_api.contributions[0]["full_description"]
        == "Migrated the billing system to Postgres"
    )

    assert len(fake_api.tags) == 1
    tag = fake_api.tags[0]
    assert tag["name"] == "PostgreSQL"
    assert tag["category"] == "Databases"

    # The invariant #117 asks for: the tag lives on the contribution that was
    # just recorded, not floating free.
    assert fake_api.contribution_tags == [(fake_api.contributions[0]["id"], tag["id"])]

    assert len(fake_api.skills) == 1
    assert fake_api.skills[0]["proficiency"] == "expert"
    assert fake_api.skills[0]["years_experience"] == 5.0


async def test_declining_a_tag_attaches_nothing(fake_api):
    graph = _graph()
    tag_model = FakeTagModel(
        [
            _TagExtraction(
                tags=[
                    _Tag(category="Databases", name="MongoDB", is_skill_candidate=True)
                ]
            )
        ]
    )
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph,
        config,
        "Acme Corp",
        "Engineer",
        "2020-01",
        "Wrote a script once that touched MongoDB",
        "no",  # declines the tag
    )

    assert "anything else" in _interrupt_text(result).lower()
    assert fake_api.tags == []
    assert fake_api.contribution_tags == []
    assert fake_api.skills == []


async def test_a_one_off_mention_is_tagged_without_a_skill_depth_question(fake_api):
    """is_skill_candidate=False still gets attached as evidence -- it just
    skips the proficiency/years follow-up, per agent/tags.py's contract."""
    graph = _graph()
    tag_model = FakeTagModel(
        [
            _TagExtraction(
                tags=[_Tag(category="Tools", name="Ansible", is_skill_candidate=False)]
            )
        ]
    )
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph,
        config,
        "Acme Corp",
        "Engineer",
        "2020-01",
        "Ran one playbook with Ansible for a one-time migration",
        "yes",
    )

    assert "anything else" in _interrupt_text(result).lower()
    assert len(fake_api.tags) == 1
    assert fake_api.contribution_tags == [
        (fake_api.contributions[0]["id"], fake_api.tags[0]["id"])
    ]
    assert fake_api.skills == []  # no depth question means no skill row


async def test_no_tag_candidates_skips_straight_to_the_next_question(fake_api):
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph,
        config,
        "Acme Corp",
        "Engineer",
        "2020-01",
        "Helped organize the team offsite",
    )

    assert "anything else" in _interrupt_text(result).lower()
    assert fake_api.tags == []
    assert fake_api.contribution_tags == []


async def test_a_second_contribution_gets_its_own_tag_extraction_call(fake_api):
    graph = _graph()
    tag_model = FakeTagModel(
        [
            _TagExtraction(tags=[]),
            _TagExtraction(
                tags=[_Tag(category="Languages", name="Go", is_skill_candidate=True)]
            ),
        ]
    )
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph,
        config,
        "Acme Corp",
        "Engineer",
        "2020-01",
        "Helped organize the team offsite",
        "Wrote the payments service in Go",  # answers "anything else?" directly
        "yes",
        "expert, 3 years",
    )

    assert "anything else" in _interrupt_text(result).lower()
    assert len(fake_api.contributions) == 2
    assert len(tag_model.calls) == 2
    assert len(fake_api.tags) == 1
    assert fake_api.tags[0]["name"] == "Go"
    # Attached to the SECOND contribution, not the first.
    assert fake_api.contribution_tags == [
        (fake_api.contributions[1]["id"], fake_api.tags[0]["id"])
    ]


async def test_a_second_employer_reuses_resolve_employer_and_starts_fresh(fake_api):
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[]), _TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph,
        config,
        "Acme Corp",
        "Engineer",
        "2020-01",
        "Helped organize the team offsite",
        "done",  # no more contributions at Acme
        "Widgets Inc",  # next employer, answers "any other jobs?" directly
        "Support Lead",
        "2022-01",
        "Ran the on-call rotation",
        "done",
        "done",
    )

    assert "__interrupt__" not in result
    assert len(fake_api.employers) == 2
    assert {e["name"] for e in fake_api.employers} == {"Acme Corp", "Widgets Inc"}
    assert len(fake_api.positions) == 2
    assert len(fake_api.contributions) == 2


async def test_an_existing_employer_is_matched_case_insensitively_not_duplicated(
    fake_api,
):
    fake_api.employers.append({"id": "existing-id", "name": "Acme Corp"})
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    result = await _drive(
        graph, config, "acme corp", "Engineer", "2020-01", "Did some work"
    )

    assert "anything else" in _interrupt_text(result).lower()
    assert len(fake_api.employers) == 1  # not duplicated
    assert fake_api.positions[0]["employer_id"] == "existing-id"


async def test_an_existing_position_at_the_same_employer_is_not_duplicated(fake_api):
    fake_api.employers.append({"id": "e1", "name": "Acme Corp"})
    fake_api.positions.append(
        {
            "id": "p1",
            "employer_id": "e1",
            "title": "Engineer",
            "started_on": "2020-01-01",
        }
    )
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    await _drive(graph, config, "Acme Corp", "engineer", "2020-01", "Did some work")

    assert len(fake_api.positions) == 1


@pytest.mark.parametrize(
    ("answer", "expected_started_on"),
    [
        ("2019-03-15", "2019-03-15"),
        ("2019-03", "2019-03-01"),
        ("2019", "2019-01-01"),
    ],
)
async def test_position_start_date_accepts_several_formats(
    fake_api, answer, expected_started_on
):
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    await _drive(graph, config, "Acme Corp", "Engineer", answer, "Did some work")

    assert fake_api.positions[0]["started_on"] == expected_started_on
    assert fake_api.positions[0]["context_narrative"] is None


async def test_an_unparseable_start_date_is_reprompted_once_then_falls_back(fake_api):
    """The bug this replaces: the free-text date was silently discarded and
    every position got the same hardcoded placeholder, with nothing kept for
    a human to fix later. This still falls back after a second bad answer,
    but keeps what the person actually said."""
    graph = _graph()
    tag_model = FakeTagModel([_TagExtraction(tags=[])])
    config = _config(str(uuid.uuid4()), fake_api, tag_model)

    await graph.ainvoke({"reply": ""}, config)
    await graph.ainvoke(Command(resume="Acme Corp"), config)
    await graph.ainvoke(Command(resume="Engineer"), config)
    bad_date_result = await graph.ainvoke(
        Command(resume="sometime last spring"), config
    )
    assert "couldn't read that as a date" in _interrupt_text(bad_date_result).lower()

    second_result = await graph.ainvoke(Command(resume="still not a date"), config)
    assert "tell me about" in _interrupt_text(second_result).lower()

    assert fake_api.positions[0]["started_on"] == "1900-01-01"
    assert "still not a date" in fake_api.positions[0]["context_narrative"]
