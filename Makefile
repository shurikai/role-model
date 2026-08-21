-include .env
export

# SEED_DIR ?= ../role-model-data/seed
# DATABASE_URL ?= postgres://rolemodel:rolemodel@localhost:5433/role_model?sslmode=disable

# Fictional sample dataset, tracked in this repo (see database/sample/README.md).
SAMPLE_DIR ?= database/sample

.PHONY: all build clean test db-up db-down db-reset db-dump migrate-up migrate-down migrate-down-all migrate-create seed seed-sample sqlc run run-frontend run-renderer dev check-prompts reset-password fmt fmt-check test-renderer

# Build
all: build

build:
	go build ./...

clean:
	go clean ./...

test:
	go test ./...

test-integration:
	go test -tags integration ./...

# The renderer is a separate process with its own toolchain, so its tests are
# a separate target rather than part of `make test`.
test-renderer:
	cd docx-renderer && uv run pytest

# Database
db-up:
	docker compose up -d

db-down:
	docker compose down

# Destroys the Postgres volume. Everything entered through the app — every
# application, jd_signals blob, resume version, and fit report — exists only
# there, so this is not recoverable from the seed repo.
db-reset: db-dump
	@echo "This DESTROYS the database volume behind $(DATABASE_URL)."
	@read -p "Type 'destroy' to continue: " confirm; \
	 [ "$$confirm" = "destroy" ] || { echo "Aborted."; exit 1; }
	docker compose down -v
	docker compose up -d

# Timestamped custom-format dump. Restore with:
#   pg_restore --clean --if-exists -d "$(DATABASE_URL)" backups/<file>.dump
#
# Worth running before anything that rewrites schema or bulk data. The seed
# repo covers career history; it does not cover applications, jd_signals,
# resume versions, fit reports, or anything added through the UI — including,
# as of 2026-08-19, seventeen skills rows that no seed file had ever carried.
db-dump:
	@mkdir -p backups
	@out="backups/role_model-$$(date +%Y%m%d-%H%M%S).dump"; \
	 pg_dump "$(DATABASE_URL)" -Fc -f "$$out" && echo "Wrote $$out"

# Migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

# One step, deliberately. `migrate ... down` with no count rolls back EVERY
# migration and empties the database, and its only guard is a y/n prompt that
# anything non-interactive sails straight through. That is what this target
# used to do, and on 2026-08-19 it destroyed a dev database holding eight
# applications and their generated resume versions — none of which live in the
# seed repo, because the seed repo is not a backup.
#
# Stepping back one migration is the operation anyone actually wants here. Pass
# STEPS=n for more.
STEPS ?= 1
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down $(STEPS)

# The full teardown, behind its own name and its own confirmation. Takes a
# dump first: the recovery path should not depend on remembering to.
migrate-down-all: db-dump
	@echo "This rolls back ALL migrations and empties $(DATABASE_URL)."
	@read -p "Type 'destroy' to continue: " confirm; \
	 [ "$$confirm" = "destroy" ] || { echo "Aborted."; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" down -all

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

seed:
	@echo "Seeding from $(SEED_DIR)..."
	@for f in $(SEED_DIR)/0*.sql; do \
		echo "  -> $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done
	@echo "Done."

# Fictional sample data, tracked in this repo. Deliberately a separate target
# rather than a default for SEED_DIR: an absent-minded `make seed` must never
# inject invented employers into a database holding real career history.
seed-sample:
	@echo "Seeding FICTIONAL sample data from $(SAMPLE_DIR) into $(DATABASE_URL)..."
	@for f in $(SAMPLE_DIR)/0*.sql; do \
		echo "  -> $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done
	@echo "Done. Log in as sample@example.com / sample-password."

# Reset a user's password. Stopgap until the UI grows a real reset flow.
# Prompts on the terminal; set NEWPASS to supply the password non-interactively.
reset-password:
ifndef EMAIL
	$(error EMAIL is required, e.g. make reset-password EMAIL=you@example.com)
endif
	@go run ./cmd/resetpw -email "$(EMAIL)"

# Formatting. Each language keeps its own pinned formatter:
#   Go      -- gofmt (toolchain)
#   TS/TSX  -- prettier, pinned in frontend/package.json
#   Python  -- ruff format, pinned in docx-renderer's dev group
# SQL is deliberately not formatted: migrations are applied history, and the
# sqlc query files carry load-bearing `-- name: ... :one` directives.
#
# Python also runs `ruff check` here, which is lint rather than formatting and
# so stretches these targets' name slightly. It earns the place: `ruff check`
# had never been wired into anything, in the Makefile or in CI, and four
# violations accumulated in that gap unnoticed. A lint that nothing runs is a
# lint that does not exist. `fmt` applies only ruff's safe fixes -- the
# unsafe ones change types or semantics and stay a human decision.
fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './frontend/*')
	cd frontend && npm run format
	cd docx-renderer && uv run ruff format . && uv run ruff check --fix .

fmt-check:
	@out="$$(gofmt -l $$(find . -name '*.go' -not -path './frontend/*'))"; \
		if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd frontend && npm run format:check
	cd docx-renderer && uv run ruff format --check . && uv run ruff check .

sqlc:
	sqlc generate

# Prompt provenance is content-addressed: generation records the git blob hash
# of each template it used. An uncommitted prompt edit still hashes correctly,
# but the blob exists in no commit, so `git cat-file` can't recover it later --
# and if you edit again, that exact text is gone.
#
# Warn only, never fail: editing a prompt and regenerating to see the effect is
# the normal tuning loop, and blocking it would be worse than the risk.
check-prompts:
	@git diff --quiet HEAD -- internal/generation/prompts/ 2>/dev/null || { \
		echo ""; \
		echo "  WARNING: uncommitted changes in internal/generation/prompts/"; \
		echo "  Resumes generated now will record prompt hashes that cannot be"; \
		echo "  resolved from git history. Commit before generating anything"; \
		echo "  you need to trace later."; \
		echo ""; \
	}

run: check-prompts
	go run ./cmd/server

run-frontend:
	cd frontend && npm run dev -- --host

run-renderer:
	cd docx-renderer && uv run uvicorn main:app --reload --port 8000

# Runs backend, frontend, and renderer together in one terminal.
# Ctrl-C stops all three.
dev:
	@trap 'kill $$(jobs -p) 2>/dev/null' EXIT INT TERM; \
	$(MAKE) run & \
	$(MAKE) run-frontend & \
	$(MAKE) run-renderer & \
	wait
