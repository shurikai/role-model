-include .env
export

# SEED_DIR ?= ../role-model-data/seed
# DATABASE_URL ?= postgres://rolemodel:rolemodel@localhost:5433/role_model?sslmode=disable

# Fictional sample dataset, tracked in this repo (see database/sample/README.md).
SAMPLE_DIR ?= database/sample

.PHONY: all setup build clean test test-all test-race check-migrations db-up db-down db-reset db-dump migrate-up migrate-down migrate-down-all migrate-create seed seed-sample seed-clinical sqlc run run-frontend run-renderer dev check-prompts reset-password fmt fmt-check test-renderer

# Build
all: build

build:
	go build ./...

clean:
	go clean ./...

test:
	go test ./...

# Everything, in one command.
#
# `make test` is Go unit tests only, which is roughly 40% of the suite -- a
# contributor running it before pushing was exercising less than half of what
# CI would. The three languages each had their own target and nothing tied
# them together.
#
# REQUIRE_INTEGRATION turns the integration suites' "no DATABASE_URL, skipping"
# into a failure, so this cannot report success having silently run nothing.
test-all:
	go test ./...
	REQUIRE_INTEGRATION=1 go test -tags integration ./...
	cd frontend && npm run test
	cd docx-renderer && uv run pytest

# The race detector needs cgo, and therefore a C toolchain. Separate from
# test-all because that requirement is not universal; CI runs it always.
test-race:
	CGO_ENABLED=1 go test -race ./...
	CGO_ENABLED=1 REQUIRE_INTEGRATION=1 go test -race -tags integration ./...

# Refuses a migration numbered at or below one already on the base branch.
# golang-migrate skips those forever and reports "no change" -- see #92.
check-migrations:
	@scripts/check-migration-order.sh $(BASE_REF)

BASE_REF ?= origin/main

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

# SEED_DIR points at a SEPARATE PRIVATE REPO holding a real career, checked
# out in place at database/seed. Anyone who is not its owner has an empty or
# absent directory there -- and the glob below then expands to nothing, passes
# the literal `database/seed/0*.sql` to psql, and fails with "No such file or
# directory". That reads as a broken repository rather than as "this target is
# not for you", which is the first thing a stranger following the README hit.
seed:
	@if [ -z "$(SEED_DIR)" ]; then \
		echo "SEED_DIR is not set. It points at your own career seed files."; \
		echo "To load a bundled fictional dataset instead, run:"; \
		echo "    make seed-sample     # a backend engineer in freight logistics"; \
		echo "    make seed-clinical   # a nurse, built through the intake"; \
		exit 1; \
	fi
	@if [ -z "$$(ls $(SEED_DIR)/0*.sql 2>/dev/null)" ]; then \
		echo "No seed files found in $(SEED_DIR)."; \
		echo ""; \
		echo "That directory holds a private career seed repo, checked out in"; \
		echo "place. If you are not its owner, you want a sample dataset:"; \
		echo "    make seed-sample     # a backend engineer in freight logistics"; \
		echo "    make seed-clinical   # a nurse, built through the intake"; \
		exit 1; \
	fi
	@echo "Seeding from $(SEED_DIR)..."
	@for f in $(SEED_DIR)/0*.sql; do \
		echo "  -> $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done
	@echo "Done."

# The clinical sample dataset, produced BY THE INTAKE rather than written by
# hand -- see database/sample-clinical/README.md. Separate from seed-sample for
# the same reason seed-sample is separate from seed: two invented careers in one
# database is not a thing anyone wants by accident.
CLINICAL_DIR ?= database/sample-clinical

seed-clinical:
	$(call guard_sample_target,5b000000-0000-0000-0000-000000000001)
	@echo "Seeding FICTIONAL clinical sample data from $(CLINICAL_DIR) into $(DATABASE_URL)..."
	@for f in $(CLINICAL_DIR)/0*.sql; do \
		echo "  -> $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f" || exit 1; \
	done
	@echo "Done. Log in as priya@example.com / sample-password."

# Refuse to inject an invented career into a database that holds a different
# one. This exists because it happened: `DATABASE_URL=<scratch> make
# seed-clinical` ran against the LIVE database, because `-include .env` sets
# DATABASE_URL as a makefile variable and a makefile assignment beats the
# environment. The target printed the DSN it was using and nobody read it.
#
# The seed files themselves are upsert-safe, which is exactly why the mistake
# is quiet: nothing errors, a fictional user simply appears alongside a real
# one and stays there.
#
# $(1) is the user id the dataset owns. Any OTHER user in the database means
# this is not that dataset's database.
define guard_sample_target
	@intruder=$$(psql "$(DATABASE_URL)" -tAc "select count(*) from users where id <> '$(1)'" 2>/dev/null || echo 0); \
	if [ "$$intruder" != "0" ]; then \
		echo ""; \
		echo "  REFUSING: $(DATABASE_URL)"; \
		echo "  already holds $$intruder user(s) that this dataset does not own."; \
		echo ""; \
		echo "  Sample datasets are fictional and this target is upsert-safe, so"; \
		echo "  loading one here would quietly add an invented career beside a real"; \
		echo "  one rather than failing. Point DATABASE_URL at a scratch database."; \
		echo ""; \
		echo "  Note: an environment DATABASE_URL does NOT override the one in .env."; \
		echo "  Use:  make $$MAKECMDGOALS DATABASE_URL=postgres://..."; \
		echo ""; \
		exit 1; \
	fi
endef

# Fictional sample data, tracked in this repo. Deliberately a separate target
# rather than a default for SEED_DIR: an absent-minded `make seed` must never
# inject invented employers into a database holding real career history.
seed-sample:
	$(call guard_sample_target,5a000000-0000-0000-0000-000000000001)
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

# One command to a working host install, for the path that does not use
# containers. `docker compose up --build` is the other one, and needs none of
# this.
#
# Every step is idempotent, so running it again after a pull is the normal way
# to pick up a new migration or dependency.
setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env — set ANTHROPIC_API_KEY and JWT_SECRET before running."; \
	else \
		echo ".env already exists, leaving it alone."; \
	fi
	@if [ ! -f frontend/.env ]; then \
		cp frontend/.env.example frontend/.env; \
		echo "Created frontend/.env"; \
	else \
		echo "frontend/.env already exists, leaving it alone."; \
	fi
	$(MAKE) db-up
	@echo "Waiting for Postgres..."
	@for i in $$(seq 1 30); do \
		docker compose exec -T db pg_isready -U rolemodel -d role_model >/dev/null 2>&1 && break; \
		sleep 1; \
	done
	$(MAKE) migrate-up
	cd frontend && npm install
	@echo ""
	@echo "Ready. Next:"
	@echo "  1. Set ANTHROPIC_API_KEY and JWT_SECRET in .env"
	@echo "     (JWT_SECRET: openssl rand -base64 32 — the server will not start without it)"
	@echo "  2. make seed-sample     # optional: a fictional career to try the pipeline against"
	@echo "  3. make dev             # API, frontend and renderer together"

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
