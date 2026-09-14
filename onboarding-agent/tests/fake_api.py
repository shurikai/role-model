"""A fake role-model REST API, standing in for the Go service via
httpx.MockTransport. Persists exactly what's posted and serves it back on the
matching GET -- close enough to the real handlers (including invalid_category
on POST /tags for a category that doesn't exist yet) to exercise
agent/tools.py's resolve-or-create logic without a real database, network
call, or Go process.
"""

from __future__ import annotations

import json
import uuid

import httpx


class FakeRoleModelAPI:
    def __init__(self) -> None:
        self.employers: list[dict] = []
        self.positions: list[dict] = []
        self.contributions: list[dict] = []
        self.tag_categories: list[dict] = []
        self.tags: list[dict] = []
        self.contribution_tags: list[tuple[str, str]] = []
        self.skills: list[dict] = []
        self.proficiency_levels: list[dict] = [
            {"value": "novice", "label": "Novice", "rank": 1},
            {"value": "proficient", "label": "Proficient", "rank": 2},
            {"value": "expert", "label": "Expert", "rank": 3},
        ]

    def client(self) -> httpx.AsyncClient:
        return httpx.AsyncClient(
            transport=httpx.MockTransport(self._handle), base_url="http://fake"
        )

    def _handle(self, request: httpx.Request) -> httpx.Response:
        method = request.method
        path = request.url.path
        body: dict = json.loads(request.content) if request.content else {}

        if method == "GET" and path == "/employers":
            return httpx.Response(200, json=self.employers)
        if method == "POST" and path == "/employers":
            return self._create(self.employers, body)

        if (
            method == "GET"
            and path.startswith("/employers/")
            and path.endswith("/positions")
        ):
            employer_id = path.split("/")[2]
            matches = [p for p in self.positions if p["employer_id"] == employer_id]
            return httpx.Response(200, json=matches)
        if method == "POST" and path == "/positions":
            return self._create(self.positions, body)

        if method == "POST" and path == "/contributions":
            return self._create(self.contributions, body)

        if method == "GET" and path == "/tag-categories":
            return httpx.Response(200, json=self.tag_categories)
        if method == "POST" and path == "/tag-categories":
            return self._create(self.tag_categories, body)

        if method == "GET" and path == "/tags":
            return httpx.Response(200, json=self.tags)
        if method == "POST" and path == "/tags":
            known = {c["name"] for c in self.tag_categories}
            if body.get("category") not in known:
                return httpx.Response(
                    400,
                    json={
                        "error": "category does not exist",
                        "code": "invalid_category",
                    },
                )
            return self._create(self.tags, body)

        if (
            method == "POST"
            and path.startswith("/contributions/")
            and path.endswith("/tags")
        ):
            contribution_id = path.split("/")[2]
            self.contribution_tags.append((contribution_id, body["tag_id"]))
            return httpx.Response(204)

        if method == "GET" and path == "/vocabulary/proficiency-levels":
            return httpx.Response(200, json=self.proficiency_levels)

        if method == "POST" and path == "/skills":
            return self._create(self.skills, body)

        return httpx.Response(404, json={"error": "not found", "code": "not_found"})

    @staticmethod
    def _create(store: list[dict], body: dict) -> httpx.Response:
        row = {"id": str(uuid.uuid4()), **body}
        store.append(row)
        return httpx.Response(201, json=row)
