-include .env
export

# SEED_DIR ?= ../role-model-data/seed
# DATABASE_URL ?= postgres://rolemodel:rolemodel@localhost:5433/role_model?sslmode=disable

.PHONY: all build clean test db-up db-down db-reset migrate-up migrate-down migrate-create seed sqlc run

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

# Database
db-up:
	docker compose up -d

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d

# Migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

seed:
	@echo "Seeding from $(SEED_DIR)..."
	@for f in $(SEED_DIR)/0*.sql; do \
		echo "  -> $$f"; \
		psql "$(DATABASE_URL)" -f "$$f" || exit 1; \
	done
	@echo "Done."

sqlc:
	sqlc generate

run:
	go run ./cmd/server
