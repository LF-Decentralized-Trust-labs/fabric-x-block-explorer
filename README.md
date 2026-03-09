# Fabric-X Block Explorer

A lightweight block explorer for Hyperledger Fabric networks. It ingests blocks from a Fabric sidecar, writes indexed data into PostgreSQL, and exposes both gRPC and REST APIs for querying blocks, transactions, and namespace policies.

## Requirements

- Go 1.21+
- Docker + docker-compose (for `make run` / DB tests)
- Access to a running Fabric-X sidecar (gRPC) and a PostgreSQL instance

## Configuration

Explorer reads a YAML config file (for example `config.yaml`) matching the structure in `pkg/config.Config`:

- `database.endpoints[]`: host/port for PostgreSQL
- `database.user`, `database.password`, `database.dbname`, `database.max_conns`
- `sidecar.connection.endpoint`: host/port for the Fabric-X sidecar
- `sidecar.channel_id`, `sidecar.start_block`, `sidecar.end_block`
- `buffer.raw_channel_size`, `buffer.proc_channel_size`
- `workers.processor_count`, `workers.writer_count`
- `server.grpc.endpoint`: gRPC bind host/port
- `server.rest.endpoint`: REST bind host/port

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
grpcurl -plaintext localhost:7051 explorerv1.BlockExplorerService/GetBlockHeight
```

## Tests and Lint

```bash
# Parser / util / config / pipeline / sidecar / workerpool (no DB)
make test-no-db

# DB-backed tests (auto-starts local postgres container)
make test-requires-db

# All packages (requires DB container)
make test-all

# Lint
make lint
```

Unit tests for the API layer live under `pkg/api/`.
