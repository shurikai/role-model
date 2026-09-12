from __future__ import annotations

import pytest

from tests.fake_api import FakeRoleModelAPI


@pytest.fixture
def fake_api() -> FakeRoleModelAPI:
    return FakeRoleModelAPI()


class FakeTagModel:
    """A canned StructuredModel (agent.tags.StructuredModel) -- one response
    per call, in order, so a test can script exactly what "the LLM" proposes
    at each contribution without spending money or getting a different answer
    each run."""

    def __init__(self, responses: list) -> None:
        self._responses = list(responses)
        self.calls: list[list[dict]] = []

    async def ainvoke(self, messages: list[dict]):
        self.calls.append(messages)
        return self._responses.pop(0)
