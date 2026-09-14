"""FastAPI entry point for the onboarding agent (#117).

Internal-only by design (see docker-compose.yml, modelled on the renderer's
own security posture): nothing here validates the bearer token itself. The Go
API already gated this call behind RequireAuth before proxying it, and every
write this service makes is itself an authenticated call back to that same
API using the relayed token -- an invalid or expired token simply fails
there, the same 401 any other client of that API would get.

The token is a per-request field on TurnRequest, never a header here, and it
is never written into graph state -- see agent/state.py for why.
"""

from __future__ import annotations

import logging
import os
import uuid
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, HTTPException
from langchain_anthropic import ChatAnthropic
from langgraph.checkpoint.sqlite.aio import AsyncSqliteSaver
from langgraph.types import Command
from pydantic import BaseModel

from agent.graph import build_graph
from agent.tags import build_tag_extraction_model

logger = logging.getLogger(__name__)

ROLE_MODEL_API_URL = os.environ.get(
    "ROLE_MODEL_API_URL", "http://localhost:8080/api/v1"
)
CHECKPOINT_DB_PATH = os.environ.get(
    "CHECKPOINT_DB_PATH", "onboarding_checkpoints.sqlite"
)
# Sonnet 5. Configurable because a self-hosted operator may reasonably want a
# cheaper or newer model without a code change.
ANTHROPIC_MODEL = os.environ.get("ANTHROPIC_MODEL", "claude-sonnet-5")


@asynccontextmanager
async def lifespan(app: FastAPI):
    async with AsyncSqliteSaver.from_conn_string(CHECKPOINT_DB_PATH) as checkpointer:
        app.state.graph = build_graph(checkpointer)
        app.state.api_client = httpx.AsyncClient(
            base_url=ROLE_MODEL_API_URL, timeout=30.0
        )
        chat_model = ChatAnthropic(model=ANTHROPIC_MODEL)
        app.state.tag_model = build_tag_extraction_model(chat_model)
        try:
            yield
        finally:
            await app.state.api_client.aclose()


app = FastAPI(lifespan=lifespan)


class TurnRequest(BaseModel):
    # None on the very first call of a new interview.
    session_id: str | None = None
    message: str | None = None
    # The caller's own bearer token, relayed from the browser through the Go
    # proxy route. Used only to call the Go API back on the user's behalf.
    token: str


class TurnResponse(BaseModel):
    session_id: str
    reply: str
    done: bool


def _interrupt_value(result: dict[str, Any]) -> str | None:
    interrupts = result.get("__interrupt__")
    if not interrupts:
        return None
    return interrupts[0].value


@app.post("/turns", response_model=TurnResponse)
async def turns(req: TurnRequest) -> TurnResponse:
    session_id = req.session_id or str(uuid.uuid4())
    config = {
        "configurable": {
            "thread_id": session_id,
            "token": req.token,
            "api_client": app.state.api_client,
            "tag_model": app.state.tag_model,
        }
    }

    graph_input: dict[str, Any] | Command
    if req.session_id is None:
        # InterviewState.reply is the one required key; everything else is
        # NotRequired and filled in as the interview progresses.
        graph_input = {"reply": ""}
    else:
        graph_input = Command(resume=req.message or "")

    try:
        result = await app.state.graph.ainvoke(graph_input, config)
    except Exception:
        # Broad on purpose: this is the boundary between the graph and the
        # outside world, and anything that reaches here -- a node's own bug,
        # the Go API rejecting a write, or (new since sessions can now be
        # resumed after a page refresh) a session_id the checkpointer has no
        # record of at all -- must fail as a clean response, not an
        # unhandled-exception traceback leaking to the Go proxy and then to
        # the browser.
        logger.exception("turn failed for session %s", session_id)
        raise HTTPException(
            status_code=502, detail="the interview could not continue"
        ) from None

    interrupt_value = _interrupt_value(result)
    if interrupt_value is not None:
        return TurnResponse(session_id=session_id, reply=interrupt_value, done=False)
    return TurnResponse(session_id=session_id, reply=result.get("reply", ""), done=True)
