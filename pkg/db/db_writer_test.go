/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteProcessedBlock tests writing a complete block with all components
func TestWriteProcessedBlock(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	// Create test data
	txID := "abc123def456"
	txIDBytes, err := hex.DecodeString(txID)
	require.NoError(t, err)

	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{
			{
				BlockNum:       1,
				TxNum:          0,
				TxID:           txID,
				NsID:           "mycc",
				NsVersion:      1,
				ValidationCode: 0,
			},
		},
		Writes: []types.WriteRecord{
			{
				Namespace:      "mycc",
				Key:            "key1",
				BlockNum:       1,
				TxNum:          0,
				Value:          []byte("value1"),
				TxID:           txID,
				ValidationCode: 0,
				IsBlindWrite:   false,
				ReadVersion:    uint64Ptr(10),
			},
		},
		Reads: []types.ReadRecord{
			{
				BlockNum:    1,
				TxNum:       0,
				NsID:        "mycc",
				Key:         "key1",
				Version:     uint64Ptr(10),
				IsReadWrite: true,
			},
		},
		Endorsements: []types.EndorsementRecord{
			{
				BlockNum:    1,
				TxNum:       0,
				NsID:        "mycc",
				Endorsement: []byte("endorsement_sig"),
				MspID:       strPtr("Org1MSP"),
				Identity:    []byte(`{"mspid":"Org1MSP","id_bytes":"cert"}`),
			},
		},
		Policies: []types.NamespacePolicyRecord{
			{
				Namespace:  "mycc",
				Version:    1,
				PolicyJSON: json.RawMessage(`{"policy_bytes":"cG9saWN5"}`),
			},
		},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       1,
			PreviousHash: []byte("prevhash"),
			DataHash:     []byte("datahash"),
		},
		Data: parsedData,
		Txns: 1,
	}

	// Write the block
	writer := NewBlockWriter(env.Pool)
	err = writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	// Verify block was written
	block, err := env.Queries.GetBlock(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), block.BlockNum)
	assert.Equal(t, int32(1), block.TxCount)
	assert.Equal(t, []byte("prevhash"), block.PreviousHash)
	assert.Equal(t, []byte("datahash"), block.DataHash)

	// Verify transaction was written
	tx, err := env.Queries.GetValidationCodeByTxID(ctx, txIDBytes)
	require.NoError(t, err)
	assert.Equal(t, int64(1), tx.BlockNum)
	assert.Equal(t, int64(0), tx.TxNum)

	// Verify counts
	assert.Equal(t, int64(1), env.GetBlockCount(t))
	assert.Equal(t, int64(1), env.GetTransactionCount(t))
}

// TestWriteProcessedBlockWithBlindWrites tests writing blind writes
func TestWriteProcessedBlockWithBlindWrites(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	txID := "deadbeef"
	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{
			{
				BlockNum:       2,
				TxNum:          0,
				TxID:           txID,
				NsID:           "testcc",
				NsVersion:      1,
				ValidationCode: 0,
			},
		},
		Writes: []types.WriteRecord{
			{
				Namespace:      "testcc",
				Key:            "blindkey",
				BlockNum:       2,
				TxNum:          0,
				Value:          []byte("blindvalue"),
				TxID:           txID,
				ValidationCode: 0,
				IsBlindWrite:   true,
				ReadVersion:    nil, // Blind writes have no read version
			},
		},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies:     []types.NamespacePolicyRecord{},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       2,
			PreviousHash: []byte("prev2"),
			DataHash:     []byte("data2"),
		},
		Data: parsedData,
		Txns: 1,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	// Verify block exists
	env.AssertBlockExists(t, 2)

	// Query the write to verify blind write flag
	var isBlindWrite bool
	err = env.Pool.QueryRow(ctx, `
		SELECT is_blind_write 
		FROM tx_writes tw
		JOIN tx_namespaces tn ON tw.tx_namespace_id = tn.id
		WHERE tn.ns_id = $1 AND tw.key = $2
	`, "testcc", []byte("blindkey")).Scan(&isBlindWrite)
	require.NoError(t, err)
	assert.True(t, isBlindWrite)
}

// TestWriteProcessedBlockMultipleTransactions tests multiple transactions in one block
func TestWriteProcessedBlockMultipleTransactions(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{
			{
				BlockNum:       3,
				TxNum:          0,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000001",
				NsID:           "cc1",
				NsVersion:      1,
				ValidationCode: 0,
			},
			{
				BlockNum:       3,
				TxNum:          1,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000002",
				NsID:           "cc2",
				NsVersion:      1,
				ValidationCode: 0,
			},
		},
		Writes: []types.WriteRecord{
			{
				Namespace:      "cc1",
				Key:            "key1",
				BlockNum:       3,
				TxNum:          0,
				Value:          []byte("val1"),
				TxID:           "0000000000000000000000000000000000000000000000000000000000000001",
				ValidationCode: 0,
				IsBlindWrite:   false,
			},
			{
				Namespace:      "cc2",
				Key:            "key2",
				BlockNum:       3,
				TxNum:          1,
				Value:          []byte("val2"),
				TxID:           "0000000000000000000000000000000000000000000000000000000000000002",
				ValidationCode: 0,
				IsBlindWrite:   false,
			},
		},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies:     []types.NamespacePolicyRecord{},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       3,
			PreviousHash: []byte("prev3"),
			DataHash:     []byte("data3"),
		},
		Data: parsedData,
		Txns: 2,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	// Verify both transactions were written
	assert.Equal(t, int64(2), env.GetTransactionCount(t))

	// Verify both namespaces exist
	var count int64
	err = env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tx_namespaces").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// TestWriteProcessedBlockNilBlock tests error handling for nil block
func TestWriteProcessedBlockNilBlock(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// TestWriteProcessedBlockInvalidData tests error handling for invalid data type
func TestWriteProcessedBlockInvalidData(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number: 1,
		},
		Data: "invalid_data_type", // Wrong type
		Txns: 0,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not *types.ParsedBlockData")
}

// TestWriteProcessedBlockWithPolicies tests policy upsert functionality
func TestWriteProcessedBlockWithPolicies(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	policyJSON := json.RawMessage(`{"policy_bytes":"base64encodedpolicy"}`)

	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{},
		Writes:       []types.WriteRecord{},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies: []types.NamespacePolicyRecord{
			{
				Namespace:  "mycc",
				Version:    1,
				PolicyJSON: policyJSON,
			},
		},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       4,
			PreviousHash: []byte("prev4"),
			DataHash:     []byte("data4"),
		},
		Data: parsedData,
		Txns: 0,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	// Verify policy was written
	policies, err := env.Queries.GetNamespacePolicies(ctx, "mycc")
	require.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "mycc", policies[0].Namespace)
	assert.Equal(t, int64(1), policies[0].Version)

	// Test upsert - update with new version
	parsedData2 := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{},
		Writes:       []types.WriteRecord{},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies: []types.NamespacePolicyRecord{
			{
				Namespace:  "mycc",
				Version:    2,
				PolicyJSON: json.RawMessage(`{"policy_bytes":"updated"}`),
			},
		},
	}

	processedBlock2 := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       5,
			PreviousHash: []byte("prev5"),
			DataHash:     []byte("data5"),
		},
		Data: parsedData2,
		Txns: 0,
	}

	err = writer.WriteProcessedBlock(ctx, processedBlock2)
	require.NoError(t, err)

	// Verify policy was updated (should have 2 versions now)
	policies, err = env.Queries.GetNamespacePolicies(ctx, "mycc")
	require.NoError(t, err)
	assert.Len(t, policies, 2)
}

// TestWriteProcessedBlockRollbackOnError tests transaction rollback on error
func TestWriteProcessedBlockRollbackOnError(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	// Create block with invalid hex txID to trigger error
	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{
			{
				BlockNum:       6,
				TxNum:          0,
				TxID:           "invalid_hex_ZZZ", // Invalid hex string
				NsID:           "testcc",
				NsVersion:      1,
				ValidationCode: 0,
			},
		},
		Writes:       []types.WriteRecord{},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies:     []types.NamespacePolicyRecord{},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       6,
			PreviousHash: []byte("prev6"),
			DataHash:     []byte("data6"),
		},
		Data: parsedData,
		Txns: 1,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode tx_id")

	// Verify block was NOT written (transaction rolled back)
	env.AssertBlockNotExists(t, 6)
}

// TestNewBlockWriter tests BlockWriter constructors
func TestNewBlockWriter(t *testing.T) {
	env := NewDatabaseTestEnv(t)

	// Test NewBlockWriter with pool
	writer1 := NewBlockWriter(env.Pool)
	assert.NotNil(t, writer1)
	assert.NotNil(t, writer1.pool)
	assert.Nil(t, writer1.conn)

	// Test NewBlockWriterFromConn with connection
	ctx := context.Background()
	conn, err := env.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	writer2 := NewBlockWriterFromConn(conn)
	assert.NotNil(t, writer2)
	assert.NotNil(t, writer2.conn)
	assert.Nil(t, writer2.pool)
}

// TestWriteProcessedBlockEmptyComponents tests writing block with empty slices
func TestWriteProcessedBlockEmptyComponents(t *testing.T) {
	env := NewDatabaseTestEnv(t)
	ctx := context.Background()

	parsedData := &types.ParsedBlockData{
		TxNamespaces: []types.TxNamespaceRecord{},
		Writes:       []types.WriteRecord{},
		Reads:        []types.ReadRecord{},
		Endorsements: []types.EndorsementRecord{},
		Policies:     []types.NamespacePolicyRecord{},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       7,
			PreviousHash: []byte("prev7"),
			DataHash:     []byte("data7"),
		},
		Data: parsedData,
		Txns: 0,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	// Verify empty block was written
	env.AssertBlockExists(t, 7)
	assert.Equal(t, int64(0), env.GetTransactionCount(t))
}

// Helper functions

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func strPtr(s string) *string {
	return &s
}
