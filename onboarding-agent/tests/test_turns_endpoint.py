"""The HTTP contract of /turns, against the same graph and fakes as
test_graph.py -- but through FastAPI, so it also pins the request/response
JSON shape internal/onboarding.Client depends on.

Uses TestClient WITHOUT the `with` context manager, which is what skips
main.py's real `lifespan` (a real SqliteSaver + a real ChatAnthropic
requiring an API key) -- confirmed empirically: TestClient only runs
lifespan when used as a context manager. app.state is populated directly
with fakes instead.
"""

from __future__ import annotations

from langgraph.checkpoint.memory import InMemorySaver

import main
from agent.graph import build_graph
from agent.tags import _Tag, _TagExtraction
from tests.conftest import FakeTagModel
from tests.fake_api import FakeRoleModelAPI


def _client(tag_responses: list) -> tuple:
    fake_api = FakeRoleModelAPI()
    tag_model = FakeTagModel(tag_responses)
    main.app.state.graph = build_graph(InMemorySaver())
    main.app.state.api_client = fake_api.client()
    main.app.state.tag_model = tag_model
    from fastapi.testclient import TestClient

    return TestClient(main.app), fake_api


def test_a_new_interview_needs_no_session_id_and_returns_one():
    client, _ = _client([_TagExtraction(tags=[])])

    resp = client.post("/turns", json={"token": "fake-jwt"})

    assert resp.status_code == 200
    body = resp.json()
    assert body["session_id"]
    assert body["done"] is False
    assert "company" in body["reply"].lower()


def test_resuming_a_session_carries_it_by_id_through_to_completion():
    client, fake_api = _client(
        [
            _TagExtraction(
                tags=[_Tag(category="Languages", name="Go", is_skill_candidate=True)]
            )
        ]
    )

    r1 = client.post("/turns", json={"token": "fake-jwt"}).json()
    session_id = r1["session_id"]

    r2 = client.post(
        "/turns",
        json={"token": "fake-jwt", "session_id": session_id, "message": "Acme Corp"},
    ).json()
    assert r2["session_id"] == session_id
    assert r2["done"] is False

    client.post(
        "/turns",
        json={"token": "fake-jwt", "session_id": session_id, "message": "Engineer"},
    ).json()
    client.post(
        "/turns",
        json={"token": "fake-jwt", "session_id": session_id, "message": "2020-01"},
    ).json()
    r4 = client.post(
        "/turns",
        json={
            "token": "fake-jwt",
            "session_id": session_id,
            "message": "Built the payments service in Go",
        },
    ).json()
    assert "go" in r4["reply"].lower()

    r5 = client.post(
        "/turns", json={"token": "fake-jwt", "session_id": session_id, "message": "yes"}
    ).json()
    assert "depth" in r5["reply"].lower()

    client.post(
        "/turns",
        json={
            "token": "fake-jwt",
            "session_id": session_id,
            "message": "expert, 4 years",
        },
    ).json()
    client.post(
        "/turns",
        json={"token": "fake-jwt", "session_id": session_id, "message": "done"},
    ).json()
    r8 = client.post(
        "/turns",
        json={"token": "fake-jwt", "session_id": session_id, "message": "done"},
    ).json()

    assert r8["done"] is True
    assert r8["session_id"] == session_id
    assert len(fake_api.contribution_tags) == 1


def test_a_missing_token_is_a_validation_error_not_a_500():
    client, _ = _client([_TagExtraction(tags=[])])

    resp = client.post("/turns", json={})

    assert resp.status_code == 422


def test_a_graph_exception_is_a_clean_502_not_a_raw_traceback():
    """New since sessions can be resumed after a page refresh: a session_id
    the checkpointer has no record of (a stale one, or one from before a
    checkpoint reset) has to fail cleanly rather than crash -- exercised here
    directly against a graph stand-in that always raises, since a real
    unrecognized-thread failure depends on LangGraph/checkpointer internals
    this suite otherwise never touches."""

    class _BrokenGraph:
        async def ainvoke(self, *args, **kwargs):
            raise RuntimeError("no checkpoint for this thread")

    client, _ = _client([_TagExtraction(tags=[])])
    main.app.state.graph = _BrokenGraph()

    resp = client.post("/turns", json={"token": "fake-jwt"})

    assert resp.status_code == 502
    assert "traceback" not in resp.text.lower()
    assert "RuntimeError" not in resp.text
