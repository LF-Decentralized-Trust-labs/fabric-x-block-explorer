#!/bin/bash
# Script to generate test data with metadata using the configured loadgen profile.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMMITTER_MODULE="github.com/hyperledger/fabric-x-committer"
COMMITTER_VERSION="$(cd "$PROJECT_ROOT" && go list -m -f '{{.Version}}' "$COMMITTER_MODULE")"
COMMITTER_IMAGE="docker.io/hyperledger/committer-test-node:${COMMITTER_VERSION}"
CONFIG_FILE="$SCRIPT_DIR/loadgen-config.yaml"

echo "=== Fabric-X Block Explorer - Test Data Generator ==="
echo ""

# Check if committer is running
echo "Checking if committer is running..."
if ! docker ps | grep -q fabric-x-committer; then
    echo "❌ Error: fabric-x-committer container is not running"
    echo "Please start the committer first with: make dev"
    exit 1
fi

echo "✅ Committer is running"
echo ""

echo "Using committer image: $COMMITTER_IMAGE"
echo ""

# Generate test data using the current load-profile schema.
echo "=== Generating Test Data ==="
echo ""
echo "Using loadgen config: $CONFIG_FILE"
docker run --rm --network host \
    -v "$CONFIG_FILE:/config.yaml:ro" \
    "$COMMITTER_IMAGE" \
    loadgen start --config /config.yaml

echo "✅ Test data generation complete"
echo ""

echo "=== Test Data Generation Complete ==="
echo ""
echo "📊 Summary:"
echo "  - Transactions generated according to loadgen-config.yaml"
echo "  - Metadata and operation counts are defined under load-profile.transaction"
echo ""
echo "🌐 View in Block Explorer:"
echo "  - Dashboard: http://localhost:3000"
echo "  - Blocks: http://localhost:3000/blocks"
echo "  - Transactions: http://localhost:3000/transactions"
echo ""
echo "🔍 Verify metadata in database:"
echo "  psql -U postgres -d fabricx -c \"SELECT tx_id, LENGTH(metadata) as metadata_size FROM transactions WHERE metadata IS NOT NULL ORDER BY block_num DESC LIMIT 10;\""
echo ""
echo "✅ Done!"

# Made with Bob
