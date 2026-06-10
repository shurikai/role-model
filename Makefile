.PHONY: all build clean test db-up db-down db-reset migrate-up migrate-down migrate-create

# Build
all: build

build:
	go build ./...

clean:
	go clean ./...

test:
	go test ./...

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
