.PHONY: help run build tidy test fmt vet docker-up docker-down docker-prod migrate-create

# Load .env if present so DATABASE_URL is available to CLI targets.
ifneq (,$(wildcard .env))
include .env
export
endif

MIGRATIONS_DIR := migrations

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

run: ## Run the server locally
	go run ./cmd/server

build: ## Build the server binary into ./bin
	go build -o bin/server ./cmd/server

tidy: ## Sync go.mod / go.sum
	go mod tidy

test: ## Run tests
	go test ./...

fmt: ## Format the code
	go fmt ./...

vet: ## Static analysis
	go vet ./...

docker-up: ## Start the dev stack (Postgres + app)
	docker compose up --build -d

docker-down: ## Stop the dev stack
	docker compose down

docker-prod: ## Build and start the production stack
	docker compose -f docker-compose.prod.yml up --build -d

# Migrations are embedded and applied automatically at startup. Use this target
# only to scaffold a new pair of migration files (requires the golang-migrate
# CLI: https://github.com/golang-migrate/migrate).
migrate-create: ## Create a new migration: make migrate-create name=add_widgets
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
