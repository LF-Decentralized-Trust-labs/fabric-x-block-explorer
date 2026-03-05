/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
	committypes "github.com/hyperledger/fabric-x-committer/api/types"
	"github.com/hyperledger/fabric-x-committer/utils/serialization"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		block            *common.Block
		expectError      bool
		expectedBlockNum uint64
		expectedTxCount  int
	}{
		{
			name:        "nil block header",
			block:       &common.Block{},
			expectError: true,
		},
		{
			name: "missing metadata",
			block: &common.Block{
				Header: &common.BlockHeader{
					Number: 1,
				},
			},
			expectError: true,
		},
		{
			name: "nil block data treats as empty",
			block: &common.Block{
				Header: &common.BlockHeader{
					Number: 3,
				},
				Data: nil,
				Metadata: &common.BlockMetadata{
					Metadata: [][]byte{
						{}, // SIGNATURES
						{}, // LAST_CONFIG
						{}, // TRANSACTIONS_FILTER
					},
				},
			},
			expectError:      false,
			expectedBlockNum: 3,
			expectedTxCount:  0,
		},
		{
			name: "empty block with valid structure",
			block: &common.Block{
				Header: &common.BlockHeader{
					Number:       5,
					PreviousHash: []byte("prevhash"),
					DataHash:     []byte("datahash"),
				},
				Data: &common.BlockData{
					Data: [][]byte{},
				},
				Metadata: &common.BlockMetadata{
					Metadata: [][]byte{
						{}, // SIGNATURES
						{}, // LAST_CONFIG
						{}, // TRANSACTIONS_FILTER
					},
				},
			},
			expectError:      false,
			expectedBlockNum: 5,
			expectedTxCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parsedData, blockInfo, err := Parse(tt.block)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, parsedData)
			assert.NotNil(t, blockInfo)
			assert.Equal(t, tt.expectedBlockNum, blockInfo.Number)
			assert.Len(t, parsedData.Transactions, tt.expectedTxCount)
		})
	}
}

func TestParseBlockWithTransaction(t *testing.T) {
	t.Parallel()
	ns := &protoblocktx.TxNamespace{
		NsId:      "mycc",
		NsVersion: 1,
		ReadWrites: []*protoblocktx.ReadWrite{
			{
				Key:     []byte("key1"),
				Value:   []byte("value1"),
				Version: util.Ptr(uint64(10)),
			},
		},
		ReadsOnly: []*protoblocktx.Read{
			{
				Key:     []byte("key2"),
				Version: util.Ptr(uint64(5)),
			},
		},
	}

	env := createEnvelope(t,
		&common.ChannelHeader{Type: int32(common.HeaderType_ENDORSER_TRANSACTION), TxId: "tx123"},
		marshalTx(t, &protoblocktx.Tx{Namespaces: []*protoblocktx.TxNamespace{ns}}),
	)
	block := buildBlock(t, blockSpec{
		num:       10,
		prevHash:  []byte("prev"),
		dataHash:  []byte("data"),
		envelopes: []*common.Envelope{env},
		statuses:  []protoblocktx.Status{protoblocktx.Status_COMMITTED},
	})

	parsedData, blockInfo, err := Parse(block)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, uint64(10), blockInfo.Number)

	require.Len(t, parsedData.Transactions, 1)
	tx := parsedData.Transactions[0]
	assert.Equal(t, "tx123", tx.TxID)

	require.Len(t, tx.Namespaces, 1)
	nsRec := tx.Namespaces[0]
	assert.Equal(t, "mycc", nsRec.NsID)

	assert.Len(t, nsRec.ReadsOnly, 1)
	assert.Equal(t, []byte("key2"), nsRec.ReadsOnly[0].Key)

	assert.Len(t, nsRec.ReadWrites, 1)
	assert.Equal(t, []byte("key1"), nsRec.ReadWrites[0].Key)
	assert.Equal(t, []byte("value1"), nsRec.ReadWrites[0].Value)
}

func TestExtractPolicies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		envelope       *common.Envelope
		expectPolicies bool
		expectedCount  int
	}{
		{
			name: "non-config transaction",
			envelope: createEnvelope(t, &common.ChannelHeader{
				Type: int32(common.HeaderType_ENDORSER_TRANSACTION),
			}, []byte("data")),
			expectPolicies: false,
		},
		{
			name: "config transaction with namespace policies",
			envelope: createEnvelope(t, &common.ChannelHeader{
				Type: int32(common.HeaderType_CONFIG),
			}, marshalNamespacePolicies(t, &protoblocktx.NamespacePolicies{
				Policies: []*protoblocktx.PolicyItem{
					{
						Namespace: "mycc",
						Version:   1,
						Policy:    []byte("policy_bytes"),
					},
				},
			})),
			expectPolicies: true,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pl, chdr, err := serialization.ParseEnvelope(tt.envelope)
			require.NoError(t, err)
			policies, ok := extractPolicies(pl, chdr)

			if !tt.expectPolicies {
				assert.False(t, ok)
				return
			}

			assert.True(t, ok)
			assert.Len(t, policies, tt.expectedCount)
		})
	}
}

func TestPolicyToJSON(t *testing.T) {
	t.Parallel()
	policyBytes := []byte("test_policy_data")
	jsonData, err := policyToJSON(policyBytes)

	require.NoError(t, err)
	assert.NotNil(t, jsonData)
	assert.Contains(t, string(jsonData), "policy_bytes")
}

func TestEndorsementToIdentityJSON(t *testing.T) {
	t.Parallel()
	// Create a valid SerializedIdentity
	serializedID := &msp.SerializedIdentity{
		Mspid:   "Org1MSP",
		IdBytes: []byte("certificate_data"),
	}
	serializedIDBytes, err := proto.Marshal(serializedID)
	require.NoError(t, err)

	// Create an Endorsement
	endorsement := &peer.Endorsement{
		Endorser:  serializedIDBytes,
		Signature: []byte("signature"),
	}
	endorsementBytes, err := proto.Marshal(endorsement)
	require.NoError(t, err)

	// Test extraction
	mspID, identityJSON, err := endorsementToIdentityJSON(endorsementBytes)

	require.NoError(t, err)
	assert.NotNil(t, mspID)
	assert.Equal(t, "Org1MSP", *mspID)
	assert.NotNil(t, identityJSON)
	assert.Contains(t, string(identityJSON), "Org1MSP")
	assert.Contains(t, string(identityJSON), "id_bytes")
}

func TestEndorsementToIdentityJSONInvalidData(t *testing.T) {
	t.Parallel()
	invalidBytes := []byte("invalid_protobuf")
	_, _, err := endorsementToIdentityJSON(invalidBytes)
	assert.Error(t, err)
}

func TestRWSets(t *testing.T) {
	t.Parallel()
	ns := &protoblocktx.TxNamespace{
		NsId:      "chaincode1",
		NsVersion: 2,
		ReadWrites: []*protoblocktx.ReadWrite{
			{Key: []byte("key1"), Value: []byte("value1"), Version: util.Ptr(uint64(1))},
		},
	}

	tx := &protoblocktx.Tx{
		Namespaces: []*protoblocktx.TxNamespace{ns},
		Signatures: [][]byte{[]byte("sig1")},
	}
	txBytes, err := proto.Marshal(tx)
	require.NoError(t, err)

	chdr := &common.ChannelHeader{
		TxId: "txid123",
	}
	chdrBytes, err := proto.Marshal(chdr)
	require.NoError(t, err)

	payload := &common.Payload{
		Header: &common.Header{
			ChannelHeader: chdrBytes,
		},
		Data: txBytes,
	}

	nsDataList, err := extractNamespaceData("txid123", payload)
	require.NoError(t, err)
	assert.Len(t, nsDataList, 1)
	assert.Equal(t, "chaincode1", nsDataList[0].Namespace.NsId)
	assert.NotNil(t, nsDataList[0].Endorsement)
}

func TestParseWithBlindWrites(t *testing.T) {
	t.Parallel()
	ns := &protoblocktx.TxNamespace{
		NsId:      "mycc",
		NsVersion: 1,
		BlindWrites: []*protoblocktx.Write{
			{
				Key:   []byte("blind_key"),
				Value: []byte("blind_value"),
			},
		},
	}

	env := createEnvelope(t,
		&common.ChannelHeader{Type: int32(common.HeaderType_ENDORSER_TRANSACTION), TxId: "tx_blind"},
		marshalTx(t, &protoblocktx.Tx{Namespaces: []*protoblocktx.TxNamespace{ns}}),
	)
	block := buildBlock(t, blockSpec{
		num:       1,
		envelopes: []*common.Envelope{env},
		statuses:  []protoblocktx.Status{protoblocktx.Status_COMMITTED},
	})

	parsedData, _, err := Parse(block)
	require.NoError(t, err)

	require.Len(t, parsedData.Transactions, 1)
	nsRec := parsedData.Transactions[0].Namespaces[0]
	assert.Len(t, nsRec.BlindWrites, 1)
	assert.Equal(t, []byte("blind_key"), nsRec.BlindWrites[0].Key)
	assert.Equal(t, []byte("blind_value"), nsRec.BlindWrites[0].Value)
}

func TestParseSkipsInvalidTransactions(t *testing.T) {
	t.Parallel()
	block := &common.Block{
		Header: &common.BlockHeader{
			Number: 1,
		},
		Data: &common.BlockData{
			Data: [][]byte{
				[]byte("invalid_envelope_data"),
			},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},
				{},
				{byte(protoblocktx.Status_COMMITTED)},
			},
		},
	}

	parsedData, blockInfo, err := Parse(block)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)

	assert.Empty(t, parsedData.Transactions)
}

func TestParseConfigTransaction(t *testing.T) {
	t.Parallel()
	configTx := &protoblocktx.ConfigTransaction{
		Version:  1,
		Envelope: []byte("config_envelope_data"),
	}
	configBytes, err := proto.Marshal(configTx)
	require.NoError(t, err)

	env := createEnvelope(t,
		&common.ChannelHeader{Type: int32(common.HeaderType_CONFIG)},
		configBytes,
	)
	block := buildBlock(t, blockSpec{
		num:       0,
		envelopes: []*common.Envelope{env},
		statuses:  []protoblocktx.Status{protoblocktx.Status_COMMITTED},
	})

	parsedData, _, err := Parse(block)
	require.NoError(t, err)

	assert.Len(t, parsedData.Policies, 1)
	assert.Equal(t, committypes.MetaNamespaceID, parsedData.Policies[0].Namespace)
}

// TestParseInvalidTransaction verifies that transactions with non-COMMITTED
// validation codes are stored (not skipped), while truly unparseable envelopes
// (MALFORMED_BAD_ENVELOPE) are silently dropped.
func TestParseInvalidTransaction(t *testing.T) {
	t.Parallel()
	// Build a valid envelope containing a MESSAGE tx with a recognisable tx_id.
	validEnv := createEnvelope(t,
		&common.ChannelHeader{
			Type: int32(common.HeaderType_MESSAGE),
			TxId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		},
		[]byte("malformed-tx-data"), // makes UnmarshalTx fail
	)
	validEnvBytes, err := proto.Marshal(validEnv)
	require.NoError(t, err)

	block := &common.Block{
		Header: &common.BlockHeader{Number: 42},
		Data: &common.BlockData{
			Data: [][]byte{
				[]byte("completely_invalid_envelope"), // MALFORMED_BAD_ENVELOPE
				validEnvBytes,                         // REJECTED_MVCC_CONFLICT
			},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},
				{},
				{
					byte(protoblocktx.Status_COMMITTED),             // tx 0 — bad envelope
					byte(protoblocktx.Status_ABORTED_MVCC_CONFLICT), // tx 1 — invalid but parseable
				},
			},
		},
	}

	parsedData, _, err := Parse(block)
	require.NoError(t, err)

	// tx 0 has a bad envelope — no tx_id can be extracted, must be dropped.
	// tx 1 has a valid envelope with no valid rwsets — stored as minimal record.
	require.Len(t, parsedData.Transactions, 1)
	tx := parsedData.Transactions[0]
	assert.Equal(t, uint64(1), tx.TxNum)
	assert.Equal(t, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", tx.TxID)
	assert.Equal(t, protoblocktx.Status_ABORTED_MVCC_CONFLICT, tx.ValidationCode)
	assert.Empty(t, tx.Namespaces)
}

// TestParseInvalidTxNotStoredForCommitted verifies that committed txns with
// unparseable rwsets are NOT stored (they indicate a bug in the pipeline).
func TestParseInvalidTxNotStoredForCommitted(t *testing.T) {
	t.Parallel()
	env := createEnvelope(t,
		&common.ChannelHeader{
			Type: int32(common.HeaderType_MESSAGE),
			TxId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		[]byte("malformed-tx-data"),
	)
	block := buildBlock(t, blockSpec{
		num:       43,
		envelopes: []*common.Envelope{env},
		statuses:  []protoblocktx.Status{protoblocktx.Status_COMMITTED},
	})

	parsedData, _, err := Parse(block)
	require.NoError(t, err)
	assert.Empty(t, parsedData.Transactions, "committed tx with bad rwset must not be stored")
}

// Helper functions

func createEnvelope(t *testing.T, chdr *common.ChannelHeader, data []byte) *common.Envelope {
	t.Helper()
	chdrBytes, err := proto.Marshal(chdr)
	require.NoError(t, err)

	payload := &common.Payload{
		Header: &common.Header{
			ChannelHeader: chdrBytes,
		},
		Data: data,
	}
	payloadBytes, err := proto.Marshal(payload)
	require.NoError(t, err)

	return &common.Envelope{
		Payload: payloadBytes,
	}
}

// blockSpec is the input to buildBlock.
type blockSpec struct {
	num       uint64
	prevHash  []byte
	dataHash  []byte
	envelopes []*common.Envelope
	statuses  []protoblocktx.Status
}

// buildBlock constructs a common.Block from a blockSpec.
// prevHash and dataHash may be nil for tests that do not check header fields.
func buildBlock(t *testing.T, spec blockSpec) *common.Block {
	t.Helper()

	data := make([][]byte, len(spec.envelopes))
	for i, env := range spec.envelopes {
		b, err := proto.Marshal(env)
		require.NoError(t, err)
		data[i] = b
	}

	filter := make([]byte, len(spec.statuses))
	for i, s := range spec.statuses {
		filter[i] = byte(s)
	}

	return &common.Block{
		Header: &common.BlockHeader{
			Number:       spec.num,
			PreviousHash: spec.prevHash,
			DataHash:     spec.dataHash,
		},
		Data: &common.BlockData{Data: data},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},     // SIGNATURES
				{},     // LAST_CONFIG
				filter, // TRANSACTIONS_FILTER
			},
		},
	}
}

// marshalTx marshals a protoblocktx.Tx and returns the raw bytes.
func marshalTx(t *testing.T, tx *protoblocktx.Tx) []byte {
	t.Helper()
	b, err := proto.Marshal(tx)
	require.NoError(t, err)
	return b
}

func marshalNamespacePolicies(t *testing.T, np *protoblocktx.NamespacePolicies) []byte {
	t.Helper()
	data, err := proto.Marshal(np)
	require.NoError(t, err)
	return data
}
