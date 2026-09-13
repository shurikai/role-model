# The onboarding agent (#117).
#
# Internal-only, the same security posture as docx-renderer: it authenticates
# nothing itself, relying entirely on the Go API having already gated the
# call before proxying it here. It must not be reachable from outside the
# compose network -- see the comment on its service in docker-compose.yml.

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

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin onboarding

COPY --from=build --chown=onboarding:onboarding /app /app

# The venv's bin first, so `uvicorn` and `python` resolve to the built
# environment without needing `uv run` at runtime.
ENV PATH="/app/.venv/bin:$PATH" \
    PYTHONUNBUFFERED=1

# The SQLite checkpoint file lives here so an in-progress interview survives
# a container recreation -- see CHECKPOINT_DB_PATH and the volume mount on
# this service in docker-compose.yml.
RUN mkdir -p /data && chown onboarding:onboarding /data
ENV CHECKPOINT_DB_PATH=/data/onboarding_checkpoints.sqlite

USER onboarding
EXPOSE 8100

# One worker, same reasoning as the renderer: a self-hosted instance serves
# one person, and this service holds an in-process SQLite connection that a
# second worker would contend with rather than share.
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8100", "--workers", "1"]
