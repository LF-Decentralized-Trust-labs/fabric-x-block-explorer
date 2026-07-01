# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: sqlc check-sqlc build lint test test-no-db test-requires-db test-all test-integration start-db ensure-db stop-db kill-test-docker coverage clean run run-down live-up live-down live-stop wait-rest smoke-rest smoke-live swagger check-live-tools ensure-compose build-test-node ui-install ui-dev ui-build ui-lint dev help

DB_CONTAINER_NAME  := sc_test_postgres_unit_tests
DB_PORT            := 5433
COMMITTER_MODULE   := github.com/hyperledger/fabric-x-committer
COMMITTER_VERSION  := $(shell go list -m -f '{{.Version}}' $(COMMITTER_MODULE) 2>/dev/null)
COMMITTER_SRC      := $(shell go env GOMODCACHE)/$(COMMITTER_MODULE)@$(COMMITTER_VERSION)
COMMITTER_SCRIPTS  := $(COMMITTER_SRC)/scripts
TEST_NODE_IMAGE    := docker.io/hyperledger/committer-test-node:$(COMMITTER_VERSION)
COMPOSE            := $(shell if docker compose version >/dev/null 2>&1; then echo 'docker compose'; elif command -v docker-compose >/dev/null 2>&1; then echo 'docker-compose'; fi)
PYTHON_CMD         ?= python3
REST_BASE_URL      ?= http://127.0.0.1:8080
SMOKE_NAMESPACE    ?= _meta
WAIT_RETRIES       ?= 60
WAIT_SLEEP_SECS    ?= 2
VERSION            ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY             := ./bin/explorer
CLI_PKG            := github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/cli
LD_FLAGS           := -ldflags "-X $(CLI_PKG).Version=$(VERSION)"

build: ## Build the explorer binary with version injection
	@mkdir -p bin
	go build $(LD_FLAGS) -o $(BINARY) ./cmd/explorer/
	@echo "✅ Built $(BINARY) version=$(VERSION)"

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
		./pkg/sidecarstream/...

start-db: ## Start a local PostgreSQL container for DB tests
	@go mod download $(COMMITTER_MODULE)
	@docker ps -aq -f name=$(DB_CONTAINER_NAME) | xargs -r docker rm -f
	@chmod +x $(COMMITTER_SCRIPTS)/db-version.sh $(COMMITTER_SCRIPTS)/get-and-start-postgres.sh
	@cd $(COMMITTER_SRC) && bash scripts/get-and-start-postgres.sh
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

test-all: ensure-db ## Run all unit tests (auto-starts Postgres if needed)
	@echo "Running all unit tests..."
	go test -race -v -count=1 $(shell go list ./pkg/... | grep -v '/integration')
test: test-all ## Alias for test-all

coverage: ensure-db ## Generate test coverage report (auto-starts Postgres if needed)
	@echo "Generating coverage report..."
	@mkdir -p coverage
	go test -race -coverprofile=coverage/coverage.out $(shell go list ./pkg/... | grep -v '/integration')
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


smoke-rest: check-live-tools ## Call representative live REST endpoints, print results, and fail on bad responses
	@set -eu; \
	height_json="$$(curl -fsS $(REST_BASE_URL)/blocks/height)"; \
	echo "REST /blocks/height"; \
	echo "$$height_json"; \
	height="$$(printf '%s' "$$height_json" | $(PYTHON_CMD) -c 'import json,sys; print(int(json.load(sys.stdin)["height"]))')"; \
	blocks_json="$$(curl -fsS '$(REST_BASE_URL)/blocks?limit=3')"; \
	echo "---"; \
	echo "REST /blocks?limit=3"; \
	echo "$$blocks_json"; \
	if [ "$$height" -gt 0 ]; then \
		block_num="$$(printf '%s' "$$height_json" | $(PYTHON_CMD) -c 'import json,sys; height=int(json.load(sys.stdin)["height"]); print(1 if height > 1 else 0)')"; \
		block_json="$$(curl -fsS "$(REST_BASE_URL)/blocks/$$block_num?tx_limit=2")"; \
		echo "---"; \
		echo "REST /blocks/$$block_num?tx_limit=2"; \
		echo "$$block_json"; \
		tx_id="$$(printf '%s' "$$block_json" | $(PYTHON_CMD) -c 'import json,sys; txs=json.load(sys.stdin).get("transactions", []); print(txs[0]["tx_id"] if txs else "")')"; \
		if [ -n "$$tx_id" ]; then \
			tx_json="$$(curl -fsS "$(REST_BASE_URL)/transactions/$$tx_id")"; \
			echo "---"; \
			echo "REST /transactions/$$tx_id"; \
			echo "$$tx_json"; \
		else \
			echo "---"; \
			echo "REST transaction detail skipped: selected block has no transactions"; \
		fi; \
	else \
		echo "---"; \
		echo "REST block/tx detail skipped: no blocks ingested yet (height=0)"; \
	fi; \
	ns_json="$$(curl -fsS "$(REST_BASE_URL)/namespaces/$(SMOKE_NAMESPACE)/policies")"; \
	echo "---"; \
	echo "REST /namespaces/$(SMOKE_NAMESPACE)/policies"; \
	echo "$$ns_json"

smoke-live: ensure-compose check-live-tools ## Recreate the live stack, wait for REST readiness, call all REST endpoints, print results, and fail on any bad response
	@$(MAKE) live-down
	@$(MAKE) live-up
	@$(MAKE) wait-rest
	@$(MAKE) smoke-rest
	@echo "✅ Live REST smoke checks passed"

run: ## Build and start postgres + explorer + UI (sidecar must be running externally)
	@if [ -z "$(COMPOSE)" ]; then echo "❌ Neither 'docker compose' nor 'docker-compose' is available"; exit 1; fi
	$(COMPOSE) up --build

run-down: ## Stop and remove Docker Compose services and volumes
	@if [ -z "$(COMPOSE)" ]; then echo "❌ Neither 'docker compose' nor 'docker-compose' is available"; exit 1; fi
	$(COMPOSE) down -v

build-test-node: ## Build the fabric-x-committer all-in-one test node Docker image
	@if docker image inspect $(TEST_NODE_IMAGE) >/dev/null 2>&1; then \
		echo "✅ $(TEST_NODE_IMAGE) already present — skipping build"; \
	else \
		echo "⚡ Building committer-test-node from module cache (this takes a few minutes)..."; \
		TMP=$$(mktemp -d); \
		trap 'rm -rf "$$TMP"' EXIT; \
		cp -r "$(COMMITTER_SRC)/." "$$TMP/"; \
		chmod -R u+w "$$TMP"; \
		$(MAKE) -C "$$TMP" build-image-test-node; \
		docker tag docker.io/hyperledger/committer-test-node:latest $(TEST_NODE_IMAGE); \
		echo "✅ committer-test-node image built and tagged as $(TEST_NODE_IMAGE)"; \
	fi

kill-test-docker: ## Stop and remove all sc_test_* Docker containers (test cleanup)
	@ids=$$(docker ps -aq -f 'name=sc_test'); \
	if [ -n "$$ids" ]; then \
		echo "Removing containers: $$ids"; \
		docker rm -f $$ids; \
	else \
		echo "No sc_test_* containers running"; \
	fi

test-integration: ensure-db build-test-node ## Run integration tests (starts committer test node + explorer against live DB)
	@echo "Running integration tests..."
	gotestsum --rerun-fails=1 --format testname --packages ./pkg/integration/... -- -v -count=1 -timeout=10m

swagger: build ## Build explorer, start full self-contained stack (committer + postgres + explorer), run smoke tests, then open Swagger UI
	bash scripts/test-live.sh --keep
	open http://127.0.0.1:18080/docs

live-stop: ## Tear down the live stack started by 'make swagger' or 'test-live.sh --keep'
	bash scripts/test-live.sh --down

ui-install: ## Install UI dependencies (npm ci)
	cd ui && npm ci

ui-dev: ## Run the UI in development mode (requires backend on :8080)
	cd ui && npm run dev

ui-build: ## Build the UI for production
	cd ui && npm run build

ui-lint: ## Lint the UI source
	cd ui && npm run lint

dev: ## 🚀 One-command local E2E: build binary, start committer test node + postgres + explorer, install UI deps, launch UI dev server
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Fabric-X Block Explorer — local E2E dev setup"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "Step 1/2  Starting self-contained backend stack..."
	@echo "          (builds binary, starts committer + postgres + explorer)"
	@echo "          Logs → /tmp/fx-explorer-live.log  |  REST → http://127.0.0.1:18080"
	@bash scripts/test-live.sh --keep
	@echo ""
	@echo "Step 2/2  Starting UI dev server..."
	@if [ ! -d ui/node_modules ]; then \
		echo "          Installing UI dependencies (npm ci)..."; \
		cd ui && npm ci --silent; \
	fi
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  ✅ Stack is live — open in your browser:"
	@echo "     UI         → http://localhost:3000"
	@echo "     REST API   → http://127.0.0.1:18080"
	@echo "     Swagger    → http://127.0.0.1:18080/docs"
	@echo "  Press Ctrl+C to stop the UI (run 'make dev-down' to stop the backend)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@cd ui && BACKEND_URL=http://127.0.0.1:18080 npm run dev

dev-down: ## 🛑 Tear down everything started by 'make dev'
	@bash scripts/test-live.sh --down
	@echo "✅ Backend stack stopped"

help: ## Display this help message
	@echo "Fabric X Block Explorer - Makefile targets"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help

# https://www.gnu.org/software/make/manual/html_node/Force-Targets.html
# If a rule has no prerequisites or recipe, and the target of the rule is a nonexistent file,
# then make imagines this target to have been updated whenever its rule is run.
FORCE:
