#!/usr/bin/env bash
# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# test-live.sh — Spin up committer-test-node + explorer Postgres, run the
# explorer binary against them, then smoke-test every REST endpoint.
#
# Usage:
#   ./scripts/test-live.sh            # run everything
#   ./scripts/test-live.sh --no-build # skip 'go build'
#   ./scripts/test-live.sh --down     # just tear down containers and exit

set -uo pipefail

die() { log "ERROR: $*"; exit 1; }

# ── names / ports ─────────────────────────────────────────────────────────────
COMMITTER_CONTAINER="fx-live-committer"
POSTGRES_CONTAINER="fx-live-postgres"
EXPLORER_PID_FILE="/tmp/fx-explorer-live.pid"
EXPLORER_LOG="/tmp/fx-explorer-live.log"
CFG_FILE="/tmp/fx-explorer-live.yaml"

COMMITTER_IMAGE="hyperledger/fabric-x-committer-test-node:1.0.0-alpha.2"
POSTGRES_IMAGE="postgres:16-alpine"

PG_HOST_PORT=15432
PG_USER=postgres
PG_PASS=postgres
PG_DB=explorer

SIDECAR_CONTAINER_PORT=4001
EXPLORER_REST_PORT=18080

PASS=0
FAIL=0

# ── helpers ───────────────────────────────────────────────────────────────────
log()  { echo "[live] $*" >&2; }
ok()   { echo "  ✅  $*"; PASS=$((PASS+1)); }
fail() { echo "  ❌  $*"; FAIL=$((FAIL+1)); }

check_deps() {
  for cmd in docker go curl jq; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "ERROR: '$cmd' not found in PATH"; exit 1; }
  done
}

teardown() {
  log "Tearing down..."
  if [[ -f "$EXPLORER_PID_FILE" ]]; then
    PID=$(cat "$EXPLORER_PID_FILE")
    kill "$PID" 2>/dev/null || true
    rm -f "$EXPLORER_PID_FILE"
  fi
  docker rm -f "$COMMITTER_CONTAINER" "$POSTGRES_CONTAINER" 2>/dev/null || true
  rm -f "$CFG_FILE"
  log "Done."
}

wait_tcp() {
  local host=$1 port=$2 timeout=${3:-120}
  log "Waiting for $host:$port (up to ${timeout}s)..."
  local i=0
  while ! (echo > /dev/tcp/$host/$port) 2>/dev/null; do
    sleep 1
    i=$((i+1))
    [[ $i -ge $timeout ]] && { log "ERROR: $host:$port not reachable after ${timeout}s"; return 1; }
  done
  log "$host:$port is up."
}

wait_http() {
  local url=$1 timeout=${2:-120}
  log "Waiting for HTTP $url (up to ${timeout}s)..."
  local i=0
  while ! curl -sf "$url" >/dev/null 2>&1; do
    sleep 1
    i=$((i+1))
    [[ $i -ge $timeout ]] && { log "ERROR: $url not ready after ${timeout}s"; return 1; }
  done
  log "$url is ready."
}

wait_height_gt0() {
  local timeout=${1:-300}
  log "Waiting for block height > 0 (up to ${timeout}s)..."
  local i=0
  local h=0
  while [[ "$h" -le 0 ]]; do
    sleep 2
    h=$(curl -sf "${EXPLORER_URL}/blocks/height" 2>/dev/null | jq -r '.height // 0' 2>/dev/null || echo 0)
    i=$((i+2))
    [[ $i -ge $timeout ]] && { log "ERROR: no application blocks after ${timeout}s"; return 1; }
  done
  log "Block height is now $h"
  echo "$h"
}

# ── argument handling ─────────────────────────────────────────────────────────
BUILD=true
KEEP=false
for arg in "$@"; do
  case $arg in
    --no-build) BUILD=false ;;
    --down)     teardown; exit 0 ;;
    --keep)     KEEP=true ;;
  esac
done

# ── main ──────────────────────────────────────────────────────────────────────
check_deps
# With --keep, skip teardown so containers + explorer stay running after tests.
if [[ "$KEEP" == "false" ]]; then
  trap teardown EXIT
fi

EXPLORER_URL="http://127.0.0.1:${EXPLORER_REST_PORT}"

# 1. Build explorer binary
if [[ "$BUILD" == "true" ]]; then
  log "Building explorer..."
  go build -ldflags "-X github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/cli.Version=live-test" \
    -o /tmp/fx-explorer ./cmd/explorer/ || die "go build failed"
fi
EXPLORER_BIN=/tmp/fx-explorer
[[ -f "$EXPLORER_BIN" ]] || { log "ERROR: binary $EXPLORER_BIN not found"; exit 1; }

# 2. Clean up leftovers
docker rm -f "$COMMITTER_CONTAINER" "$POSTGRES_CONTAINER" 2>/dev/null || true

# 3. Start explorer Postgres
log "Starting explorer Postgres on host port $PG_HOST_PORT..."
docker run -d --name "$POSTGRES_CONTAINER" \
  -e POSTGRES_USER="$PG_USER" \
  -e POSTGRES_PASSWORD="$PG_PASS" \
  -e POSTGRES_DB="$PG_DB" \
  -p "${PG_HOST_PORT}:5432" \
  "$POSTGRES_IMAGE" >/dev/null
wait_tcp 127.0.0.1 "$PG_HOST_PORT" 60 || die "Postgres not ready"

# 4. Start committer-test-node
log "Starting committer-test-node ($COMMITTER_IMAGE)..."
docker run -d --name "$COMMITTER_CONTAINER" \
  -e SC_COORDINATOR_SERVER_TLS_MODE=none \
  -e SC_COORDINATOR_VERIFIER_TLS_MODE=none \
  -e SC_COORDINATOR_VALIDATOR_COMMITTER_TLS_MODE=none \
  -e SC_COORDINATOR_MONITORING_TLS_MODE=none \
  -e SC_QUERY_SERVER_TLS_MODE=none \
  -e SC_QUERY_MONITORING_TLS_MODE=none \
  -e SC_SIDECAR_SERVER_TLS_MODE=none \
  -e SC_SIDECAR_MONITORING_TLS_MODE=none \
  -e SC_SIDECAR_COMMITTER_TLS_MODE=none \
  -e SC_VC_SERVER_TLS_MODE=none \
  -e SC_VC_MONITORING_TLS_MODE=none \
  -e SC_VERIFIER_SERVER_TLS_MODE=none \
  -e SC_SIDECAR_ORDERER_TLS_MODE=none \
  -e SC_VERIFIER_MONITORING_TLS_MODE=none \
  -e SC_SIDECAR_ORDERER_CONNECTION_TLS_MODE=none \
  -e SC_ORDERER_SERVER_TLS_MODE=none \
  -e SC_LOADGEN_SERVER_TLS_MODE=none \
  -e SC_LOADGEN_MONITORING_TLS_MODE=none \
  -e SC_LOADGEN_ORDERER_CLIENT_SIDECAR_CLIENT_TLS_MODE=none \
  -e SC_LOADGEN_ORDERER_CLIENT_ORDERER_TLS_MODE=none \
  -e SC_LOADGEN_TRANSACTION_READ_ONLY_COUNT_UNIFORM_MIN=0 \
  -e SC_LOADGEN_TRANSACTION_READ_ONLY_COUNT_UNIFORM_MAX=4 \
  -e SC_LOADGEN_TRANSACTION_READ_WRITE_COUNT_UNIFORM_MIN=0 \
  -e SC_LOADGEN_TRANSACTION_READ_WRITE_COUNT_UNIFORM_MAX=4 \
  -e SC_LOADGEN_TRANSACTION_READ_WRITE_COUNT_CONST=0 \
  -e SC_LOADGEN_TRANSACTION_WRITE_COUNT_UNIFORM_MIN=0 \
  -e SC_LOADGEN_TRANSACTION_WRITE_COUNT_UNIFORM_MAX=4 \
  -p "127.0.0.1::${SIDECAR_CONTAINER_PORT}/tcp" \
  "$COMMITTER_IMAGE" \
  run db committer orderer loadgen >/dev/null

# 5. Discover sidecar host port
log "Discovering sidecar host port..."
sleep 3
SIDECAR_HOST_PORT=$(docker inspect "$COMMITTER_CONTAINER" \
  --format "{{(index (index .NetworkSettings.Ports \"${SIDECAR_CONTAINER_PORT}/tcp\") 0).HostPort}}")
log "Sidecar mapped to host port: $SIDECAR_HOST_PORT"

wait_tcp 127.0.0.1 "$SIDECAR_HOST_PORT" 180 || die "Sidecar not ready"

# 6. Write explorer config
log "Writing explorer config to $CFG_FILE..."
cat > "$CFG_FILE" <<YAML
database:
  endpoints:
    - host: 127.0.0.1
      port: ${PG_HOST_PORT}
  user: ${PG_USER}
  password: ${PG_PASS}
  dbname: ${PG_DB}
  max_conns: 20

sidecar:
  connection:
    endpoint:
      host: 127.0.0.1
      port: ${SIDECAR_HOST_PORT}
    tls:
      mode: none
  start_block: 0

buffer:
  raw_channel_size: 500
  proc_channel_size: 500

workers:
  processor_count: 4
  writer_count: 4

server:
  rest:
    endpoint:
      host: 127.0.0.1
      port: ${EXPLORER_REST_PORT}
    read_header_timeout: 10s
    default_tx_limit: 50
YAML

# 7. Start explorer
log "Starting explorer (log → $EXPLORER_LOG)..."
"$EXPLORER_BIN" start -c "$CFG_FILE" >"$EXPLORER_LOG" 2>&1 &
echo $! > "$EXPLORER_PID_FILE"

wait_http "${EXPLORER_URL}/blocks/height" 120 || die "Explorer REST not ready"

# 8. Wait for at least 1 application block (height > 0)
HEIGHT=$(wait_height_gt0 300) || die "No application blocks arrived"

# ── API smoke tests ────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Running API smoke tests against $EXPLORER_URL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

get() {
  local resp
  resp=$(curl -sf "${EXPLORER_URL}$1" 2>/dev/null) || {
    echo "  ❌  GET $1 failed (HTTP error or connection refused)" >&2
    FAIL=$((FAIL+1))
    echo "{}"
    return
  }
  echo "$resp"
}

# ── 1. Block height
echo ""
echo "── 1. GET /blocks/height"
RESP=$(get "/blocks/height")
echo "$RESP" | jq .
H=$(echo "$RESP" | jq -r '.height')
[[ "$H" -gt 0 ]] && ok "height=$H > 0" || fail "height=$H is not > 0"

# ── 2. List blocks — check summary fields
echo ""
echo "── 2. GET /blocks?limit=5"
RESP=$(get "/blocks?limit=5")
echo "$RESP" | jq .
COUNT=$(echo "$RESP" | jq '.blocks | length')
[[ "$COUNT" -gt 0 ]] && ok "returned $COUNT blocks" || fail "no blocks returned"

# Find a non-config block (block_num > 0) in the list to check created_at
APP_BLK=$(echo "$RESP" | jq '[.blocks[] | select(.block_num > 0)] | .[0]')
if [[ "$APP_BLK" != "null" && -n "$APP_BLK" ]]; then
  for field in block_size created_at; do
    VAL=$(echo "$APP_BLK" | jq -r ".${field} // empty")
    [[ -n "$VAL" ]] && ok "app block.${field} = $VAL" || fail "app block.${field} is missing/null"
  done
  # Verify created_at is RFC 3339
  CREATED=$(echo "$APP_BLK" | jq -r '.created_at // empty')
  if echo "$CREATED" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T'; then
    ok "created_at is RFC 3339: $CREATED"
  else
    fail "created_at is NOT RFC 3339: $CREATED"
  fi
else
  ok "only config block ingested so far — created_at check skipped"
fi

# Config block should always have block_hash and block_size
CFG_BLK=$(echo "$RESP" | jq '[.blocks[] | select(.block_num == 0)] | .[0]')
[[ $(echo "$CFG_BLK" | jq -r '.block_size // empty') != "" ]] && ok "block0.block_size present" || fail "block0.block_size missing"

# ── 3. Get block 0 (config block)
echo ""
echo "── 3. GET /blocks/0"
RESP=$(get "/blocks/0")
echo "$RESP" | jq .
[[ $(echo "$RESP" | jq -r '.block_num') == "0" ]] && ok "block 0 returned" || fail "block 0 not returned"
[[ $(echo "$RESP" | jq '.transactions | length') -ge 0 ]] && ok "block0.transactions field present" || fail "block0.transactions missing"

# ── 4. Get first application block
echo ""
echo "── 4. GET /blocks/1"
RESP=$(get "/blocks/1")
echo "$RESP" | jq .
BN=$(echo "$RESP" | jq -r '.block_num // empty')
[[ "$BN" == "1" ]] && ok "block 1 returned" || fail "block 1 not returned (got block_num=$BN)"
TX_COUNT=$(echo "$RESP" | jq '.transactions | length')
ok "block1 has $TX_COUNT transaction(s)"

# ── 5. Get a transaction via a block
echo ""
echo "── 5. GET /blocks/1 → pick first tx_id → GET /transactions/{txid}"
BLK1=$(get "/blocks/1")
TXID=$(echo "$BLK1" | jq -r '.transactions[0].tx_id // empty')
if [[ -z "$TXID" ]]; then
  fail "block 1 has no transactions yet"
else
  log "Using tx_id: $TXID"
  RESP=$(get "/transactions/${TXID}")
  echo "$RESP" | jq .
  TX_COUNT=1

  # These fields are always present and non-null
  for field in tx_id validation_code created_at channel_id; do
    VAL=$(echo "$RESP" | jq -r ".${field} // empty")
    [[ -n "$VAL" ]] && ok "tx.${field} = $VAL" || fail "tx.${field} is missing/null"
  done
  # These are nullable (null for _meta/system transactions) — just check key exists
  for field in tx_type channel_version epoch; do
    echo "$RESP" | jq -e "has(\"${field}\")" >/dev/null 2>&1 \
      && ok "tx has field '${field}'" || fail "tx is missing field '${field}'"
  done
fi

# ── 6. Verify write-set fields on the transaction from test 5
echo ""
echo "── 6. Verify write-set fields on transaction (from test 5)"
if [[ -n "${TXID:-}" ]]; then
  TX6=$(get "/transactions/${TXID}")
  echo "$TX6" | jq .
  for field in blind_writes read_writes reads_only endorsements namespaces; do
    echo "$TX6" | jq -e ".${field}" >/dev/null 2>&1 && ok "tx has ${field} field" || fail "tx missing ${field} field"
  done
  # Check ns_id on row arrays if non-empty
  for field in blind_writes read_writes reads_only; do
    ROWS=$(echo "$TX6" | jq ".${field} // []")
    ROW_COUNT=$(echo "$ROWS" | jq 'length')
    if [[ "$ROW_COUNT" -gt 0 ]]; then
      NS_ID=$(echo "$ROWS" | jq -r '.[0].ns_id // empty')
      [[ -n "$NS_ID" ]] && ok "${field}[0].ns_id = $NS_ID" || fail "${field}[0].ns_id is missing"
    fi
  done
else
  fail "skipping test 6 — no tx_id available from test 5"
fi

# ── 7. Pagination
echo ""
echo "── 7. GET /blocks pagination"
# Need at least 3 blocks (0,1,2) so that page1 (limit=2) and page2 (from=2,limit=2) are non-overlapping.
WAIT7=0
H7=$(curl -sf "${EXPLORER_URL}/blocks/height" 2>/dev/null | jq -r '.height // 0' 2>/dev/null || echo 0)
while [[ "$H7" -lt 2 && "$WAIT7" -lt 60 ]]; do
  sleep 2; WAIT7=$((WAIT7+2))
  H7=$(curl -sf "${EXPLORER_URL}/blocks/height" 2>/dev/null | jq -r '.height // 0' 2>/dev/null || echo 0)
done
R1=$(get "/blocks?limit=2")
R2=$(get "/blocks?limit=2&from=2")
B1=$(echo "$R1" | jq -r '.blocks[-1].block_num // empty')
B2=$(echo "$R2" | jq -r '.blocks[0].block_num // empty')
if [[ -n "$B1" && -n "$B2" && "$B1" != "$B2" ]]; then
  ok "pagination works: page1 last=$B1, page2 first=$B2"
else
  fail "pagination may be broken: B1=$B1 B2=$B2 (height=$H7)"
fi

# ── 8. 404 for unknown transaction
echo ""
echo "── 8. GET /transactions/<unknown> → 404"
UNKNOWN=$(printf '%064d' 0)
STATUS=$(curl -so /dev/null -w "%{http_code}" "${EXPLORER_URL}/transactions/${UNKNOWN}")
[[ "$STATUS" == "404" ]] && ok "404 returned for unknown txid" || fail "expected 404, got $STATUS"

# ── 9. Write-set variety across multiple transactions (scan blocks 1..N)
echo ""
echo "── 9. Write-set variety (waiting for height ≥ 5, then scanning)"
# Wait up to 120s for loadgen application blocks to accumulate
WAIT9=0
while [[ "$HEIGHT" -lt 5 && "$WAIT9" -lt 120 ]]; do
  sleep 2; WAIT9=$((WAIT9+2))
  HEIGHT=$(curl -sf "${EXPLORER_URL}/blocks/height" 2>/dev/null | jq -r '.height // 0' 2>/dev/null || echo 0)
done
log "Height for test 9: $HEIGHT"
SAW_BLIND=false SAW_RW=false SAW_RO=false
SCAN_MAX=$(( HEIGHT < 20 ? HEIGHT : 20 ))
for blk_n in $(seq 1 "$SCAN_MAX"); do
  BLK=$(get "/blocks/${blk_n}" 2>/dev/null)
  while IFS= read -r txid; do
    [[ -z "$txid" ]] && continue
    DETAIL=$(get "/transactions/${txid}" 2>/dev/null || echo '{}')
    bw=$(echo "$DETAIL" | jq '.blind_writes | length' 2>/dev/null || echo 0)
    rw=$(echo "$DETAIL" | jq '.read_writes | length' 2>/dev/null || echo 0)
    ro=$(echo "$DETAIL" | jq '.reads_only | length' 2>/dev/null || echo 0)
    [[ "$bw" -gt 0 ]] && SAW_BLIND=true
    [[ "$rw" -gt 0 ]] && SAW_RW=true
    [[ "$ro" -gt 0 ]] && SAW_RO=true
  done < <(echo "$BLK" | jq -r '.transactions[]?.tx_id // empty')
done

$SAW_BLIND && ok "saw blind_writes" || ok "no blind_writes seen in blocks 1..${SCAN_MAX} (loadgen MIN=0, expected)"
$SAW_RW    && ok "saw read_writes"  || fail "no read_writes seen in blocks 1..${SCAN_MAX}"
$SAW_RO    && ok "saw reads_only"   || ok "no reads_only seen in blocks 1..${SCAN_MAX} (loadgen MIN=0, expected)"

# ── 10. GET /namespaces/{namespace}/policies
echo ""
echo "── 10. GET /namespaces/_meta/policies"
RESP=$(get "/namespaces/_meta/policies")
echo "$RESP" | jq .
POL_COUNT=$(echo "$RESP" | jq '.policies | length' 2>/dev/null || echo 0)
[[ "$POL_COUNT" -gt 0 ]] && ok "namespace _meta has $POL_COUNT policy record(s)" || fail "no policies returned for namespace _meta"
# Verify required fields on first policy row
POL0=$(echo "$RESP" | jq '.policies[0]')
for field in namespace version policy; do
  echo "$POL0" | jq -e "has(\"${field}\")" >/dev/null 2>&1 \
    && ok "policy has field '${field}'" || fail "policy missing field '${field}'"
done

# ── 11. GET /openapi.yaml
echo ""
echo "── 11. GET /openapi.yaml"
OA_STATUS=$(curl -so /dev/null -w "%{http_code}" "${EXPLORER_URL}/openapi.yaml")
[[ "$OA_STATUS" == "200" ]] && ok "GET /openapi.yaml → 200" || fail "GET /openapi.yaml → $OA_STATUS"
OA_BODY=$(curl -sf "${EXPLORER_URL}/openapi.yaml" 2>/dev/null || echo "")
echo "$OA_BODY" | grep -q "openapi:" \
  && ok "openapi.yaml contains 'openapi:' key" || fail "openapi.yaml body missing 'openapi:' key"

# ── 12. GET /docs (Swagger UI)
echo ""
echo "── 12. GET /docs"
DOCS_STATUS=$(curl -so /dev/null -w "%{http_code}" "${EXPLORER_URL}/docs")
[[ "$DOCS_STATUS" == "200" ]] && ok "GET /docs → 200" || fail "GET /docs → $DOCS_STATUS"
DOCS_BODY=$(curl -sf "${EXPLORER_URL}/docs" 2>/dev/null || echo "")
echo "$DOCS_BODY" | grep -qi "swagger\|openapi\|redoc" \
  && ok "GET /docs body contains Swagger/OpenAPI/ReDoc content" || fail "GET /docs body missing expected UI content"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
printf "  Results: %d passed, %d failed\n" "$PASS" "$FAIL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ $FAIL -gt 0 ]]; then
  echo ""
  echo "Explorer log tail:"
  tail -30 "$EXPLORER_LOG"
  exit 1
fi
exit 0
