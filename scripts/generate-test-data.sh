#!/bin/bash
# Script to generate test data with metadata using loadgen
# Requires fabric-x-committer v1.0.3+ to be running

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

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

# Check committer version
echo "Checking committer version..."
COMMITTER_VERSION=$(docker ps --format '{{.Image}}' | grep fabric-x-committer | cut -d: -f2)
echo "Committer version: $COMMITTER_VERSION"

if [[ "$COMMITTER_VERSION" < "1.0.3" ]]; then
    echo "⚠️  Warning: Committer version is less than 1.0.3. Metadata feature may not be available."
fi
echo ""

# Generate test data with different metadata sizes
echo "=== Generating Test Data ==="
echo ""

# Scenario 1: Transactions with 256-byte metadata
echo "1️⃣  Generating 20 transactions with 256-byte metadata..."
docker run --rm --network host \
    -v "$SCRIPT_DIR/loadgen-with-metadata.yaml:/config.yaml" \
    hyperledger/fabric-x-committer:1.0.3 \
    loadgen --config /config.yaml \
    --workload.transactions 20 \
    --workload.metadata-size 256 \
    --workload.namespaces '[{"name":"test-256","keys":50}]'

echo "✅ Generated 20 transactions with 256-byte metadata"
echo ""

# Scenario 2: Transactions with 512-byte metadata
echo "2️⃣  Generating 15 transactions with 512-byte metadata..."
docker run --rm --network host \
    -v "$SCRIPT_DIR/loadgen-with-metadata.yaml:/config.yaml" \
    hyperledger/fabric-x-committer:1.0.3 \
    loadgen --config /config.yaml \
    --workload.transactions 15 \
    --workload.metadata-size 512 \
    --workload.namespaces '[{"name":"test-512","keys":50}]'

echo "✅ Generated 15 transactions with 512-byte metadata"
echo ""

# Scenario 3: Transactions with 1KB metadata
echo "3️⃣  Generating 10 transactions with 1KB metadata..."
docker run --rm --network host \
    -v "$SCRIPT_DIR/loadgen-with-metadata.yaml:/config.yaml" \
    hyperledger/fabric-x-committer:1.0.3 \
    loadgen --config /config.yaml \
    --workload.transactions 10 \
    --workload.metadata-size 1024 \
    --workload.namespaces '[{"name":"test-1kb","keys":50}]'

echo "✅ Generated 10 transactions with 1KB metadata"
echo ""

# Scenario 4: Transactions WITHOUT metadata (for comparison)
echo "4️⃣  Generating 10 transactions WITHOUT metadata..."
docker run --rm --network host \
    -v "$SCRIPT_DIR/loadgen-with-metadata.yaml:/config.yaml" \
    hyperledger/fabric-x-committer:1.0.3 \
    loadgen --config /config.yaml \
    --workload.transactions 10 \
    --workload.metadata-size 0 \
    --workload.namespaces '[{"name":"test-no-metadata","keys":50}]'

echo "✅ Generated 10 transactions WITHOUT metadata"
echo ""

# Scenario 5: Large metadata (2KB)
echo "5️⃣  Generating 5 transactions with 2KB metadata..."
docker run --rm --network host \
    -v "$SCRIPT_DIR/loadgen-with-metadata.yaml:/config.yaml" \
    hyperledger/fabric-x-committer:1.0.3 \
    loadgen --config /config.yaml \
    --workload.transactions 5 \
    --workload.metadata-size 2048 \
    --workload.namespaces '[{"name":"test-2kb","keys":50}]'

echo "✅ Generated 5 transactions with 2KB metadata"
echo ""

echo "=== Test Data Generation Complete ==="
echo ""
echo "📊 Summary:"
echo "  - 20 transactions with 256-byte metadata"
echo "  - 15 transactions with 512-byte metadata"
echo "  - 10 transactions with 1KB metadata"
echo "  - 10 transactions WITHOUT metadata"
echo "  - 5 transactions with 2KB metadata"
echo "  - Total: 60 transactions"
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
