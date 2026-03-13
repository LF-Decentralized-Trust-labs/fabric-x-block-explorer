# Fabric-X Block Explorer

A lightweight block explorer for Hyperledger Fabric networks. It ingests blocks from a Fabric sidecar, writes indexed data into PostgreSQL, and exposes both gRPC and REST APIs for querying blocks, transactions, and namespace policies.

## Requirements

- Go 1.26+
- Docker with either `docker compose` or `docker-compose` available (for `make run` / DB tests)
- `grpcurl`, `curl`, and `python3` for `make smoke-live`
- Access to a running Fabric-X sidecar (gRPC) and a PostgreSQL instance

## Configuration

Explorer reads a YAML config file (for example `config.yaml`) matching the structure in `pkg/config.Config`:

- `database.endpoints[]`: host/port for PostgreSQL
- `database.user`, `database.password`, `database.dbname`, `database.max_conns`
- `database.max_conn_idle_time`, `database.max_conn_lifetime`, `database.retry`
- `sidecar.connection.endpoint`: host/port for the Fabric-X sidecar
- `sidecar.channel_id`, `sidecar.start_block`, `sidecar.end_block`, `sidecar.max_reconnect_wait`, `sidecar.retry`
- `buffer.raw_channel_size`, `buffer.proc_channel_size`
- `workers.processor_count`, `workers.writer_count`
- `server.grpc.endpoint`: gRPC bind host/port
- `server.rest.endpoint`: REST bind host/port
- `server.rest.read_header_timeout`, `server.rest.default_tx_limit`

A working example is in `config.local.yaml`.

## Running

### Local (dev)

```bash
# From repo root
cp config.local.yaml config.yaml   # or point --config to config.local.yaml

# Start the explorer
go run ./cmd/explorer start --config config.yaml
```

The REST API listens on the host/port configured under `server.rest.endpoint` (in `config.local.yaml` this is `127.0.0.1:8080`). The gRPC API listens on `server.grpc.endpoint` (default `127.0.0.1:7051`).

### With Docker Compose

```bash
# Build explorer image and start explorer + DB (sidecar must be running separately)
make run

# Tear down
make run-down
```

### Live smoke test via Make

```bash
# Recreate explorer + DB, wait for REST/gRPC readiness,
# call representative live APIs, print results, and fail on errors.
make smoke-live

# Optional helpers
make live-up
make wait-rest
make wait-grpc
make smoke-rest
make smoke-grpc
make live-down
```

## REST API

The REST server is defined in `pkg/api/rest.go` and delegates directly to the gRPC service. Paths:

- `GET /blocks/height` – returns the current stored block height
- `GET /blocks?from=&to=&limit=&offset=` – list block summaries in a range
- `GET /blocks/{block_num}?tx_limit=&tx_offset=` – block detail with paginated transactions
- `GET /transactions/{tx_id}` – transaction detail by tx ID (hex string)
- `GET /namespaces/{namespace}/policies` – namespace policies with decoded fields

All responses are JSON produced via `protojson` with `EmitUnpopulated: true`.

## gRPC API

The gRPC service is defined in `api/proto/explorer.proto` and implemented in `pkg/api/grpc.go`:

- `GetBlockHeight(google.protobuf.Empty) -> GetBlockHeightResponse`
- `ListBlocks(ListBlocksRequest) -> ListBlocksResponse`
- `GetBlockDetail(GetBlockDetailRequest) -> BlockDetail`
- `GetTransactionDetail(GetTxDetailRequest) -> TxDetail`
- `GetNamespacePolicies(GetNamespacePoliciesRequest) -> GetNamespacePoliciesResponse`

Run the server and then use `grpcurl`:

```bash
grpcurl -plaintext localhost:7051 list
grpcurl -plaintext -d '{}' localhost:7051 explorerv1.BlockExplorerService.GetBlockHeight
```

## Tests and Lint

```bash
# Parser / util / config / pipeline / sidecar / workerpool (no DB)
make test-no-db

# DB-backed tests (auto-starts local postgres container)
make test-requires-db

# All packages (requires DB container)
make test-all

# SQLC generation and verification
make sqlc
make check-sqlc

# Lint
make lint

# End-to-end live smoke test
make smoke-live
```

Unit tests for the API layer live under `pkg/api/`.
