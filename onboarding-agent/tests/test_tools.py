"""agent/tools.py against a transport that mimics Go's actual serialization
quirks, not just the well-behaved fake in fake_api.py.

Found by driving a real interview against a real, empty account: Go's
encoding/json serializes a nil slice as JSON `null`, not `[]`, so a GET on any
list endpoint for an account with zero rows returns a bare `null` body, which
httpx.Response.json() turns into Python None -- and `for tag in None` raised
TypeError rather than the loop simply not running.
"""

from __future__ import annotations

import httpx
import pytest

from agent import tools


def _null_body_client() -> httpx.AsyncClient:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200, content=b"null", headers={"content-type": "application/json"}
        )

    return httpx.AsyncClient(
        transport=httpx.MockTransport(handler), base_url="http://fake"
    )


@pytest.mark.parametrize(
    "list_fn",
    [
        tools.list_employers,
        tools.list_tag_categories,
        tools.list_tags,
        tools.list_proficiency_levels,
    ],
)
async def test_a_null_list_body_reads_as_empty_not_none(list_fn):
    client = _null_body_client()
    result = await list_fn(client, "fake-jwt")
    assert result == []


async def test_resolve_or_create_employer_creates_one_against_a_null_list_body():
    """The bug this file exists for was hit inside a resolve-or-create call,
    not a bare list call -- reproduce it at that level too."""

    created = []

    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, content=b"null")
        import json

        body = json.loads(request.content)
        row = {"id": "e1", **body}
        created.append(row)
        return httpx.Response(201, json=row)

    client = httpx.AsyncClient(
        transport=httpx.MockTransport(handler), base_url="http://fake"
    )

    employer, was_created = await tools.resolve_or_create_employer(
        client, "fake-jwt", "Acme Corp"
    )

    assert was_created is True
    assert employer["name"] == "Acme Corp"
    assert created == [employer]
