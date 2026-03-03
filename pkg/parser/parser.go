/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"encoding/base64"
	"encoding/json"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
	committypes "github.com/hyperledger/fabric-x-committer/api/types"
	"github.com/hyperledger/fabric/common/flogging"
	"google.golang.org/protobuf/proto"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = flogging.MustGetLogger("parser")

// nsData wraps a TxNamespace with transaction metadata.
type nsData struct {
	Namespace   *protoblocktx.TxNamespace
	TxID        string
	Endorsement []byte
}

// Parser parses a raw Fabric block into structured data.
// Implement this interface to substitute a custom parser in the pipeline.
type Parser interface {
	Parse(*common.Block) (*types.ParsedBlockData, *types.BlockInfo, error)
}

// BlockParser is the default Parser implementation.
type BlockParser struct{}

// New returns a new BlockParser.
func New() *BlockParser {
	return &BlockParser{}
}

// Parse implements the Parser interface.
func (*BlockParser) Parse(b *common.Block) (*types.ParsedBlockData, *types.BlockInfo, error) {
	return Parse(b)
}

// Parse extracts transactions and write-sets from a Fabric block and returns
// the data organised in the hierarchical ParsedBlockData structure alongside
// a BlockInfo containing the block header metadata.
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
	transactions, policies := parseTxData(header, b.Data.Data, txFilter)

	return &types.ParsedBlockData{
		Transactions: transactions,
		Policies:     policies,
	}, blockInfo, nil
}

// parseTxData iterates over the raw envelope data and builds transaction and policy records.
func parseTxData(
	header *common.BlockHeader,
	data [][]byte,
	txFilter []byte,
) ([]types.TxRecord, []types.NamespacePolicyRecord) {
	transactions := []types.TxRecord{}
	policies := []types.NamespacePolicyRecord{}

	for txNum, envBytes := range data {
		if txNum >= len(txFilter) {
			continue
		}

		validationCode := protoblocktx.Status(txFilter[txNum])
		if validationCode != protoblocktx.Status_COMMITTED {
			continue
		}

		env := &common.Envelope{}
		if err := proto.Unmarshal(envBytes, env); err != nil {
			logger.Warnf("block %d tx %d invalid envelope: %v", header.Number, txNum, err)
			continue
		}

		if policyItems, ok := extractPolicies(env); ok {
			policies = append(policies, policyItems...)
			continue
		}

		nsList, err := rwSets(env)
		if err != nil {
			logger.Warnf("block %d tx %d invalid rwset: %v", header.Number, txNum, err)
			continue
		}

		if len(nsList) == 0 {
			continue
		}

		transactions = append(transactions, buildTxRecord(header, txNum, validationCode, nsList))
	}

	return transactions, policies
}

// buildTxRecord constructs a TxRecord from a parsed namespace list.
func buildTxRecord(
	header *common.BlockHeader,
	txNum int,
	validationCode protoblocktx.Status,
	nsList []nsData,
) types.TxRecord {
	txRecord := types.TxRecord{
		BlockNum:       header.Number,
		TxNum:          uint64(txNum), //nolint:gosec // txNum originates from a range index and is always non-negative
		TxID:           nsList[0].TxID,
		ValidationCode: int32(validationCode),
		Namespaces:     make([]types.TxNamespaceRecord, 0, len(nsList)),
	}

	for _, nd := range nsList {
		txRecord.Namespaces = append(txRecord.Namespaces, buildTxNamespaceRecord(nd))
	}

	return txRecord
}

// buildTxNamespaceRecord constructs a TxNamespaceRecord from a single nsData entry.
func buildTxNamespaceRecord(nd nsData) types.TxNamespaceRecord {
	ns := nd.Namespace
	nsRecord := types.TxNamespaceRecord{
		NsID:      ns.NsId,
		NsVersion: ns.NsVersion,
	}

	if len(nd.Endorsement) > 0 {
		mspID, identityJSON, err := endorsementToIdentityJSON(nd.Endorsement)
		if err != nil {
			nsRecord.Endorsements = append(nsRecord.Endorsements, types.EndorsementRecord{
				Endorsement: nd.Endorsement,
			})
		} else {
			nsRecord.Endorsements = append(nsRecord.Endorsements, types.EndorsementRecord{
				Endorsement: nd.Endorsement,
				MspID:       mspID,
				Identity:    identityJSON,
			})
		}
	}

	for _, ro := range ns.ReadsOnly {
		roRecord := types.ReadOnlyRecord{Key: string(ro.Key)}
		if ro.Version != nil && *ro.Version > 0 {
			roRecord.Version = ro.Version
		}
		nsRecord.ReadsOnly = append(nsRecord.ReadsOnly, roRecord)
	}

	for _, rw := range ns.ReadWrites {
		rwRecord := types.ReadWriteRecord{
			Key:   string(rw.Key),
			Value: rw.Value,
		}
		if rw.Version != nil && *rw.Version > 0 {
			rwRecord.ReadVersion = rw.Version
		}
		nsRecord.ReadWrites = append(nsRecord.ReadWrites, rwRecord)
	}

	for _, bw := range ns.BlindWrites {
		nsRecord.BlindWrites = append(nsRecord.BlindWrites, types.BlindWriteRecord{
			Key:   string(bw.Key),
			Value: bw.Value,
		})
	}

	return nsRecord
}

// policyToJSON converts protobuf policy bytes to a JSON object with base64-encoded policy.
func policyToJSON(policyBytes []byte) (json.RawMessage, error) {
	// Store as base64-encoded bytes in a simple JSON structure.
	// This allows storing in JSONB while preserving exact binary data.
	return json.Marshal(map[string]string{
		"policy_bytes": base64.StdEncoding.EncodeToString(policyBytes),
	})
}

// endorsementToIdentityJSON extracts identity information from endorsement protobuf.
func endorsementToIdentityJSON(endorsementBytes []byte) (*string, []byte, error) {
	endorsement := &peer.Endorsement{}
	if err := proto.Unmarshal(endorsementBytes, endorsement); err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal endorsement")
	}

	serializedID := &msp.SerializedIdentity{}
	if err := proto.Unmarshal(endorsement.Endorser, serializedID); err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal endorser")
	}

	mspID := serializedID.Mspid

	identityData := map[string]any{
		"mspid":    serializedID.Mspid,
		"id_bytes": base64.StdEncoding.EncodeToString(serializedID.IdBytes),
	}

	identityJSON, err := json.Marshal(identityData)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal identity")
	}

	return &mspID, identityJSON, nil
}

// extractPolicies parses namespace policy updates from the envelope.
func extractPolicies(env *common.Envelope) ([]types.NamespacePolicyRecord, bool) {
	pl := &common.Payload{}
	if err := proto.Unmarshal(env.Payload, pl); err != nil {
		return nil, false
	}

	chdr := &common.ChannelHeader{}
	if pl.Header == nil || pl.Header.ChannelHeader == nil {
		return nil, false
	}
	if err := proto.Unmarshal(pl.Header.ChannelHeader, chdr); err != nil {
		return nil, false
	}
	if chdr.Type != int32(common.HeaderType_CONFIG) && chdr.Type != int32(common.HeaderType_CONFIG_UPDATE) {
		return nil, false
	}

	if items, ok := extractNamespacePolicies(pl.Data); ok {
		return items, true
	}

	return extractConfigTxPolicy(pl.Data)
}

// extractNamespacePolicies parses a NamespacePolicies proto from data and returns policy records.
func extractNamespacePolicies(data []byte) ([]types.NamespacePolicyRecord, bool) {
	policies := &protoblocktx.NamespacePolicies{}
	if err := proto.Unmarshal(data, policies); err != nil || len(policies.Policies) == 0 {
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

// extractConfigTxPolicy parses a ConfigTransaction proto from data and returns a policy record.
func extractConfigTxPolicy(data []byte) ([]types.NamespacePolicyRecord, bool) {
	configTx := &protoblocktx.ConfigTransaction{}
	if err := proto.Unmarshal(data, configTx); err != nil || len(configTx.Envelope) == 0 {
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

// rwSets extracts namespace data and txID from an envelope.
// Returns the proto TxNamespace data directly without intermediate conversion.
func rwSets(env *common.Envelope) ([]nsData, error) {
	out := []nsData{}

	pl := &common.Payload{}
	if err := proto.Unmarshal(env.Payload, pl); err != nil {
		return out, errors.Wrap(err, "payload")
	}

	chdr := &common.ChannelHeader{}
	if err := proto.Unmarshal(pl.Header.ChannelHeader, chdr); err != nil {
		return out, errors.Wrap(err, "channel header")
	}
	txID := chdr.TxId

	tx := &protoblocktx.Tx{}
	if err := proto.Unmarshal(pl.Data, tx); err != nil {
		return out, errors.Wrap(err, "transaction")
	}

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
			TxID:        txID,
			Endorsement: endorsement,
		})
	}

	return out, nil
}
