#!/bin/sh
#
# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Starts the Go backend in the background, then execs the Next.js UI server
# as PID 1 so Docker signals (SIGTERM/SIGINT) reach it correctly.
#
# If the Go backend exits unexpectedly, the health-check will start failing
# and the container will be restarted by the orchestrator.

set -e

# Default config path — can be overridden by mounting a file and setting
# EXPLORER_CONFIG_PATH, or by passing --config as CMD args.
CONFIG_PATH="${EXPLORER_CONFIG_PATH:-/home/explorer/config.yaml}"

echo "Starting Fabric-X Block Explorer backend on :8080"
/bin/explorer start --config "${CONFIG_PATH}" &

echo "Starting Fabric-X Block Explorer UI on :3000"
exec node /app/server.js
