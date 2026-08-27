# The DOCX renderer.
#
# A separate process by design: it never touches the database, and the JSON
# document it receives is self-contained. That is also why it has no
# credentials of any kind, and why it must not be reachable from outside the
# compose network — see the comment on its service in docker-compose.yml.

FROM ghcr.io/astral-sh/uv:python3.14-bookworm-slim AS build
WORKDIR /app

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_PYTHON_DOWNLOADS=never

# Dependencies before source, so editing main.py does not resolve the whole
# tree again. --frozen fails rather than silently updating the lockfile: a
# container build is not the place to discover a new transitive version.
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-install-project --no-dev

COPY . .
RUN uv sync --frozen --no-dev

FROM python:3.14-slim-bookworm
WORKDIR /app

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin renderer

COPY --from=build --chown=renderer:renderer /app /app

# The venv's bin first, so `uvicorn` and `python` resolve to the built
# environment without needing `uv run` at runtime.
ENV PATH="/app/.venv/bin:$PATH" \
    PYTHONUNBUFFERED=1

USER renderer
EXPOSE 8000

# One worker. Rendering is CPU-bound and short, the service is stateless, and
# a self-hosted instance serves one person — concurrency here would buy
# nothing and cost memory per worker.
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]
