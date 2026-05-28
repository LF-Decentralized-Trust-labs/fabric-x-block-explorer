# Fabric-X Block Explorer

A lightweight block explorer for Hyperledger Fabric networks. It ingests blocks from a Fabric-X sidecar, writes indexed data into PostgreSQL, and exposes a REST API for querying blocks, transactions, and namespace policies. A Next.js web UI is included in the `ui/` directory and served as a separate container (or dev server).

```
┌─────────────────┐     gRPC      ┌──────────────────┐     SQL      ┌────────────┐
│  Fabric-X       │  ──────────►  │  Explorer        │  ─────────►  │ PostgreSQL │
│  Sidecar        │               │  (Go binary)     │  ◄─────────  │            │
└─────────────────┘               └────────┬─────────┘              └────────────┘
                                           │ REST :8080
                                           ▼
                                  ┌──────────────────┐
                                  │  Next.js UI      │
                                  │  :3000           │
                                  └──────────────────┘
```

---

## Requirements

| Tool | Version | Notes |
|---|---|---|
| Go | 1.21+ | For building the explorer binary |
| Node.js | 18+ | For the UI dev server or production build |
| npm | 9+ | UI package manager |
| Docker | any | Required for `docker compose` stack and DB tests |
| `docker compose` / `docker-compose` | v2 recommended | Used by `make run`, `make live-up`, etc. |
| `curl` + `python3` | any | Used by smoke test targets |

---

## Quickstart — Full Stack with Docker Compose

The easiest way to run the entire system (PostgreSQL + Explorer + UI) in one command. You still need a running Fabric-X sidecar to stream blocks from.

```bash
# 1. Clone the repository
git clone https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer.git
cd fabric-x-block-explorer

# 2. Edit config.docker.yaml to set your sidecar host/port if needed.
#    By default the explorer inside Docker reaches the sidecar via
#    host.docker.internal:4001 (i.e. port 4001 on your host machine).

# 3. Start postgres + explorer + UI
docker compose up --build

# Services once running:
#   postgres  → localhost:5432
#   explorer  → http://localhost:8080   (REST API + Swagger at /docs)
#   ui        → http://localhost:3000   (web interface)

# 4. Tear down
docker compose down -v
```

The `ui` container proxies all `/api/*` requests to the `explorer` container automatically via Next.js route rewrites — no extra browser-side configuration needed.

---

## Quickstart — Self-contained Live Stack (no external sidecar)

For a fully self-contained environment including a Fabric-X committer test node:

```bash
# Start the full stack, run smoke tests, and open Swagger UI
make swagger

# Tear down when done
make live-stop
```

`make swagger` builds the explorer binary, starts the committer test node, postgres, and explorer via Docker Compose, waits for the REST API to respond, runs smoke tests against all endpoints, then opens `http://127.0.0.1:18080/docs` in your browser.

To also run the UI against this stack:

```bash
# In a second terminal, after make swagger has finished
cd ui && npm ci                                    # first time only
BACKEND_URL=http://127.0.0.1:18080 npm run dev
# UI → http://localhost:3000
```

---

## Quickstart — Local Development (all components separate)

### 1. Start PostgreSQL

```bash
# Starts a postgres container on port 5433 using the committer project helper
make start-db
```

### 2. Configure the Explorer

`config.local.yaml` is pre-configured for local dev:
- PostgreSQL → `localhost:5433`
- Fabric-X sidecar → `localhost:4001` (start your sidecar separately)

### 3. Start the Explorer Backend

```bash
go run ./cmd/explorer start --config config.local.yaml
# REST API → http://127.0.0.1:8080
# Swagger  → http://127.0.0.1:8080/docs
```

### 4. Start the UI Dev Server

```bash
make ui-install                                   # npm ci inside ui/ (first time only)
BACKEND_URL=http://127.0.0.1:8080 make ui-dev    # starts Next.js with hot-reload
# UI → http://localhost:3000
```

`BACKEND_URL` is used only at dev-server startup to configure the Next.js API proxy. The browser never hits the backend directly — all requests go through `/api/*` on the Next.js server, which rewrites them to `BACKEND_URL`.

---

## Configuration Reference

The explorer reads a YAML config file passed via `--config`. See `config.local.yaml` for a fully annotated example.

### `database`
| Field | Description |
|---|---|
| `endpoints[]` | PostgreSQL host/port pairs |
| `user`, `password`, `dbname` | Connection credentials |
| `max_conns` | Maximum connection pool size |
| `max_conn_idle_time`, `max_conn_lifetime` | Pool eviction durations |
| `retry` | Exponential back-off for initial connection |
| `tls` | PostgreSQL TLS settings |

### `sidecar`
| Field | Description |
|---|---|
| `connection.endpoint` | Fabric-X sidecar `host:port` |
| `connection.tls.mode` | `none` (plaintext), `tls` (server-auth), `mtls` (mutual TLS) |
| `connection.tls.ca_cert_paths[]` | CA cert(s) — required for `tls` / `mtls` |
| `connection.tls.cert_path` | Client certificate — `mtls` only |
| `connection.tls.key_path` | Client private key — `mtls` only |
| `start_block` | Block number to begin streaming from (default `0`) |

### `buffer`
| Field | Description |
|---|---|
| `raw_channel_size` | Raw-block channel capacity between receiver and processor |
| `proc_channel_size` | Processed-block channel capacity between processor and writer |

### `workers`
| Field | Description |
|---|---|
| `processor_count` | Number of block processor goroutines |
| `writer_count` | Number of DB writer goroutines |

### `server.rest`
| Field | Description |
|---|---|
| `endpoint` | REST bind address, e.g. `127.0.0.1:8080` |
| `read_header_timeout` | Maximum time to read request headers |
| `read_timeout` | Maximum time to read the full request |
| `write_timeout` | Maximum time to write a response |
| `shutdown_timeout` | Graceful shutdown drain time |
| `default_tx_limit` | Default page size for transactions in block detail responses |

---

## REST API

All responses are JSON. CORS is enabled on all endpoints (`Access-Control-Allow-Origin: *`). The OpenAPI spec and interactive Swagger UI are always available at `/openapi.yaml` and `/docs`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/blocks/height` | Current stored block height |
| `GET` | `/blocks` | List block summaries (`offset`, `limit`) |
| `GET` | `/blocks/{block_num}` | Block detail with embedded transactions (`tx_limit`, `tx_offset`) |
| `GET` | `/transactions/{tx_id}` | Transaction detail by hex tx ID |
| `GET` | `/namespaces/policies` | Latest policy for every namespace in the DB |
| `GET` | `/namespaces/{namespace}/policies` | All policy versions for a specific namespace, newest first |
| `GET` | `/openapi.yaml` | OpenAPI 3.0 specification |
| `GET` | `/docs` | Interactive Swagger UI |

---

## Web UI

The UI (`ui/`) is a Next.js 14 app (App Router, TypeScript, Tailwind CSS).

### Pages

| Route | Description |
|---|---|
| `/` | Dashboard — live block height, connection status, recent blocks |
| `/blocks` | Paginated block list with sortable columns and block number search |
| `/blocks/{num}` | Block detail — timestamp, size, tx status summary (committed/aborted counts), hashes, commit hash, paginated transaction list with chaincode and timestamp per row |
| `/transactions/{id}` | Transaction detail — metadata (channel, chaincode, timestamp, MSP, namespaces), read/write sets with human-readable hex decode, blind writes, read-only rows, endorsements, cryptographic fields (nonce, signature, TLS cert hash) |
| `/policies` | Namespace policy explorer |

### Hex Field Decoding

Keys and values in Fabric read-write sets are raw bytes hex-encoded by the backend. The UI decodes them with the following priority:

1. **JSON** — if the bytes parse as a JSON object or array, renders a collapsible VS Code-style syntax-coloured JSON tree with a **Decoded JSON** badge and a Raw toggle button
2. **UTF-8 text** — if the bytes are printable, renders the string in cyan with the raw hex shown below
3. **Binary** — truncated orange hex string with a copy-to-clipboard button

---

## Make Targets

```
make help                # Print all targets with descriptions

# Building
make build               # Build the explorer binary → ./bin/explorer

# Testing
make test-no-db          # Tests that don't need a database (parser, config, pipeline, …)
make test-requires-db    # DB tests — auto-starts postgres if needed
make test-all            # All unit tests
make test-integration    # Integration tests against a live committer test node
make coverage            # Generate HTML coverage report → coverage/coverage.html

# Database helpers
make start-db            # Start a local postgres container on port 5433
make ensure-db           # Start postgres if not running, create 'explorer' database
make stop-db             # Remove the test postgres container

# Docker Compose stack
make run                 # Build and start postgres + explorer + ui (sidecar must be external)
make run-down            # Stop and remove the stack
make live-up             # Same as run (detached mode)
make live-down           # Stop and remove stack + volumes

# Live smoke tests (full self-contained stack)
make swagger             # Start full stack (committer + postgres + explorer), smoke-test, open Swagger
make live-stop           # Tear down the stack started by make swagger
make wait-rest           # Wait until the REST API responds at REST_BASE_URL
make smoke-rest          # Call representative REST endpoints and fail on any bad response
make smoke-live          # Recreate stack + wait-rest + smoke-rest in one shot

# UI
make ui-install          # npm ci inside ui/
make ui-dev              # Start UI dev server (requires backend on :8080)
make ui-build            # Production build of the UI
make ui-lint             # Lint the UI source

# Code generation & lint
make sqlc                # Regenerate Go code from SQL via sqlc
make check-sqlc          # Verify generated SQLC code is up to date
make lint                # Run golangci-lint
```

---

## Project Structure

```
.
├── cmd/explorer/           # Main entry point (cobra CLI)
├── pkg/
│   ├── api/                # REST server, response types, policy decoder
│   ├── blockpipeline/      # Receiver → processor → DB writer pipeline
│   ├── cli/                # cobra commands
│   ├── config/             # YAML config loader and defaults
│   ├── db/                 # PostgreSQL client, schema, sqlc generated code
│   │   ├── queries/        # Raw SQL query files
│   │   └── sqlc/           # sqlc-generated Go code (do not edit manually)
│   ├── parser/             # Fabric envelope parser (protobuf decode)
│   ├── sidecarstream/      # gRPC block stream client
│   ├── types/              # Shared domain types
│   └── util/               # Helpers (nullable, ptr, …)
├── ui/                     # Next.js 14 web interface
│   ├── app/                # App Router pages
│   │   ├── blocks/         # Block list + block detail
│   │   ├── transactions/   # Transaction detail
│   │   └── policies/       # Namespace policy viewer
│   ├── components/
│   │   ├── explorer/       # Domain-specific (MetricCard, HexField, HashValue, …)
│   │   └── ui/             # Generic (Button, Badge, Card, Loading, …)
│   ├── lib/
│   │   ├── api.ts          # Typed REST client + transform layer
│   │   └── utils.ts        # Hex decode, formatting, validation code helpers
│   └── Dockerfile          # Multi-stage Next.js production image
├── api/proto/              # Protobuf definitions
├── config.local.yaml       # Config for local dev (postgres :5433, sidecar :4001)
├── docker-compose.yaml     # Unified stack: postgres + explorer + ui
├── Dockerfile              # Explorer binary image
├── Makefile
└── sqlc.yaml               # sqlc codegen configuration
```
