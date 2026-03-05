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

	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
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
				TxNum:          0,
				TxID:           txID,
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "mycc",
						NsVersion: 1,
						ReadWrites: []types.ReadWriteRecord{
							{Key: []byte("key1"), ReadVersion: util.Ptr(uint64(10)), Value: []byte("value1")},
						},
						Endorsements: []types.EndorsementRecord{
							{
								Endorsement: []byte("endorsement_sig"),
								MspID:       util.Ptr("Org1MSP"),
								Identity:    json.RawMessage(`{"mspid":"Org1MSP","id_bytes":"cert"}`),
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
				TxNum:          0,
				TxID:           txID,
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "testcc",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: []byte("blindkey"), Value: []byte("blindvalue")},
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
				TxNum:          0,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000001",
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "cc1",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: []byte("key1"), Value: []byte("val1")},
						},
					},
				},
			},
			{
				TxNum:          1,
				TxID:           "0000000000000000000000000000000000000000000000000000000000000002",
				ValidationCode: 0,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "cc2",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: []byte("key2"), Value: []byte("val2")},
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

func TestWriteProcessedBlockNilBlockInfo(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, &types.ProcessedBlock{
		BlockInfo: nil,
		Data:      &types.ParsedBlockData{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestWriteProcessedBlockNilData(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{Number: 99},
		Data:      nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
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
	}

	writer := NewBlockWriter(env.Pool)
	err := writer.WriteProcessedBlock(ctx, processedBlock)
	require.NoError(t, err)

	env.AssertBlockExists(t, 7)
	assert.Equal(t, int64(0), env.GetTransactionCount(t))
}

// TestWriteInvalidTransaction verifies that transactions with non-COMMITTED
// validation codes are stored with the correct status and without namespace data.
func TestWriteInvalidTransaction(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	committedTxID := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	invalidTxID := "0000000000000000000000000000000000000000000000000000000000000099"

	parsedData := &types.ParsedBlockData{
		Transactions: []types.TxRecord{
			{
				TxNum:          0,
				TxID:           committedTxID,
				ValidationCode: protoblocktx.Status_COMMITTED,
				Namespaces: []types.TxNamespaceRecord{
					{
						NsID:      "cc",
						NsVersion: 1,
						BlindWrites: []types.BlindWriteRecord{
							{Key: []byte("k"), Value: []byte("v")},
						},
					},
				},
			},
			{
				// Minimal record: no namespaces, as produced by buildMinimalTxRecord.
				TxNum:          1,
				TxID:           invalidTxID,
				ValidationCode: protoblocktx.Status_ABORTED_MVCC_CONFLICT,
			},
		},
	}

	processedBlock := &types.ProcessedBlock{
		BlockInfo: &types.BlockInfo{
			Number:       10,
			PreviousHash: []byte("prev10"),
			DataHash:     []byte("data10"),
		},
		Data: parsedData,
	}

	writer := NewBlockWriter(env.Pool)
	require.NoError(t, writer.WriteProcessedBlock(ctx, processedBlock))

	// Both the committed and the invalid transaction must be persisted.
	assert.Equal(t, int64(2), env.GetTransactionCount(t))

	// The invalid transaction must carry the correct validation code.
	invalidTxIDBytes, err := hex.DecodeString(invalidTxID)
	require.NoError(t, err)
	tx, err := env.Queries.GetValidationCodeByTxID(ctx, invalidTxIDBytes)
	require.NoError(t, err)
	assert.Equal(t, int16(protoblocktx.Status_ABORTED_MVCC_CONFLICT), tx.ValidationCode)
	assert.Equal(t, int64(1), tx.TxNum)

	// The invalid transaction must have no namespace rows.
	var nsCount int64
	q := `SELECT COUNT(*) FROM tx_namespaces WHERE block_num = $1 AND tx_num = 1`
	err = env.Pool.QueryRow(ctx, q, int64(10)).Scan(&nsCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), nsCount)
}
