# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: sqlc check-sqlc lint test test-no-db test-requires-db test-all start-db ensure-db stop-db coverage clean run run-down live-up live-down wait-rest wait-grpc smoke-rest smoke-grpc smoke-live check-live-tools ensure-compose help

DB_CONTAINER_NAME  := sc_postgres_unit_tests
DB_PORT            := 5433
COMMITTER_MODULE   := github.com/hyperledger/fabric-x-committer
COMMITTER_VERSION  := $(shell go list -m -f '{{.Version}}' $(COMMITTER_MODULE) 2>/dev/null)
COMMITTER_SCRIPTS  := $(shell go env GOMODCACHE)/$(COMMITTER_MODULE)@$(COMMITTER_VERSION)/scripts
COMPOSE            := $(shell if docker compose version >/dev/null 2>&1; then echo 'docker compose'; elif command -v docker-compose >/dev/null 2>&1; then echo 'docker-compose'; fi)
PYTHON_CMD         ?= python3
REST_BASE_URL      ?= http://127.0.0.1:8080
GRPC_TARGET        ?= 127.0.0.1:7051
SMOKE_NAMESPACE    ?= _meta
WAIT_RETRIES       ?= 60
WAIT_SLEEP_SECS    ?= 2

sqlc: ## Generate Go code from SQL using sqlc
	@echo "Generating Go code from SQL files..."
	sqlc generate
	@echo "✅ SQLC code generation complete"

check-sqlc: sqlc ## Verify SQLC generated code is up to date (fails if regeneration produces a diff)
	@tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	cp -R pkg/db/sqlc/. "$$tmp_dir"/; \
	sqlc generate; \
	if ! diff -qr "$$tmp_dir" pkg/db/sqlc >/dev/null; then \
		echo "❌ Generated SQLC code is out of sync with SQL files!"; \
		echo "Run 'make sqlc' locally and commit the changes."; \
		diff -ru "$$tmp_dir" pkg/db/sqlc || true; \
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
		./pkg/workerpool/...

start-db: ## Start a local PostgreSQL container for DB tests
	@go mod download $(COMMITTER_MODULE)
	@docker ps -aq -f name=$(DB_CONTAINER_NAME) | xargs -r docker rm -f
	@bash $(COMMITTER_SCRIPTS)/get-and-start-postgres.sh
	@echo "Waiting for Postgres to be ready..."
	@until docker exec $(DB_CONTAINER_NAME) pg_isready -U yugabyte -q; do sleep 1; done
	@echo "✅ Postgres is ready on localhost:$(DB_PORT)"

ensure-db: ## Ensure the test DB container is running and explorer DB exists; starts/creates if needed
	@if docker exec $(DB_CONTAINER_NAME) pg_isready -U yugabyte -q 2>/dev/null; then \
		echo "✅ Postgres already running on localhost:$(DB_PORT)"; \
	else \
		echo "⚡ Postgres not running — starting it now..."; \
		$(MAKE) start-db; \
	fi
	@if ! docker exec $(DB_CONTAINER_NAME) psql -U yugabyte -lqt 2>/dev/null | cut -d\| -f1 | grep -qw explorer; then \
		echo "⚡ 'explorer' database missing — creating it..."; \
		docker exec $(DB_CONTAINER_NAME) psql -U yugabyte -c "CREATE DATABASE explorer;" > /dev/null; \
		echo "✅ 'explorer' database created"; \
	else \
		echo "✅ 'explorer' database exists"; \
	fi

stop-db: ## Stop and remove the local PostgreSQL container
	docker ps -aq -f name=$(DB_CONTAINER_NAME) | xargs -r docker rm -f

test-requires-db: ensure-db ## Run DB tests (auto-starts Postgres if needed)
	@echo "Running tests with database..."
	go test -race -v -count=1 ./pkg/db/...

test-all: ensure-db ## Run all tests (auto-starts Postgres if needed)
	@echo "Running all tests..."
	go test -race -v -count=1 ./pkg/...

test: test-all ## Alias for test-all

coverage: ensure-db ## Generate test coverage report (auto-starts Postgres if needed)
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

ensure-compose:
	@if [ -z "$(COMPOSE)" ]; then echo "❌ Neither 'docker compose' nor 'docker-compose' is available"; exit 1; fi

check-live-tools:
	@command -v curl >/dev/null 2>&1 || { echo "❌ curl is required for live smoke tests"; exit 1; }
	@command -v $(PYTHON_CMD) >/dev/null 2>&1 || { echo "❌ $(PYTHON_CMD) is required for live smoke tests"; exit 1; }
	@command -v grpcurl >/dev/null 2>&1 || { echo "❌ grpcurl is required for live smoke tests"; exit 1; }

live-up: ensure-compose ## Build and start the live explorer stack in detached mode
	$(COMPOSE) up -d --build

live-down: ensure-compose ## Stop and remove the live explorer stack and volumes
	$(COMPOSE) down -v

wait-rest: check-live-tools ## Wait until the REST API responds successfully
	@attempt=0; \
	until curl -fsS $(REST_BASE_URL)/blocks/height >/dev/null 2>&1; do \
		attempt=$$((attempt + 1)); \
		if [ $$attempt -ge $(WAIT_RETRIES) ]; then \
			echo "❌ REST API did not become ready after $$attempt attempts"; \
			$(COMPOSE) logs --no-color --tail=80 explorer; \
			exit 1; \
		fi; \
		sleep $(WAIT_SLEEP_SECS); \
	done
	@echo "✅ REST API is ready at $(REST_BASE_URL)"

wait-grpc: check-live-tools ## Wait until the gRPC API responds successfully
	@attempt=0; \
	until grpcurl -plaintext $(GRPC_TARGET) list explorerv1.BlockExplorerService >/dev/null 2>&1; do \
		attempt=$$((attempt + 1)); \
		if [ $$attempt -ge $(WAIT_RETRIES) ]; then \
			echo "❌ gRPC API did not become ready after $$attempt attempts"; \
			$(COMPOSE) logs --no-color --tail=80 explorer; \
			exit 1; \
		fi; \
		sleep $(WAIT_SLEEP_SECS); \
	done
	@echo "✅ gRPC API is ready at $(GRPC_TARGET)"

smoke-rest: check-live-tools ## Call representative live REST endpoints, print results, and fail on bad responses
	@set -eu; \
	height_json="$$(curl -fsS $(REST_BASE_URL)/blocks/height)"; \
	echo "REST /blocks/height"; \
	echo "$$height_json"; \
	block_num="$$(printf '%s' "$$height_json" | $(PYTHON_CMD) -c 'import json,sys; height=int(json.load(sys.stdin)["height"]); print(1 if height > 1 else 0)')"; \
	blocks_json="$$(curl -fsS '$(REST_BASE_URL)/blocks?limit=3')"; \
	echo "---"; \
	echo "REST /blocks?limit=3"; \
	echo "$$blocks_json"; \
	block_json="$$(curl -fsS "$(REST_BASE_URL)/blocks/$$block_num?tx_limit=2")"; \
	echo "---"; \
	echo "REST /blocks/$$block_num?tx_limit=2"; \
	echo "$$block_json"; \
	tx_id="$$(printf '%s' "$$block_json" | $(PYTHON_CMD) -c 'import json,sys; txs=json.load(sys.stdin).get("transactions", []); print(txs[0]["txId"] if txs else "")')"; \
	if [ -n "$$tx_id" ]; then \
		tx_json="$$(curl -fsS "$(REST_BASE_URL)/transactions/$$tx_id")"; \
		echo "---"; \
		echo "REST /transactions/$$tx_id"; \
		echo "$$tx_json"; \
	else \
		echo "---"; \
		echo "REST transaction detail skipped: selected block has no transactions"; \
	fi; \
	ns_json="$$(curl -fsS "$(REST_BASE_URL)/namespaces/$(SMOKE_NAMESPACE)/policies")"; \
	echo "---"; \
	echo "REST /namespaces/$(SMOKE_NAMESPACE)/policies"; \
	echo "$$ns_json"

smoke-grpc: check-live-tools ## Call representative live gRPC endpoints with grpcurl, print results, and fail on bad responses
	@set -eu; \
	height_json="$$(grpcurl -plaintext -d '{}' $(GRPC_TARGET) explorerv1.BlockExplorerService.GetBlockHeight)"; \
	echo "gRPC GetBlockHeight"; \
	echo "$$height_json"; \
	block_num="$$(printf '%s' "$$height_json" | $(PYTHON_CMD) -c 'import json,sys; height=int(json.load(sys.stdin)["height"]); print(1 if height > 1 else 0)')"; \
	list_json="$$(grpcurl -plaintext -d '{"limit":3}' $(GRPC_TARGET) explorerv1.BlockExplorerService.ListBlocks)"; \
	echo "---"; \
	echo "gRPC ListBlocks"; \
	echo "$$list_json"; \
	block_json="$$(grpcurl -plaintext -d '{"block_num":'"$$block_num"',"tx_limit":2}' $(GRPC_TARGET) explorerv1.BlockExplorerService.GetBlockDetail)"; \
	echo "---"; \
	echo "gRPC GetBlockDetail block_num=$$block_num tx_limit=2"; \
	echo "$$block_json"; \
	tx_id="$$(printf '%s' "$$block_json" | $(PYTHON_CMD) -c 'import json,sys; txs=json.load(sys.stdin).get("transactions", []); print(txs[0]["txId"] if txs else "")')"; \
	if [ -n "$$tx_id" ]; then \
		tx_json="$$(grpcurl -plaintext -d '{"tx_id":"'"$$tx_id"'"}' $(GRPC_TARGET) explorerv1.BlockExplorerService.GetTransactionDetail)"; \
		echo "---"; \
		echo "gRPC GetTransactionDetail tx_id=$$tx_id"; \
		echo "$$tx_json"; \
	else \
		echo "---"; \
		echo "gRPC transaction detail skipped: selected block has no transactions"; \
	fi; \
	ns_json="$$(grpcurl -plaintext -d '{"namespace":"$(SMOKE_NAMESPACE)"}' $(GRPC_TARGET) explorerv1.BlockExplorerService.GetNamespacePolicies)"; \
	echo "---"; \
	echo "gRPC GetNamespacePolicies namespace=$(SMOKE_NAMESPACE)"; \
	echo "$$ns_json"

smoke-live: ensure-compose check-live-tools ## Recreate the live stack, wait for readiness, call REST and gRPC endpoints, print results, and fail on any bad response
	@$(MAKE) live-down
	@$(MAKE) live-up
	@$(MAKE) wait-rest
	@$(MAKE) wait-grpc
	@$(MAKE) smoke-rest
	@$(MAKE) smoke-grpc
	@echo "✅ Live REST and gRPC smoke checks passed"

run: ## Build and start postgres + explorer (sidecar must be running externally)
	@if [ -z "$(COMPOSE)" ]; then echo "❌ Neither 'docker compose' nor 'docker-compose' is available"; exit 1; fi
	$(COMPOSE) up --build

run-down: ## Stop and remove Docker Compose services and volumes
	@if [ -z "$(COMPOSE)" ]; then echo "❌ Neither 'docker compose' nor 'docker-compose' is available"; exit 1; fi
	$(COMPOSE) down -v

help: ## Display this help message
	@echo "Fabric X Block Explorer - Makefile targets"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
