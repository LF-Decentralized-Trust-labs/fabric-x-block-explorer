#!/usr/bin/env sh
set -eu

# Defaults for local/demo all-in-one runs.
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:=postgres}"
: "${POSTGRES_DB:=explorer}"
: "${SIDECAR_HOST:=host.docker.internal}"
: "${SIDECAR_PORT:=4001}"
: "${SIDECAR_TLS_MODE:=none}"
: "${REST_HOST:=0.0.0.0}"
: "${REST_PORT:=8080}"
: "${UI_PORT:=3000}"
: "${START_BLOCK:=0}"

export POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB

EXPLORER_PID=""
UI_PID=""

echo "[allinone] starting postgres..."
# postgres image helper initializes DB on first run and starts server.
/docker-entrypoint.sh postgres &
PG_PID=$!

cleanup() {
  echo "[allinone] stopping services..."
  [ -n "$EXPLORER_PID" ] && kill "$EXPLORER_PID" 2>/dev/null || true
  [ -n "$UI_PID" ] && kill "$UI_PID" 2>/dev/null || true
  kill "$PG_PID" 2>/dev/null || true
  [ -n "$EXPLORER_PID" ] && wait "$EXPLORER_PID" 2>/dev/null || true
  [ -n "$UI_PID" ] && wait "$UI_PID" 2>/dev/null || true
  wait "$PG_PID" 2>/dev/null || true
}
trap cleanup INT TERM

# Wait for postgres
for i in $(seq 1 120); do
  if pg_isready -h 127.0.0.1 -p 5432 -U "$POSTGRES_USER" >/dev/null 2>&1; then
    break
  fi
  sleep 1
  if [ "$i" -eq 120 ]; then
    echo "[allinone] postgres did not become ready in time" >&2
    exit 1
  fi
done

echo "[allinone] writing runtime explorer config..."
cat > /app/config.allinone.runtime.yaml <<EOF
database:
  endpoints:
    - host: 127.0.0.1
      port: 5432
  user: ${POSTGRES_USER}
  password: ${POSTGRES_PASSWORD}
  dbname: ${POSTGRES_DB}
  max_conns: 20

sidecar:
  connection:
    endpoint:
      host: ${SIDECAR_HOST}
      port: ${SIDECAR_PORT}
    tls:
      mode: ${SIDECAR_TLS_MODE}
  start_block: ${START_BLOCK}

buffer:
  raw_channel_size: 500
  proc_channel_size: 500

workers:
  processor_count: 4
  writer_count: 4

server:
  rest:
    endpoint:
      host: ${REST_HOST}
      port: ${REST_PORT}
    read_header_timeout: 10s
    default_tx_limit: 50
EOF

echo "[allinone] starting explorer backend..."
explorer start --config /app/config.allinone.runtime.yaml &
EXPLORER_PID=$!

echo "[allinone] starting UI..."
cd /app/ui
export NODE_ENV=production
export PORT="$UI_PORT"
export BACKEND_URL="http://127.0.0.1:${REST_PORT}"
npm start &
UI_PID=$!

echo "[allinone] ready"
while true; do
  if ! kill -0 "$PG_PID" 2>/dev/null; then
    wait "$PG_PID" || true
    echo "[allinone] postgres exited"
    exit 1
  fi
  if ! kill -0 "$EXPLORER_PID" 2>/dev/null; then
    wait "$EXPLORER_PID" || true
    echo "[allinone] explorer exited"
    exit 1
  fi
  if ! kill -0 "$UI_PID" 2>/dev/null; then
    wait "$UI_PID" || true
    echo "[allinone] ui exited"
    exit 1
  fi
  sleep 2
done
