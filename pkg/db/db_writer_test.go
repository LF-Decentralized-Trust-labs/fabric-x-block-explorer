/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

func TestWriteProcessedBlock(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	txID := "abc123def456"
	txIDBytes, err := hex.DecodeString(txID)
	require.NoError(t, err)

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{
			{
				BlockNum:       1,
				TxNum:          0,
				TxID:           txID,
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "mycc",
						NsVersion: 1,
						ReadWrites: []types.ReadWriteRecord{
							{Key: "key1", ReadVersion: uint64Ptr(10), Value: []byte("value1")},
						},
						Endorsements: []types.EndorsementRecord{
							{
								Endorsement: []byte("endorsement_sig"),
								MspID:       strPtr("Org1MSP"),
								Identity:    []byte(`{"mspid":"Org1MSP","id_bytes":"cert"}`),
							},
						},
					},
				},
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

	writer := NewBlockWriter(env.Pool)
	err = writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	block, err := env.Queries.GetBlock(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), block.BlockNum)
	assert.Equal(t, int32(1), block.TxCount)
	assert.Equal(t, []byte("prevhash"), block.PreviousHash)
	assert.Equal(t, []byte("datahash"), block.DataHash)

	tx, err := env.Queries.GetValidationCodeByTxID(ctx, txIDBytes)
	require.NoError(t, err)
	assert.Equal(t, int64(1), tx.BlockNum)
	assert.Equal(t, int64(0), tx.TxNum)

	assert.Equal(t, int64(1), env.GetBlockCount(t))
	assert.Equal(t, int64(1), env.GetTransactionCount(t))
}

func TestWriteProcessedBlockWithBlindWrites(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	txID := "deadbeef"
	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{
			{
				BlockNum:       2,
				TxNum:          0,
				TxID:           txID,
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "testcc",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: "blindkey", Value: []byte("blindvalue")},
						},
					},
				},
			},
		},
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

	env.AssertBlockExists(t, 2)

	var key []byte
	err = env.Pool.QueryRow(ctx, `
		SELECT key
		FROM tx_blind_writes
		WHERE ns_id = $1 AND key = $2
	`, "testcc", []byte("blindkey")).Scan(&key)
	require.NoError(t, err)
	assert.Equal(t, []byte("blindkey"), key)
}

func TestWriteProcessedBlockMultipleTransactions(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{
			{
				BlockNum:       3,
				TxNum:          0,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000001",
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "cc1",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: "key1", Value: []byte("val1")},
						},
					},
				},
			},
			{
				BlockNum:       3,
				TxNum:          1,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000002",
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "cc2",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: "key2", Value: []byte("val2")},
						},
					},
				},
			},
		},
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

	assert.Equal(t, int64(2), env.GetTransactionCount(t))

	var count int64
	err = env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tx_namespaces").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestWriteProcessedBlockNilBlock(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestWriteProcessedBlockInvalidData(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number: 1,
		},
		Data: "invalid_data_type",
		Txns: 0,
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not *types.ParsedBlockData")
}

func TestWriteProcessedBlockWithPolicies(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	policyJSON := json.RawMessage(`{"policy_bytes":"base64encodedpolicy"}`)

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{},
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

	policies, err := env.Queries.GetNamespacePolicies(ctx, "mycc")
	require.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "mycc", policies[0].Namespace)
	assert.Equal(t, int64(1), policies[0].Version)

	parsedData2 := &types.ParsedBlockData{
		Transactions: []types.TxRecord{},
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

	policies, err = env.Queries.GetNamespacePolicies(ctx, "mycc")
	require.NoError(t, err)
	assert.Len(t, policies, 2)
}

func TestWriteProcessedBlockRollbackOnError(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{
			{
				BlockNum:       6,
				TxNum:          0,
				TxID:           "invalid_hex_ZZZ",
				ValidationCode: 0,
				Namespaces:     []types.TxNamespaceRecord{},
			},
		},
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode tx_id")

	env.AssertBlockNotExists(t, 6)
}

func TestNewBlockWriter(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)

	writer1 := NewBlockWriter(env.Pool)
	assert.NotNil(t, writer1)
	assert.NotNil(t, writer1.pool)
	assert.Nil(t, writer1.conn)

	ctx := t.Context()
	conn, err := env.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	writer2 := NewBlockWriterFromConn(conn)
	assert.NotNil(t, writer2)
	assert.NotNil(t, writer2.conn)
	assert.Nil(t, writer2.pool)
}

func TestWriteProcessedBlockEmptyComponents(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{},
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
