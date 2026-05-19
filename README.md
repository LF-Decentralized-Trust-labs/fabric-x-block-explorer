# Fabric-X Block Explorer

A lightweight block explorer for Hyperledger Fabric networks. It ingests blocks from a Fabric-X sidecar, writes indexed data into PostgreSQL, and exposes a REST API for querying blocks, transactions, and namespace policies.

## Requirements

- Go 1.26+
- Docker with either `docker compose` or `docker-compose` available (for `make run` / DB tests)
- `curl` and `python3` for smoke tests
- Access to a running Fabric-X sidecar and a PostgreSQL instance

## Configuration

The explorer reads a YAML config file passed via `--config`. A fully annotated example is in `config.local.yaml`. The top-level keys map to `pkg/config.Config`:

### `database`
| Field | Description |
|---|---|
| `endpoints[]` | PostgreSQL host/port pairs |
| `user`, `password`, `dbname` | Connection credentials |
| `max_conns` | Maximum connection pool size |
| `max_conn_idle_time`, `max_conn_lifetime` | Pool eviction durations |
| `retry` | Exponential back-off profile for initial connection |
| `tls` | PostgreSQL TLS settings (`dbconn.DatabaseTLSConfig`) |

### `sidecar`
| Field | Description |
|---|---|
| `connection.endpoint` | Fabric-X sidecar host/port |
| `connection.retry` | gRPC-level retry policy (stream reconnection is automatic) |
| `connection.tls.mode` | `""` or `"none"` (plaintext), `"tls"` (server-auth), `"mtls"` (mutual TLS) |
| `connection.tls.ca_cert_paths[]` | CA cert(s) to verify the sidecar — required for `tls` and `mtls` |
| `connection.tls.cert_path` | Client certificate — required for `mtls` |
| `connection.tls.key_path` | Client private key — required for `mtls` |
| `start_block` | Block number to begin streaming from (default `0`) |

### `buffer`
| Field | Description |
|---|---|
| `raw_channel_size` | Capacity of the raw-block channel between receiver and processor |
| `proc_channel_size` | Capacity of the processed-block channel between processor and writer |

### `workers`
| Field | Description |
|---|---|
| `processor_count` | Number of block processor goroutines |
| `writer_count` | Number of DB writer goroutines |

### `server.rest`
| Field | Description |
|---|---|
| `endpoint` | REST bind address — e.g. `127.0.0.1:8080` |
| `read_header_timeout` | Maximum time to read request headers |
| `read_timeout` | Maximum time to read the full request |
| `write_timeout` | Maximum time to write a response |
| `shutdown_timeout` | Graceful shutdown drain time |
| `default_tx_limit` | Default page size for transactions in block detail responses |

## Running

### Local (dev)

```bash
# Start the explorer (requires a running sidecar and postgres)
go run ./cmd/explorer start --config config.local.yaml
```

The REST API listens on the address configured under `server.rest.endpoint`.

### With Docker Compose

```bash
# Start postgres + explorer (sidecar must be running separately)
make run

# Tear down
make run-down
```

### Self-contained live stack (committer + postgres + explorer)

```bash
# Build, start the full stack, run all smoke tests, then open Swagger UI in browser
make swagger

# Tear down the stack started by 'make swagger'
make live-stop
```

### Individual stack helpers

```bash
make live-up      # Start the docker-compose stack in detached mode
make live-down    # Stop and remove containers + volumes
make wait-rest    # Block until the REST API is responding
make smoke-rest   # Run the REST smoke-test suite only
make smoke-live   # wait-rest + smoke-rest combined
```

## REST API

The REST server is defined in `pkg/api/rest.go`. All responses are JSON. Response types are defined in `pkg/api/types.go`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/blocks/height` | Current stored block height |
| `GET` | `/blocks` | List block summaries (`from`, `to`, `limit`, `offset`) |
| `GET` | `/blocks/{block_num}` | Block detail with transactions (`tx_limit`, `tx_offset`) |
| `GET` | `/transactions/{tx_id}` | Transaction detail by hex tx ID |
| `GET` | `/namespaces/policies` | Latest policy for **every** namespace in the DB |
| `GET` | `/namespaces/{namespace}/policies` | All policy versions for a specific namespace, newest first |
| `GET` | `/openapi.yaml` | OpenAPI 3.0 specification (server URL injected dynamically from request host) |
| `GET` | `/docs` | Interactive Swagger UI |

CORS is enabled on all endpoints (`Access-Control-Allow-Origin: *`).

## Tests and Lint

```bash
# Build the explorer binary
make build

# Parser / util / config / pipeline / sidecar (no DB required)
make test-no-db

# DB-backed unit tests (auto-starts a local postgres container)
make test-requires-db

# All unit tests
make test-all

# Integration test against a live committer-test-node container
make test-integration

# Coverage report
make coverage

# SQLC codegen and verification
make sqlc
make check-sqlc

# Lint
make lint

# Full live smoke test (full stack, all endpoints)
make smoke-live
```

Policy decoder tests live in `pkg/api/`. Configuration tests live in `pkg/config/`.
