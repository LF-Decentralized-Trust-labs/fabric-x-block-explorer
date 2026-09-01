# Production Readiness Plan — Fabric-X Block Explorer

**Status:** Living document · Follow during phased implementation  
**Repo:** `fabric-x-block-explorer` · Go 1.26 · Next.js 14 · PostgreSQL 16

---

## Overview

This document is the single source of truth for achieving production readiness.
It is organized into **5 phases**, each delivered as **one focused PR**.  
Every PR is small, reviewable, and independently mergeable — nothing is a
mega-commit. Each phase builds on the previous one without blocking it.

```
Phase 1 ── Security & Config Hardening
Phase 2 ── REST API Unit Tests + Coverage Gate
Phase 3 ── Reliability: Readiness Probe, start.sh Watchdog, Structured Logs
Phase 4 ── UI Tests + OpenAPI Contract Validation
Phase 5 ── Performance & Soak Tests + Observability
```

---

## Guiding Principles

- **Every PR is independently reviewable** — max ~300 lines of production code change.
- **No PR breaks existing tests** — each one must pass all CI checks green before merge.
- **Test first where possible** — add the test, then make it green.
- **Only change what the phase demands** — no opportunistic refactors.

---

## Phase 1 — Security & Configuration Hardening

**PR title:** `fix: security hardening — CORS config, request cap, weak password warning`  
**Branch:** `fix/phase1-security-hardening`  
**Size:** ~150 lines of production code, ~60 lines of config/docs

### Goals
- Eliminate the three Critical gaps (G-01, G-02, G-03) before any other work.
- No infra dependencies — pure Go config/handler changes.

### Changes

#### 1.1 — Configurable CORS (`pkg/config/config.go`, `pkg/api/rest.go`)

**What:** Replace `Access-Control-Allow-Origin: *` with a configurable allow-list.

**Config addition** (`RESTConfig`):
```yaml
server:
  rest:
    cors_allowed_origins:
      - "http://localhost:3000"   # default — UI on same host
      # - "*"                     # uncomment only for fully public deployments
```

**Implementation:**
- Add `CORSAllowedOrigins []string` field to `RESTConfig`.
- In `corsMiddleware`: if the list is empty or contains `"*"`, keep current open
  behaviour (backward-compatible for dev). Otherwise reflect the request `Origin`
  only if it appears in the list.
- Add viper default: `["*"]` so existing deployments are unaffected.
- Update `config.local.yaml` and `config.docker.yaml` with the new key (commented out).

**Tests to add** (`pkg/api/rest_test.go` — new file):
```
TestCORSMiddleware/origin_in_allowlist        → allowed
TestCORSMiddleware/origin_not_in_allowlist    → not reflected
TestCORSMiddleware/wildcard_default           → reflects any origin
TestCORSMiddleware/preflight_OPTIONS          → 204, correct headers
```

#### 1.2 — Request size cap (`pkg/api/rest.go`, `pkg/config/config.go`)

**What:** Enforce a hard maximum on `limit` and `tx_limit` query parameters.

**Config addition:**
```yaml
server:
  rest:
    max_list_limit: 500   # max allowed value for ?limit / ?tx_limit
```

**Implementation:**
- Add `MaxListLimit int32` to `RESTConfig`. Default `500`.
- In `handleListBlocks` and `handleGetBlockByNumber`: after parsing `limit` /
  `tx_limit`, if the value exceeds `MaxListLimit` return HTTP 400 with message
  `"limit must be <= <max>"`.

**Tests to add** (`pkg/api/rest_test.go`):
```
TestHandleListBlocks/limit_over_max           → 400
TestHandleListBlocks/limit_at_max             → 200
TestHandleListBlocks/limit_zero_uses_default  → 200
TestHandleGetBlockByNumber/tx_limit_over_max  → 400
```

#### 1.3 — Weak password warning (`pkg/api/service.go` or `pkg/cli/explorer.go`)

**What:** Log a prominent WARN on startup when the DB password is a known weak
default so operators notice it immediately.

**Implementation:**
- Add `warnWeakSecrets(cfg *config.Config)` in `pkg/cli/explorer.go`, called
  just after `cfg.Validate()`.
- Known weak values: `"postgres"`, `"password"`, `"changeme"`, `""`.
- Log format: `WARN ⚠️  database.password is a well-known default — change it before production use`.
- Not a fatal error — only a log line.

**Tests to add** (`pkg/cli/explorer_test.go`):
```
TestWarnWeakSecrets/known_weak    → warning logged
TestWarnWeakSecrets/strong_value  → no warning
```

### Corner cases tested in Phase 1
| Scenario | Expected |
|---|---|
| `?limit=0` | Uses `default_tx_limit`, not 0 |
| `?limit=-1` | 400 Bad Request |
| `?limit=501` (over max) | 400 Bad Request |
| `?limit=500` (at max) | 200 OK |
| CORS preflight from allowed origin | 204, correct allow headers |
| CORS preflight from blocked origin | 204, no `Allow-Origin` reflected |
| DB password = `"postgres"` on startup | WARN in log |
| DB password = `"my-strong-pass"` on startup | No warn |

### Definition of Done
- [ ] `make lint` clean
- [ ] `make test-no-db` green
- [ ] New `pkg/api/rest_test.go` with `go test -race -count=1` green
- [ ] `config.local.yaml` and `config.docker.yaml` updated with commented example
- [ ] PR description references each gap (G-01, G-02, G-03) with ✅

---

## Phase 2 — REST API Unit Tests & Coverage Gate

**PR title:** `test: REST handler unit tests, resumeBlockNum coverage, CI coverage gate`  
**Branch:** `test/phase2-rest-unit-tests`  
**Size:** ~400 lines of test code, ~10 lines of CI YAML

### Goals
- Add first-class unit tests for the REST layer (G-09).
- Test the critical `resumeBlockNum` gap-healing logic (G-10).
- Enforce a 60% coverage floor in CI (G-13).

### Changes

#### 2.1 — REST handler unit tests (`pkg/api/rest_test.go`)

Uses `net/http/httptest` with a mock `Querier` generated by `gomock` or a simple
hand-written stub implementing `dbsqlc.Querier`.

**Tests to add:**

_Healthz_
```
TestHandleHealthz                             → 200, body {status:"ok"}
```

_Block height_
```
TestHandleGetBlockHeight/returns_height       → 200, correct JSON
TestHandleGetBlockHeight/db_error             → 500
TestHandleGetBlockHeight/context_cancelled    → 499
```

_List blocks_
```
TestHandleListBlocks/happy_path               → 200, correct blocks slice
TestHandleListBlocks/empty_db                 → 200, empty blocks []
TestHandleListBlocks/from_gt_to              → 400
TestHandleListBlocks/negative_offset         → 400
TestHandleListBlocks/non_integer_limit       → 400
TestHandleListBlocks/db_error                → 500
```

_Get block by number_
```
TestHandleGetBlockByNumber/found             → 200
TestHandleGetBlockByNumber/not_found         → 404
TestHandleGetBlockByNumber/negative_num     → 400
TestHandleGetBlockByNumber/non_integer      → 400
TestHandleGetBlockByNumber/db_error         → 500
```

_Get transaction_
```
TestHandleGetTxByID/found                   → 200
TestHandleGetTxByID/not_found               → 404
TestHandleGetTxByID/non_hex_tx_id          → 400
TestHandleGetTxByID/empty_tx_id            → 400 (or 404 via mux)
```

_Namespace policies_
```
TestHandleListAllNamespacePolicies/found    → 200
TestHandleGetNamespacePolicies/found        → 200
TestHandleGetNamespacePolicies/not_found    → 200, empty policies []
```

_CORS + logging middleware pass-through_ (already in 1.1 but add negative paths here)

#### 2.2 — `resumeBlockNum` unit tests (`pkg/api/service_test.go`)

Requires real Postgres (tagged `//go:build db`).

```
TestResumeBlockNum/empty_table             → returns fallback
TestResumeBlockNum/contiguous_from_0       → returns max+1
TestResumeBlockNum/gap_at_middle           → returns gap position
TestResumeBlockNum/gap_at_start            → returns fallback
TestResumeBlockNum/fallback_already_in_db  → returns max+1
TestResumeBlockNum/multi_writer_gap        → returns lowest gap
```

#### 2.3 — CI coverage gate (`.github/workflows/ci.yaml`)

Add a step to the existing `test` job:
```yaml
- name: Check coverage threshold
  run: |
    go tool cover -func=coverage/coverage.out | \
      awk '/^total:/ { pct = $3+0; if (pct < 60) { printf "Coverage %.1f%% < 60%% threshold\n", pct; exit 1 } }'
- name: Upload coverage report
  uses: actions/upload-artifact@v4
  with:
    name: coverage-report
    path: coverage/coverage.html
```

Modify `coverage` target to also write `coverage.out`:
```makefile
coverage: ensure-db
    go test -race -coverprofile=coverage/coverage.out \
        $(shell go list ./pkg/... | grep -v '/integration')
    go tool cover -html=coverage/coverage.out -o coverage/coverage.html
    go tool cover -func=coverage/coverage.out
```

### Corner cases tested in Phase 2
| Scenario | Expected |
|---|---|
| `block_num = 0` in path | 200 OK (genesis block) |
| `block_num = -1` | 400 Bad Request |
| `tx_id = "zzzz"` (invalid hex) | 400 Bad Request |
| `tx_id = "000...0"` (valid hex, not in DB) | 404 Not Found |
| DB returns `pgx.ErrNoRows` | 404 Not Found |
| DB returns context.DeadlineExceeded | 499 |
| DB returns any other error | 500, no internal details leaked |
| `resumeBlockNum` with blocks 0,1,3 (gap at 2) | returns 2 |
| `resumeBlockNum` with no blocks | returns fallback |
| `resumeBlockNum` with blocks 0–N, no gap | returns N+1 |

### Definition of Done
- [ ] `make test-all` green (includes new DB-tagged tests via `make test-requires-db`)
- [ ] Coverage gate in CI passes ≥ 60% total
- [ ] Coverage report uploads as artifact on every CI run
- [ ] No existing test removed or weakened

---

## Phase 3 — Reliability: Readiness Probe, Watchdog, Structured Logs

**PR title:** `feat: /readyz probe, start.sh watchdog, structured JSON logging`  
**Branch:** `feat/phase3-reliability`  
**Size:** ~120 lines production code, ~30 lines shell

### Goals
- Operators and orchestrators can distinguish live-but-degraded from healthy (G-04).
- Go binary crash causes the container to restart promptly (G-05).
- Log output is parseable by every major aggregator (G-06).

### Changes

#### 3.1 — Readiness probe `GET /readyz` (`pkg/api/rest.go`, `pkg/api/service.go`)

**What:** A new endpoint that returns 200 only when the DB pool is reachable AND the
pipeline has ingested at least one block within the last `stale_threshold` seconds.

**Config addition:**
```yaml
server:
  rest:
    stale_block_threshold: 120s  # /readyz returns 503 if no new block in this window
```

**Implementation:**
- Add `lastBlockAt atomic.Int64` (Unix ns) to `Service`, updated by `blockWriter`
  on each successful `WriteProcessedBlock`.
- Add `handleReadyz(w, r)`:
  - Ping the DB pool (`pool.Ping(ctx)` with 3 s timeout).
  - If ping fails → 503 `{"status":"unavailable","reason":"db_unreachable"}`.
  - If `lastBlockAt` is set and `time.Since(lastBlockAt) > staleThreshold` → 503
    `{"status":"degraded","reason":"pipeline_stalled","last_block_ago":"..."}`.
  - Otherwise → 200 `{"status":"ready"}`.
- Register `GET /readyz` in `newRESTRouter`.
- Update `docker-compose.yaml` healthcheck to use `/readyz` for the ready check:
  ```yaml
  healthcheck:
    test: ["CMD","sh","-c","wget -q --spider http://localhost:8080/healthz && wget -q --spider http://localhost:8080/readyz"]
  ```

**Tests to add** (`pkg/api/rest_test.go`):
```
TestHandleReadyz/db_ok_pipeline_fresh     → 200
TestHandleReadyz/db_unreachable           → 503, reason=db_unreachable
TestHandleReadyz/pipeline_stalled         → 503, reason=pipeline_stalled
TestHandleReadyz/first_start_no_blocks    → 200 (stale check skipped until first block)
```

#### 3.2 — `start.sh` watchdog (`docker/images/release/start.sh`)

**What:** Background loop monitors the Go PID. If it exits, kill PID 1 so the
container restarts (controlled by `restart: unless-stopped`).

```sh
# Background watchdog: if explorer exits, bring down the container so
# the orchestrator can restart it and alert on the failure.
watch_backend() {
  while kill -0 "$BACKEND_PID" 2>/dev/null; do
    sleep 5
  done
  echo "ERROR: explorer backend exited unexpectedly — killing container" >&2
  kill 1
}
watch_backend &
```

**Tests:**  
In `scripts/test-live.sh`, add a check after `--keep`:
- Kill the Go PID manually inside the container.
- Wait up to 30 s for the container to transition to `restarting` or `unhealthy`.
- Assert the health state changed.  
(This is a manual smoke test documented in the test-live script comments — not a CI test due to container restart cycle time.)

#### 3.3 — Structured JSON logging (`docker/images/release/Dockerfile`)

**What:** Enable `flogging` JSON output in the runtime image.

```dockerfile
ENV FABRIC_LOGGING_FORMAT=json
ENV FABRIC_LOGGING_SPEC=INFO
```

**Validation step added to CI** (inside `test` job):
```yaml
- name: Verify JSON log output
  run: |
    ./bin/explorer version 2>&1 | head -1 | python3 -c \
      "import sys,json; json.loads(sys.stdin.read().strip())" && echo "✅ JSON logs"
```
(The `version` subcommand produces one log line and exits, making it easy to test.)

### Corner cases tested in Phase 3
| Scenario | Expected |
|---|---|
| `/readyz` with healthy DB and recent block | 200 `{"status":"ready"}` |
| `/readyz` when Postgres is stopped | 503 `reason=db_unreachable` within 3 s |
| `/readyz` when pipeline stalled > threshold | 503 `reason=pipeline_stalled` |
| `/readyz` immediately after startup (no blocks yet) | 200 (stale guard skipped) |
| `/healthz` when DB is down | 200 (liveness must never touch DB) |
| `FABRIC_LOGGING_FORMAT=json` — first log line | Valid JSON |

### Definition of Done
- [ ] `GET /readyz` returns 200 in `make smoke-live`
- [ ] `docker-compose.yaml` uses `/readyz` in healthcheck
- [ ] `FABRIC_LOGGING_FORMAT=json` verified in Dockerfile ENV
- [ ] `start.sh` watchdog present and tested by manual smoke step
- [ ] `make lint` clean
- [ ] `make test-no-db` green

---

## Phase 4 — UI Tests & OpenAPI Contract Validation

**PR title:** `test: Vitest unit tests for UI utils/api, schemathesis contract CI step`  
**Branch:** `test/phase4-ui-and-contract`  
**Size:** ~300 lines test code, ~20 lines CI YAML, ~10 lines package.json

### Goals
- Give the UI a test runner and cover the logic-heavy helper modules (G-11).
- Prevent OpenAPI spec drift from catching real responses (G-12).

### Changes

#### 4.1 — Vitest + React Testing Library setup (`ui/`)

**Packages to add (devDependencies):**
```json
"vitest": "^1.6.0",
"@vitejs/plugin-react": "^4.2.1",
"@testing-library/react": "^15.0.0",
"@testing-library/jest-dom": "^6.4.0",
"msw": "^2.2.0",
"jsdom": "^24.0.0"
```

**New files:**
```
ui/vitest.config.ts
ui/vitest.setup.ts
ui/__tests__/utils.test.ts
ui/__tests__/api.test.ts
ui/__tests__/policyDecoder.test.ts
```

**`package.json` script addition:**
```json
"test": "vitest run",
"test:watch": "vitest"
```

**Tests for `lib/utils.ts`:**
```
decodeHexBytes
  ├── empty/null input → empty result
  ├── valid UTF-8 hex → isReadable=true
  ├── JSON hex → isJson=true, jsonValue parsed
  ├── binary hex (non-UTF-8) → isReadable=false, raw returned
  └── odd-length hex → graceful fallback

formatBytes
  ├── 0 → "0 B"
  ├── 1023 → "1023 B"
  ├── 1024 → "1.0 KB"
  └── 1048576 → "1.0 MB"

getValidationCodeText
  ├── string "COMMITTED" → "COMMITTED"
  ├── number 0 → "VALID"
  └── unknown number → "UNKNOWN (n)"

getValidationTone
  ├── "VALID" → success
  ├── "COMMITTED" → success
  ├── "NIL_ENVELOPE" → warning
  └── "ENDORSEMENT_POLICY_FAILURE" → error

truncateMiddle
  ├── short string → unchanged
  └── long string → truncated with "..."

parseProtoNumber
  ├── number input → same number
  ├── string "42" → 42
  └── null/undefined → 0
```

**Tests for `lib/api.ts`** (using `msw` to mock fetch):
```
transformBlockSummary
  ├── maps block_num → block_number
  ├── maps tx_count → transaction_count
  └── null previous_hash → null (not undefined)

transformTransaction
  ├── maps block_num → block_number
  ├── maps tx_num → tx_index
  ├── null chaincode_name → null
  └── nested creator_identity mapped correctly

api.getBlockHeight
  ├── success → { height: N }
  └── 500 error → axios error thrown

api.getBlock
  ├── success → transformed Block
  └── 404 → axios error thrown

api.getPolicies
  ├── success → transformed NamespacePolicy[]
  └── empty policies array → []
```

**Tests for `lib/policyDecoder.ts`:**
```
decodePolicyExpression
  ├── valid policy string → human-readable expression
  ├── empty input → empty string
  └── malformed policy → graceful empty result
```

#### 4.2 — npm audit in CI (`.github/workflows/ci.yaml`)

Add to the existing CI, new job `ui-security`:
```yaml
ui-security:
  name: UI Security Audit
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@...
    - uses: actions/setup-node@v4
      with: { node-version: '22' }
    - run: cd ui && npm ci --prefer-offline
    - run: cd ui && npm audit --audit-level=high
```

#### 4.3 — OpenAPI contract validation (`.github/workflows/ci.yaml`)

Add a new CI job `contract-test` that runs after the `integration` job:
```yaml
contract-test:
  name: OpenAPI Contract Tests
  needs: [integration]
  runs-on: ubuntu-latest
  timeout-minutes: 20
  steps:
    - uses: actions/checkout@...
    - uses: actions/setup-go@...
    - name: Install schemathesis
      run: pip install schemathesis
    - name: Start live stack
      run: make smoke-live &   # starts stack, runs smoke, keeps running
      # (or use make live-up + make wait-rest)
    - name: Run schemathesis
      run: |
        st run http://127.0.0.1:8080/openapi.yaml \
          --base-url http://127.0.0.1:8080 \
          --checks not_a_server_error \
          --hypothesis-max-examples 50
    - name: Tear down
      if: always()
      run: make live-down
```

### Corner cases tested in Phase 4
| Scenario | Expected |
|---|---|
| `decodeHexBytes(null)` | `{ text:'', isReadable:false, ... }` |
| Hex with odd length (e.g. `"abc"`) | No exception, treated as binary |
| `formatBytes(0)` | `"0 B"` |
| `getValidationTone` with unknown code | `"error"` |
| API call returns 404 | Axios throws, component handles gracefully |
| API call returns 500 | Axios throws, component shows error state |
| `schemathesis` fuzz against `/blocks?limit=abc` | Must return 400, not 500 |
| `schemathesis` fuzz against `/blocks/{block_num}` with huge int | Must return 400 |

### Definition of Done
- [ ] `cd ui && npm test` exits 0 with all tests passing
- [ ] `npm audit --audit-level=high` clean
- [ ] `contract-test` CI job passes
- [ ] `make ui-lint` still clean
- [ ] `ui-security` CI job passes

---

## Phase 5 — Performance, Soak Tests & Observability

**PR title:** `feat: Prometheus /metrics, load test scripts, k6 CI soak check`  
**Branch:** `feat/phase5-observability-and-perf`  
**Size:** ~200 lines production code, ~150 lines test/script code

### Goals
- Expose runtime metrics so operators can alert on ingestion lag (G-07, G-08).
- Document and automate load test baselines (load test targets).
- Complete the observability picture needed for a production on-call runbook.

### Changes

#### 5.1 — Prometheus `/metrics` endpoint (`pkg/api/`)

**Packages to add:**
```
github.com/prometheus/client_golang/prometheus
github.com/prometheus/client_golang/prometheus/promhttp
```

**Metrics to expose:**

| Metric | Type | Labels | Description |
|---|---|---|---|
| `explorer_block_height_current` | Gauge | — | Latest ingested block number |
| `explorer_pipeline_queue_depth` | Gauge | `stage={raw,processed}` | Channel occupancy |
| `explorer_blocks_ingested_total` | Counter | — | Total blocks written to DB |
| `explorer_db_pool_acquired_conns` | Gauge | — | pgxpool acquired connections |
| `explorer_http_requests_total` | Counter | `method,path,status` | REST request counts |
| `explorer_http_request_duration_seconds` | Histogram | `method,path` | REST latency |

**Implementation:**
- Create `pkg/api/metrics.go` with a `Metrics` struct holding all promethus
  objects, registered against a private registry (not `prometheus.DefaultRegisterer`)
  to avoid conflicts in test code.
- Inject `Metrics` into `Service`. Update `blockWriter` to call
  `metrics.BlockIngested(blockNum)`.
- Wrap the REST mux with a `prometheusMiddleware` that records
  `explorer_http_requests_total` and `explorer_http_request_duration_seconds`.
- Add `GET /metrics` to `newRESTRouter` returning `promhttp.HandlerFor(registry)`.
- Update `pkg/api/openapi.yaml` to document `/metrics` (text/plain, no schema).

**Config addition:**
```yaml
server:
  rest:
    metrics_enabled: true   # set false to disable /metrics (e.g. in public-facing deployments)
```

**Tests to add** (`pkg/api/metrics_test.go`):
```
TestMetricsEndpoint/returns_200_text_plain        → 200
TestMetricsEndpoint/disabled_returns_404           → 404
TestBlockHeightMetric/increments_on_ingest         → gauge updated
TestHTTPRequestCounter/increments_per_request      → counter incremented
```

#### 5.2 — Load test scripts (`scripts/load-test/`)

**Tool:** k6 (single binary, no daemon required).

**New files:**
```
scripts/load-test/smoke.js        # 5 VU × 30s — quick sanity check
scripts/load-test/load.js         # 50 VU × 5min — sustained throughput
scripts/load-test/soak.js         # 10 VU × 30min — memory/leak check
scripts/load-test/README.md       # how to run, what the thresholds mean
```

**`scripts/load-test/load.js` thresholds:**
```js
export const options = {
  vus: 50,
  duration: '5m',
  thresholds: {
    http_req_failed:           ['rate<0.01'],     // < 1% errors
    http_req_duration:         ['p(99)<500'],     // p99 < 500 ms
    'http_req_duration{path:/blocks/height}': ['p(99)<50'], // height fast
  },
};
```

**Makefile targets to add:**
```makefile
load-test-smoke: ## Quick 30s k6 smoke (requires live stack on :18080)
    k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/smoke.js

load-test-load: ## Sustained 5-min load test (requires live stack on :18080)
    k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/load.js
```

#### 5.3 — CI smoke load check (`.github/workflows/ci.yaml`)

Add a `perf-smoke` job (runs only on `push` to `main`, not on PRs to keep CI fast):
```yaml
perf-smoke:
  name: Performance Smoke
  if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  runs-on: ubuntu-latest
  timeout-minutes: 15
  steps:
    - uses: actions/checkout@...
    - uses: actions/setup-go@...
    - name: Install k6
      run: |
        sudo gpg -k; curl -s https://dl.k6.io/key.gpg | sudo gpg --dearmor \
          -o /usr/share/keyrings/k6-archive-keyring.gpg
        echo "deb [signed-by=...] https://dl.k6.io/deb stable main" | sudo tee ...
        sudo apt-get update && sudo apt-get install k6
    - name: Start live stack
      run: make swagger   # starts full stack, waits for height > 0
    - name: Run smoke load test
      run: make load-test-smoke
    - name: Stop stack
      if: always()
      run: make live-stop
```

### Corner cases / performance scenarios tested in Phase 5
| Scenario | Expected |
|---|---|
| 50 concurrent `GET /blocks?limit=500` | p99 < 500 ms, 0 errors |
| `GET /blocks/height` under load | p99 < 50 ms (no DB work) |
| 100 concurrent `GET /transactions/{id}` | No DB pool exhaustion (pool=20) |
| Pipeline running + 50 REST VU simultaneously | Ingestion lag < 5 blocks |
| Soak 30 min — goroutine count | Stable, no growth |
| Soak 30 min — heap RSS | Stable, no growth beyond 50 MB from baseline |
| `/metrics` endpoint format | Valid Prometheus text format, parseable by `curl | prom2json` |
| `explorer_block_height_current` after 100 blocks | = 100 |

### Definition of Done
- [ ] `GET /metrics` returns valid Prometheus text in `make smoke-live`
- [ ] `make load-test-smoke` passes (0 errors, p99 < 500ms) against local stack
- [ ] `perf-smoke` CI job added and green on `main`
- [ ] `scripts/load-test/README.md` documents how to run each scenario
- [ ] `make lint` clean (add `prometheus` to `depguard` allowlist)

---

## Cross-cutting: Migration safety & image tag pinning

These two items are small enough to fold into any phase PR as follow-on commits
but are called out explicitly so they are not forgotten:

### DB migration down-migration + schema version guard

**Target PR:** Add to Phase 3 PR or as its own micro-PR.

- Add `pkg/db/migrations/001_initial_schema.down.sql` (drop all tables in reverse FK order).
- Add a `SchemaVersion` constant in `pkg/db/schema.go`.
- In `ApplyMigrations`: log the schema version on startup at `INFO` level.
- Add a test: apply migrations, check version matches constant.

### docker-compose.yaml version pinning at release

**Target PR:** Add to Phase 1 PR or release workflow.

- In `.github/workflows/docker-release.yml`, after computing `VERSION`, add a step:
  ```bash
  sed -i "s|:latest|:${VERSION}|g" docker-compose.yaml
  ```
  and commit the pinned file as a release artifact (or just document that users
  should pin in the release notes).

---

## PR Sequence Summary

| PR # | Phase | Branch | Key deliverables | Blocks on |
|---|---|---|---|---|
| 1 | Phase 1 | `fix/phase1-security-hardening` | CORS config, request cap, password warn | — |
| 2 | Phase 2 | `test/phase2-rest-unit-tests` | REST handler tests, resumeBlockNum tests, coverage gate | PR 1 |
| 3 | Phase 3 | `feat/phase3-reliability` | `/readyz`, watchdog, JSON logs | PR 2 |
| 4 | Phase 4 | `test/phase4-ui-and-contract` | Vitest, API contract tests, npm audit | PR 3 |
| 5 | Phase 5 | `feat/phase5-observability-and-perf` | `/metrics`, k6 scripts, perf CI job | PR 4 |

PRs 1–3 must be merged in order. PRs 4 and 5 can be developed in parallel with each
other once PR 3 is merged.

---

## Test Coverage Targets

| Package | Current (est.) | Target |
|---|---|---|
| `pkg/config` | ~85% | ≥ 85% |
| `pkg/parser` | ~70% | ≥ 75% |
| `pkg/blockpipeline` | ~60% | ≥ 70% |
| `pkg/db` | ~65% | ≥ 70% |
| `pkg/api` | ~10% | ≥ 70% |
| `pkg/sidecarstream` | ~50% | ≥ 60% |
| `pkg/util` | ~90% | ≥ 90% |
| **Total** | **~45%** | **≥ 60%** |
| UI `lib/` | 0% | ≥ 70% |

---

## Acceptance Checklist (Production Gate)

Before declaring production-ready, every item below must be checked:

### Security
- [ ] CORS restricted to known origins in production config
- [ ] `?limit` capped server-side — verified by test
- [ ] No weak default passwords — startup warn present
- [ ] `npm audit --audit-level=high` clean
- [ ] `govulncheck ./...` clean (run manually before each release)

### Reliability
- [ ] `GET /readyz` returns 503 when DB is down
- [ ] `GET /readyz` returns 503 when pipeline is stalled
- [ ] Container restarts when Go binary crashes (watchdog)
- [ ] Graceful shutdown drains in-flight requests within `shutdown_timeout`
- [ ] Restart resume gap-healing covered by unit tests

### Observability
- [ ] JSON-formatted logs in Docker image
- [ ] `/metrics` endpoint live and scraped by Prometheus in staging
- [ ] `explorer_block_height_current` matches DB `MAX(block_num)`
- [ ] Alert configured: `block_height` not increasing for > 5 min → page

### Test & CI
- [ ] All 5 CI jobs green: lint, test, check-sqlc, integration, contract-test
- [ ] Coverage gate ≥ 60% enforced
- [ ] `make smoke-live` passes (all 12 API smoke checks)
- [ ] UI `npm test` passes
- [ ] `make load-test-smoke` passes (p99 < 500 ms, 0 errors)

### Operations
- [ ] `docker-compose.yaml` ships with pinned version tag (not `:latest`)
- [ ] `docs/ops/backup-restore.md` runbook present with tested restore steps
- [ ] `make backup` and `make restore` Makefile targets documented
