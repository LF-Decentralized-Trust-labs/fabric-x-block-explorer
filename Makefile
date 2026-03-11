# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: sqlc check-sqlc lint test test-no-db test-requires-db test-all start-db stop-db coverage clean help

DB_CONTAINER_NAME  := sc_postgres_unit_tests
DB_PORT            := 5433
COMMITTER_MODULE   := github.com/hyperledger/fabric-x-committer
COMMITTER_VERSION  := $(shell go list -m -f '{{.Version}}' $(COMMITTER_MODULE) 2>/dev/null)
COMMITTER_SCRIPTS  := $(shell go env GOMODCACHE)/$(COMMITTER_MODULE)@$(COMMITTER_VERSION)/scripts

sqlc: ## Generate Go code from SQL using sqlc
	@echo "Generating Go code from SQL files..."
	sqlc generate
	@echo "✅ SQLC code generation complete"

check-sqlc: sqlc ## Verify SQLC generated code is up to date (fails if regeneration produces a diff)
	@if ! git diff --exit-code pkg/db/sqlc/; then \
		echo "❌ Generated SQLC code is out of sync with SQL files!"; \
		echo "Run 'make sqlc' locally and commit the changes."; \
		exit 1; \
	fi
	@echo "✅ Generated SQLC code is up to date"

lint: ## Run golangci-lint
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "✅ Lint passed"

test-no-db: ## Run tests that do not require a database (parser, types, util, config, pipeline)
	@echo "Running tests without database requirement..."
	go test -race -v -count=1 \
		./pkg/parser/... \
		./pkg/util/... \
		./pkg/config/... \
		./pkg/blockpipeline/... \
		./pkg/sidecarstream/... \
		./pkg/pipeline/...

start-db: ## Start a local PostgreSQL container for DB tests
	@go mod download $(COMMITTER_MODULE)
	@docker ps -aq -f name=$(DB_CONTAINER_NAME) | xargs -r docker rm -f
	@bash $(COMMITTER_SCRIPTS)/get-and-start-postgres.sh
	@echo "Waiting for Postgres to be ready..."
	@until docker exec $(DB_CONTAINER_NAME) pg_isready -U yugabyte -q; do sleep 1; done
	@echo "✅ Postgres is ready on localhost:$(DB_PORT)"

stop-db: ## Stop and remove the local PostgreSQL container
	docker ps -aq -f name=$(DB_CONTAINER_NAME) | xargs -r docker rm -f

test-requires-db: ## Run DB tests (requires running Postgres: make start-db)
	@echo "Running tests with database..."
	go test -race -v -count=1 ./pkg/db/...

test-all: ## Run all tests (requires running Postgres: make start-db)
	@echo "Running all tests..."
	go test -race -v -count=1 ./pkg/...

test: test-all ## Alias for test-all

coverage: ## Generate test coverage report (requires running Postgres: make start-db)
	@echo "Generating coverage report..."
	@mkdir -p coverage
	go test -race -coverprofile=coverage/coverage.out ./pkg/...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	go tool cover -func=coverage/coverage.out
	@echo ""
	@echo "Coverage report generated: coverage/coverage.html"

clean: ## Remove build artifacts and coverage reports
	@echo "Cleaning build artifacts..."
	rm -rf coverage/
	@echo "Clean complete"

help: ## Display this help message
	@echo "Fabric X Block Explorer - Makefile targets"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
