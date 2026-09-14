"""Thin async client for the Go REST API.

Every write the interview makes goes through these functions, and every one of
them is an ordinary authenticated call to an endpoint a human or the frontend
could use just as well -- the agent has no other way to write career data (see
#117's acceptance criteria). The bearer token is passed explicitly on every
call rather than held on an object here, because the underlying httpx client
is shared across every session while the token is per-turn and per-user --
see the note on InterviewState about why it never lives in graph state either.
"""

from __future__ import annotations

import httpx


class ApiError(Exception):
    """A non-2xx response from the Go API."""

    def __init__(self, status_code: int, body: str) -> None:
        self.status_code = status_code
        self.body = body
        super().__init__(f"role-model API returned {status_code}: {body}")


async def _request(
    client: httpx.AsyncClient,
    token: str,
    method: str,
    path: str,
    json_body: dict | None = None,
) -> dict:
    response = await client.request(
        method,
        path,
        json=json_body,
        headers={"Authorization": f"Bearer {token}"},
    )
    if response.status_code >= 400:
        raise ApiError(response.status_code, response.text)
    if response.status_code == 204 or not response.content:
        return {}
    return response.json()


async def _list_request(client: httpx.AsyncClient, token: str, path: str) -> list[dict]:
    # Go's encoding/json serializes a nil slice as JSON null, not []: an
    # account with zero rows on a GET list endpoint returns a bare `null`
    # body, which httpx.Response.json() turns into Python None -- found by
    # driving a real interview against a real account with no tags yet, where
    # `for tag in None` raised TypeError rather than the loop simply not
    # running. Every list endpoint this client calls goes through here so
    # that quirk is normalized in exactly one place.
    result = await _request(client, token, "GET", path)
    return result or []


async def list_employers(client: httpx.AsyncClient, token: str) -> list[dict]:
    return await _list_request(client, token, "/employers")


async def create_employer(client: httpx.AsyncClient, token: str, name: str) -> dict:
    return await _request(
        client,
        token,
        "POST",
        "/employers",
        {"name": name, "industry": None, "notes": None},
    )


async def resolve_or_create_employer(
    client: httpx.AsyncClient, token: str, name: str
) -> tuple[dict, bool]:
    """Case-insensitive match against the account's own employers, else
    create. Returns (employer, created) so the caller can tell the person
    which happened -- the same resolve-then-create shape as PositionPicker and
    ResolveOrCreateTag on the Go side, voiced conversationally instead of
    clicked."""
    for employer in await list_employers(client, token):
        if employer["name"].strip().lower() == name.strip().lower():
            return employer, False
    return await create_employer(client, token, name), True


async def list_positions(
    client: httpx.AsyncClient, token: str, employer_id: str
) -> list[dict]:
    return await _list_request(client, token, f"/employers/{employer_id}/positions")


async def create_position(
    client: httpx.AsyncClient,
    token: str,
    employer_id: str,
    title: str,
    started_on: str,
    context_narrative: str | None = None,
) -> dict:
    return await _request(
        client,
        token,
        "POST",
        "/positions",
        {
            "employer_id": employer_id,
            "title": title,
            "industry_level": None,
            "industry_role": None,
            "location": None,
            "level_rationale": None,
            "started_on": started_on,
            "ended_on": None,
            "context_narrative": context_narrative,
            "sort_order": 0,
        },
    )


async def resolve_or_create_position(
    client: httpx.AsyncClient,
    token: str,
    employer_id: str,
    title: str,
    started_on: str,
    context_narrative: str | None = None,
) -> tuple[dict, bool]:
    """Case-insensitive match against this employer's own positions, else
    create -- the same resolve-then-create shape as resolve_or_create_employer.
    Without this, re-running the interview for the same job (or correcting a
    typo'd employer name by starting the job over) duplicated the position
    with no dedup at all."""
    for position in await list_positions(client, token, employer_id):
        if position["title"].strip().lower() == title.strip().lower():
            return position, False
    created = await create_position(
        client, token, employer_id, title, started_on, context_narrative
    )
    return created, True


async def create_contribution(
    client: httpx.AsyncClient,
    token: str,
    position_id: str,
    summary: str,
    full_description: str,
    outcomes: str | None = None,
    scale_context: str | None = None,
) -> dict:
    return await _request(
        client,
        token,
        "POST",
        "/contributions",
        {
            "position_id": position_id,
            "summary": summary,
            "full_description": full_description,
            "outcomes": outcomes,
            "scale_context": scale_context,
            "is_active": True,
        },
    )


async def list_tag_categories(client: httpx.AsyncClient, token: str) -> list[dict]:
    return await _list_request(client, token, "/tag-categories")


async def resolve_or_create_category(
    client: httpx.AsyncClient, token: str, name: str
) -> dict:
    for category in await list_tag_categories(client, token):
        if category["name"].strip().lower() == name.strip().lower():
            return category
    return await _request(
        client,
        token,
        "POST",
        "/tag-categories",
        {"name": name, "sort_order": None},
    )


async def list_tags(client: httpx.AsyncClient, token: str) -> list[dict]:
    return await _list_request(client, token, "/tags")


async def resolve_or_create_tag(
    client: httpx.AsyncClient, token: str, category: str, name: str
) -> dict:
    """Case-insensitive match on name, the same rule
    intake.ResolveOrCreateTag enforces on the Go side -- but done here from
    outside, because POST /tags (unlike POST /skills) 400s with
    invalid_category if the category doesn't already exist rather than
    creating it inline."""
    for tag in await list_tags(client, token):
        if tag["name"].strip().lower() == name.strip().lower():
            return tag
    resolved_category = await resolve_or_create_category(client, token, category)
    return await _request(
        client,
        token,
        "POST",
        "/tags",
        {
            "name": name,
            "category": resolved_category["name"],
            "aliases": [],
            "sort_order": None,
        },
    )


async def attach_tag_to_contribution(
    client: httpx.AsyncClient, token: str, contribution_id: str, tag_id: str
) -> None:
    await _request(
        client,
        token,
        "POST",
        f"/contributions/{contribution_id}/tags",
        {"tag_id": tag_id},
    )


async def list_proficiency_levels(client: httpx.AsyncClient, token: str) -> list[dict]:
    return await _list_request(client, token, "/vocabulary/proficiency-levels")


async def create_skill(
    client: httpx.AsyncClient,
    token: str,
    category: str,
    tag: str,
    proficiency: str,
    years_experience: float | None = None,
) -> dict:
    # Unlike /tags, POST /skills resolves-or-creates both the category and the
    # tag itself (it calls the same intake.ResolveOrCreateTag Go-side), so
    # there is no separate resolve step here.
    return await _request(
        client,
        token,
        "POST",
        "/skills",
        {
            "category": category,
            "tag": tag,
            "proficiency": proficiency,
            "years_experience": years_experience,
            "is_active": True,
        },
    )
