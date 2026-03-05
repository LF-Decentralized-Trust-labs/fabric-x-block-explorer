/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"encoding/base64"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
	committypes "github.com/hyperledger/fabric-x-committer/api/types"
	"github.com/hyperledger/fabric-x-committer/service/sidecar"
	"github.com/hyperledger/fabric-x-committer/utils/logging"
	"github.com/hyperledger/fabric-x-committer/utils/serialization"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = logging.New("parser")

// txMeta carries per-transaction context through the parsing chain.
type txMeta struct {
	blockNum       uint64
	txNum          int
	txID           string
	validationCode protoblocktx.Status
}

// nsData wraps a TxNamespace with its associated endorsement bytes.
type nsData struct {
	Namespace   *protoblocktx.TxNamespace
	Endorsement []byte
}

// Parse extracts transactions and write-sets from a Fabric block into ParsedBlockData and BlockInfo.
func Parse(b *common.Block) (*types.ParsedBlockData, *types.BlockInfo, error) {
	header := b.GetHeader()
	if header == nil {
		return nil, nil, errors.New("block header missing")
	}

	blockInfo := &types.BlockInfo{
		Number:       header.Number,
		PreviousHash: header.PreviousHash,
		DataHash:     header.DataHash,
	}

	if b.Metadata == nil || len(b.Metadata.Metadata) <= int(common.BlockMetadataIndex_TRANSACTIONS_FILTER) {
		return nil, blockInfo, errors.New("block metadata missing TRANSACTIONS_FILTER")
	}

	txFilter := b.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER]
	var rawData [][]byte
	if b.Data != nil {
		rawData = b.Data.Data
	}
	transactions, policies := parseTxData(header, rawData, txFilter)

	return &types.ParsedBlockData{
		Transactions: transactions,
		Policies:     policies,
	}, blockInfo, nil
}

func parseTxData(
	header *common.BlockHeader,
	data [][]byte,
	txFilter []byte,
) ([]types.TxRecord, []types.NamespacePolicyRecord) {
	transactions := make([]types.TxRecord, 0, len(data))
	policies := make([]types.NamespacePolicyRecord, 0)

	for txNum, envBytes := range data {
		if txNum >= len(txFilter) {
			continue
		}
		validationCode := protoblocktx.Status(txFilter[txNum])
		txRec, policyItems := handleTx(header, txNum, envBytes, validationCode)
		policies = append(policies, policyItems...)
		if txRec != nil {
			transactions = append(transactions, *txRec)
		}
	}

	return transactions, policies
}

func handleTx(
	header *common.BlockHeader,
	txNum int,
	envBytes []byte,
	validationCode protoblocktx.Status,
) (*types.TxRecord, []types.NamespacePolicyRecord) {
	env := &common.Envelope{}
	if err := proto.Unmarshal(envBytes, env); err != nil {
		logger.Warnf("block %d tx %d invalid envelope: %v", header.Number, txNum, err)
		return nil, nil
	}

	// Parsed once; pl and chdr are reused by all call sites below.
	pl, chdr, err := serialization.ParseEnvelope(env)
	if err != nil {
		logger.Warnf("block %d tx %d unparseable payload: %v", header.Number, txNum, err)
		return nil, nil
	}

	// Config txs go to namespace_policies and have no tx_id; check before validating tx_id.
	if policyItems, ok := extractCommittedPolicies(validationCode, pl, chdr); ok {
		return nil, policyItems
	}

	txID := chdr.TxId
	if txID == "" {
		logger.Warnf("block %d tx %d: missing or invalid tx_id", header.Number, txNum)
		return nil, nil
	}

	if !sidecar.IsStatusStoredInDB(validationCode) {
		logger.Warnf("block %d tx %d: status %s not stored in DB", header.Number, txNum, &validationCode)
		return nil, nil
	}

	meta := txMeta{
		blockNum:       header.Number,
		txNum:          txNum,
		txID:           txID,
		validationCode: validationCode,
	}
	return parseTxRecord(meta, pl), nil
}

// parseTxRecord builds a TxRecord: full when rwsets parse, minimal (no namespaces)
// for invalid txs with bad rwsets, nil for committed txs with bad rwsets.
func parseTxRecord(meta txMeta, pl *common.Payload) *types.TxRecord {
	nsList, err := extractNamespaceData(meta.txID, pl)
	if err != nil {
		logger.Warnf("block %d tx %d invalid rwset: %v", meta.blockNum, meta.txNum, err)
	}

	// Non-COMMITTED txs are always stored, even without parseable rwsets.
	if err != nil || len(nsList) == 0 {
		if meta.validationCode != protoblocktx.Status_COMMITTED {
			rec := buildMinimalTxRecord(meta)
			return &rec
		}
		return nil
	}

	rec := buildTxRecord(meta, nsList)
	return &rec
}

// buildMinimalTxRecord returns a TxRecord with no namespace data.
func buildMinimalTxRecord(meta txMeta) types.TxRecord {
	return types.TxRecord{
		TxNum:          uint64(meta.txNum), //nolint:gosec // txNum is a range index, always non-negative
		TxID:           meta.txID,
		ValidationCode: meta.validationCode,
	}
}

func buildTxRecord(meta txMeta, nsList []nsData) types.TxRecord {
	txRecord := types.TxRecord{
		TxNum:          uint64(meta.txNum), //nolint:gosec // txNum is a range index, always non-negative
		TxID:           meta.txID,
		ValidationCode: meta.validationCode,
		Namespaces:     make([]types.TxNamespaceRecord, 0, len(nsList)),
	}

	for _, nd := range nsList {
		txRecord.Namespaces = append(txRecord.Namespaces, buildTxNamespaceRecord(nd))
	}

	return txRecord
}

func buildTxNamespaceRecord(nd nsData) types.TxNamespaceRecord {
	ns := nd.Namespace
	nsRecord := types.TxNamespaceRecord{
		NsID:        ns.NsId,
		NsVersion:   ns.NsVersion,
		ReadsOnly:   make([]types.ReadOnlyRecord, 0, len(ns.ReadsOnly)),
		ReadWrites:  make([]types.ReadWriteRecord, 0, len(ns.ReadWrites)),
		BlindWrites: make([]types.BlindWriteRecord, 0, len(ns.BlindWrites)),
	}

	if len(nd.Endorsement) > 0 {
		rec := types.EndorsementRecord{Endorsement: nd.Endorsement}
		if mspID, identityJSON, err := endorsementToIdentityJSON(nd.Endorsement); err == nil {
			rec.MspID = mspID
			rec.Identity = identityJSON
		} else {
			logger.Warnf("failed to parse endorsement identity: %v", err)
		}
		nsRecord.Endorsements = []types.EndorsementRecord{rec}
	}

	for _, ro := range ns.ReadsOnly {
		roRecord := types.ReadOnlyRecord{Key: ro.Key}
		if ro.Version != nil && *ro.Version > 0 {
			roRecord.Version = ro.Version
		}
		nsRecord.ReadsOnly = append(nsRecord.ReadsOnly, roRecord)
	}

	for _, rw := range ns.ReadWrites {
		rwRecord := types.ReadWriteRecord{
			Key:   rw.Key,
			Value: rw.Value,
		}
		if rw.Version != nil && *rw.Version > 0 {
			rwRecord.ReadVersion = rw.Version
		}
		nsRecord.ReadWrites = append(nsRecord.ReadWrites, rwRecord)
	}

	for _, bw := range ns.BlindWrites {
		nsRecord.BlindWrites = append(nsRecord.BlindWrites, types.BlindWriteRecord{
			Key:   bw.Key,
			Value: bw.Value,
		})
	}

	return nsRecord
}

// policyEncoding is the JSON shape for a serialised policy value.
type policyEncoding struct {
	PolicyBytes string `json:"policy_bytes"`
}

// identityEncoding is the JSON shape for a parsed endorser identity.
type identityEncoding struct {
	MspID   string `json:"mspid"`
	IDBytes string `json:"id_bytes"`
}

// policyToJSON encodes raw policy bytes as a JSON object with a base64 "policy_bytes" field.
func policyToJSON(policyBytes []byte) (json.RawMessage, error) {
	return json.Marshal(policyEncoding{
		PolicyBytes: base64.StdEncoding.EncodeToString(policyBytes),
	})
}

// endorsementToIdentityJSON extracts the MSP ID and identity JSON from a serialised endorsement.
func endorsementToIdentityJSON(endorsementBytes []byte) (*string, json.RawMessage, error) {
	endorsement := &peer.Endorsement{}
	if err := proto.Unmarshal(endorsementBytes, endorsement); err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal endorsement")
	}

	serializedID := &msp.SerializedIdentity{}
	if err := proto.Unmarshal(endorsement.Endorser, serializedID); err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal endorser")
	}

	mspID := serializedID.Mspid

	identityJSON, err := json.Marshal(identityEncoding{
		MspID:   mspID,
		IDBytes: base64.StdEncoding.EncodeToString(serializedID.IdBytes),
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal identity")
	}

	return &mspID, identityJSON, nil
}

// extractCommittedPolicies returns policy records for COMMITTED config transactions; nil, false otherwise.
func extractCommittedPolicies(
	code protoblocktx.Status, pl *common.Payload, chdr *common.ChannelHeader,
) ([]types.NamespacePolicyRecord, bool) {
	if code != protoblocktx.Status_COMMITTED {
		return nil, false
	}
	return extractPolicies(pl, chdr)
}

func extractPolicies(pl *common.Payload, chdr *common.ChannelHeader) ([]types.NamespacePolicyRecord, bool) {
	if chdr.Type != int32(common.HeaderType_CONFIG) && chdr.Type != int32(common.HeaderType_CONFIG_UPDATE) {
		return nil, false
	}

	if items, ok := extractNamespacePolicies(pl.Data); ok {
		return items, true
	}

	return extractConfigTxPolicy(pl.Data)
}

func extractNamespacePolicies(data []byte) ([]types.NamespacePolicyRecord, bool) {
	policies := &protoblocktx.NamespacePolicies{}
	if err := proto.Unmarshal(data, policies); err != nil {
		logger.Debugf("data is not NamespacePolicies proto: %v", err)
		return nil, false
	}
	if len(policies.Policies) == 0 {
		return nil, false
	}

	items := make([]types.NamespacePolicyRecord, 0, len(policies.Policies))
	for _, pd := range policies.Policies {
		if len(pd.Policy) == 0 {
			continue
		}
		ns := pd.Namespace
		if ns == "" {
			ns = committypes.MetaNamespaceID
		}
		policyJSON, err := policyToJSON(pd.Policy)
		if err != nil {
			logger.Warnf("failed to convert policy to JSON for namespace %s: %v", ns, err)
			continue
		}
		items = append(items, types.NamespacePolicyRecord{
			Namespace:  ns,
			Version:    pd.Version,
			PolicyJSON: policyJSON,
		})
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

func extractConfigTxPolicy(data []byte) ([]types.NamespacePolicyRecord, bool) {
	configTx := &protoblocktx.ConfigTransaction{}
	if err := proto.Unmarshal(data, configTx); err != nil {
		logger.Debugf("data is not ConfigTransaction proto: %v", err)
		return nil, false
	}
	if len(configTx.Envelope) == 0 {
		return nil, false
	}

	policyJSON, err := policyToJSON(configTx.Envelope)
	if err != nil {
		logger.Warnf("failed to convert config envelope to JSON: %v", err)
		return nil, false
	}

	return []types.NamespacePolicyRecord{
		{
			Namespace:  committypes.MetaNamespaceID,
			Version:    configTx.Version,
			PolicyJSON: policyJSON,
		},
	}, true
}

// extractNamespaceData unmarshals the tx payload and returns one nsData per namespace with its endorsement.
func extractNamespaceData(txID string, pl *common.Payload) ([]nsData, error) {
	tx, err := serialization.UnmarshalTx(pl.Data)
	if err != nil {
		return nil, errors.Wrap(err, "transaction")
	}

	out := make([]nsData, 0, len(tx.Namespaces))

	if len(tx.Signatures) > 0 && len(tx.Signatures) != len(tx.Namespaces) {
		logger.Warnf(
			"tx %s signature count %d does not match namespaces %d",
			txID, len(tx.Signatures), len(tx.Namespaces),
		)
	}

	for i, ns := range tx.Namespaces {
		var endorsement []byte
		if i < len(tx.Signatures) {
			endorsement = tx.Signatures[i]
		}

		out = append(out, nsData{
			Namespace:   ns,
			Endorsement: endorsement,
		})
	}

	return out, nil
}
