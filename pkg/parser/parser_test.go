/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
	committypes "github.com/hyperledger/fabric-x-committer/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestParse(t *testing.T) {
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
	ns := &protoblocktx.TxNamespace{
		NsId:      "mycc",
		NsVersion: 1,
		ReadWrites: []*protoblocktx.ReadWrite{
			{
				Key:     []byte("key1"),
				Value:   []byte("value1"),
				Version: uint64Ptr(10),
			},
		},
		ReadsOnly: []*protoblocktx.Read{
			{
				Key:     []byte("key2"),
				Version: uint64Ptr(5),
			},
		},
	}

	protoTx := &protoblocktx.Tx{
		Namespaces: []*protoblocktx.TxNamespace{ns},
	}
	txBytes, err := proto.Marshal(protoTx)
	require.NoError(t, err)

	chdr := &common.ChannelHeader{
		Type:  int32(common.HeaderType_ENDORSER_TRANSACTION),
		TxId:  "tx123",
		Epoch: 0,
	}
	chdrBytes, err := proto.Marshal(chdr)
	require.NoError(t, err)

	payload := &common.Payload{
		Header: &common.Header{
			ChannelHeader: chdrBytes,
		},
		Data: txBytes,
	}
	payloadBytes, err := proto.Marshal(payload)
	require.NoError(t, err)

	env := &common.Envelope{
		Payload: payloadBytes,
	}
	envBytes, err := proto.Marshal(env)
	require.NoError(t, err)

	block := &common.Block{
		Header: &common.BlockHeader{
			Number:       10,
			PreviousHash: []byte("prev"),
			DataHash:     []byte("data"),
		},
		Data: &common.BlockData{
			Data: [][]byte{envBytes},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},                                    // SIGNATURES
				{},                                    // LAST_CONFIG
				{byte(protoblocktx.Status_COMMITTED)}, // TRANSACTIONS_FILTER
			},
		},
	}

	parsedData, blockInfo, err := Parse(block)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, uint64(10), blockInfo.Number)

	require.Len(t, parsedData.Transactions, 1)
	tx := parsedData.Transactions[0]
	assert.Equal(t, "tx123", tx.TxID)
	assert.Equal(t, uint64(10), tx.BlockNum)

	require.Len(t, tx.Namespaces, 1)
	nsRec := tx.Namespaces[0]
	assert.Equal(t, "mycc", nsRec.NsID)

	assert.Len(t, nsRec.ReadsOnly, 1)
	assert.Equal(t, "key2", nsRec.ReadsOnly[0].Key)

	assert.Len(t, nsRec.ReadWrites, 1)
	assert.Equal(t, "key1", nsRec.ReadWrites[0].Key)
	assert.Equal(t, []byte("value1"), nsRec.ReadWrites[0].Value)
}

func TestExtractPolicies(t *testing.T) {
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
			policies, ok := extractPolicies(tt.envelope)

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
	policyBytes := []byte("test_policy_data")
	jsonData, err := policyToJSON(policyBytes)

	require.NoError(t, err)
	assert.NotNil(t, jsonData)
	assert.Contains(t, string(jsonData), "policy_bytes")
}

func TestEndorsementToIdentityJSON(t *testing.T) {
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
	invalidBytes := []byte("invalid_protobuf")
	_, _, err := endorsementToIdentityJSON(invalidBytes)
	assert.Error(t, err)
}

func TestRWSets(t *testing.T) {
	ns := &protoblocktx.TxNamespace{
		NsId:      "chaincode1",
		NsVersion: 2,
		ReadWrites: []*protoblocktx.ReadWrite{
			{Key: []byte("key1"), Value: []byte("value1"), Version: uint64Ptr(1)},
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
	payloadBytes, err := proto.Marshal(payload)
	require.NoError(t, err)

	env := &common.Envelope{
		Payload: payloadBytes,
	}

	nsDataList, err := rwSets(env)
	require.NoError(t, err)
	assert.Len(t, nsDataList, 1)
	assert.Equal(t, "txid123", nsDataList[0].TxID)
	assert.Equal(t, "chaincode1", nsDataList[0].Namespace.NsId)
	assert.NotNil(t, nsDataList[0].Endorsement)
}

func TestParseWithBlindWrites(t *testing.T) {
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

	tx := &protoblocktx.Tx{
		Namespaces: []*protoblocktx.TxNamespace{ns},
	}
	txBytes, _ := proto.Marshal(tx)

	chdr := &common.ChannelHeader{
		Type: int32(common.HeaderType_ENDORSER_TRANSACTION),
		TxId: "tx_blind",
	}
	chdrBytes, _ := proto.Marshal(chdr)

	payload := &common.Payload{
		Header: &common.Header{
			ChannelHeader: chdrBytes,
		},
		Data: txBytes,
	}
	payloadBytes, _ := proto.Marshal(payload)

	env := &common.Envelope{
		Payload: payloadBytes,
	}
	envBytes, _ := proto.Marshal(env)

	block := &common.Block{
		Header: &common.BlockHeader{
			Number: 1,
		},
		Data: &common.BlockData{
			Data: [][]byte{envBytes},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},
				{},
				{byte(protoblocktx.Status_COMMITTED)},
			},
		},
	}

	parsedData, _, err := Parse(block)
	require.NoError(t, err)

	require.Len(t, parsedData.Transactions, 1)
	nsRec := parsedData.Transactions[0].Namespaces[0]
	assert.Len(t, nsRec.BlindWrites, 1)
	assert.Equal(t, "blind_key", nsRec.BlindWrites[0].Key)
	assert.Equal(t, []byte("blind_value"), nsRec.BlindWrites[0].Value)
}

func TestParseSkipsInvalidTransactions(t *testing.T) {
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
	configTx := &protoblocktx.ConfigTransaction{
		Version:  1,
		Envelope: []byte("config_envelope_data"),
	}
	configBytes, _ := proto.Marshal(configTx)

	chdr := &common.ChannelHeader{
		Type: int32(common.HeaderType_CONFIG),
	}
	chdrBytes, _ := proto.Marshal(chdr)

	payload := &common.Payload{
		Header: &common.Header{
			ChannelHeader: chdrBytes,
		},
		Data: configBytes,
	}
	payloadBytes, _ := proto.Marshal(payload)

	env := &common.Envelope{
		Payload: payloadBytes,
	}
	envBytes, _ := proto.Marshal(env)

	block := &common.Block{
		Header: &common.BlockHeader{
			Number: 0,
		},
		Data: &common.BlockData{
			Data: [][]byte{envBytes},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{
				{},
				{},
				{byte(protoblocktx.Status_COMMITTED)},
			},
		},
	}

	parsedData, _, err := Parse(block)
	require.NoError(t, err)

	assert.Len(t, parsedData.Policies, 1)
	assert.Equal(t, committypes.MetaNamespaceID, parsedData.Policies[0].Namespace)
}

// Helper functions

func uint64Ptr(v uint64) *uint64 {
	return &v
}

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

func marshalNamespacePolicies(t *testing.T, np *protoblocktx.NamespacePolicies) []byte {
	t.Helper()
	data, err := proto.Marshal(np)
	require.NoError(t, err)
	return data
}
