# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: sqlc test test-no-db test-requires-db test-all coverage clean help

sqlc: ## Generate Go code from SQL using sqlc
	@echo "Generating Go code from SQL files..."
	sqlc generate
	@echo "✅ SQLC code generation complete"

test-no-db: ## Run tests that don't require database
	@echo "Running tests without database requirement..."
	go test -v -count=1 \
		./pkg/types/... \
		./pkg/util/... \
		./pkg/logging/... \
		./pkg/constants/...

test-requires-db: ## Run tests that require database (uses testcontainers by default)
	@echo "Running tests that require database..."
	go test -v -count=1 ./pkg/db/...

test-all: ## Run all tests
	@echo "Running all tests..."
	go test -v -count=1 ./pkg/...

test: test-all ## Alias for test-all

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./pkg/...
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
