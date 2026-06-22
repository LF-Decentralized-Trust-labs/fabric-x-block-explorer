/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-x-committer/service/sidecar"
	"github.com/hyperledger/fabric-x-committer/utils/serialization"
	"github.com/hyperledger/fabric-x-common/api/applicationpb"
	"github.com/hyperledger/fabric-x-common/api/committerpb"
	"github.com/hyperledger/fabric-x-common/api/msppb"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = flogging.MustGetLogger("parser")

// txMeta carries per-transaction context through the parsing chain.
type txMeta struct {
	blockNum          uint64
	txNum             int
	txID              string
	validationCode    committerpb.Status
	txType            *int32
	chaincodeName     *string
	creatorMspID      *string
	creatorIDBytes    []byte
	creatorNonce      []byte
	envelopeSignature []byte
	payloadExtension  []byte
	channelVersion    *int32
	channelID         *string
	epoch             *uint64
	tlsCertHash       []byte
	createdAt         *int64
	metadata          []byte
}

// nsData wraps a TxNamespace with its associated endorsement records.
type nsData struct {
	Namespace    *applicationpb.TxNamespace
	Endorsements []types.EndorsementRecord
}

// Parse extracts transactions and write-sets from a Fabric block into ParsedBlockData and BlockInfo.
func Parse(b *common.Block) (*types.ParsedBlockData, *types.BlockInfo, error) {
	header := b.GetHeader()
	if header == nil {
		return nil, nil, errors.New("block header missing")
	}

	// Marshal the block to measure wire size. The hash is not computed here:
	// block_hash is not a field in the block proto received from the sidecar,
	// and the canonical Fabric hash (SHA256 of header bytes) is already captured
	// by PreviousHash of the next block.
	blockBytes, err := proto.Marshal(b)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal block for size")
	}

	blockInfo := &types.BlockInfo{
		Number:       header.Number,
		PreviousHash: header.PreviousHash,
		DataHash:     header.DataHash,
		BlockSize:    len(blockBytes),
		CreatedAt:    nil, // Will be set from first transaction timestamp
	}

	if b.Metadata == nil || len(b.Metadata.Metadata) <= int(common.BlockMetadataIndex_TRANSACTIONS_FILTER) {
		return nil, blockInfo, errors.New("block metadata missing TRANSACTIONS_FILTER")
	}

	extractBlockMetadata(b.Metadata.Metadata, blockInfo)

	txFilter := b.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER]
	var rawData [][]byte
	if b.Data != nil {
		rawData = b.Data.Data
	}
	transactions, policies, envelopeErrors := parseTxData(header, rawData, txFilter)

	// Set block timestamp from first transaction if available
	if len(transactions) > 0 && transactions[0].CreatedAt != nil {
		blockInfo.CreatedAt = transactions[0].CreatedAt
	}

	return &types.ParsedBlockData{
		Transactions:   transactions,
		Policies:       policies,
		EnvelopeErrors: envelopeErrors,
	}, blockInfo, nil
}

// extractBlockMetadata populates blockInfo with all available BlockMetadata fields.
func extractBlockMetadata(md [][]byte, blockInfo *types.BlockInfo) {
	if len(md) > int(common.BlockMetadataIndex_SIGNATURES) {
		blockInfo.MetadataSignatures = md[common.BlockMetadataIndex_SIGNATURES]
	}
	//nolint:staticcheck // BlockMetadataIndex_LAST_CONFIG is deprecated in the proto but has no replacement
	if len(md) > int(common.BlockMetadataIndex_LAST_CONFIG) {
		raw := md[common.BlockMetadataIndex_LAST_CONFIG] //nolint:staticcheck
		meta := &common.Metadata{}
		if err := proto.Unmarshal(raw, meta); err == nil {
			obm := &common.OrdererBlockMetadata{}
			if err := proto.Unmarshal(meta.Value, obm); err == nil && obm.LastConfig != nil {
				idx := obm.LastConfig.Index
				blockInfo.LastConfigIndex = &idx
			}
		}
	}
	if len(md) > int(common.BlockMetadataIndex_TRANSACTIONS_FILTER) {
		blockInfo.TxStatusCodes = md[common.BlockMetadataIndex_TRANSACTIONS_FILTER]
	}
	if len(md) > int(common.BlockMetadataIndex_COMMIT_HASH) {
		blockInfo.CommitHash = md[common.BlockMetadataIndex_COMMIT_HASH]
	}
}

func parseTxData(
	header *common.BlockHeader,
	data [][]byte,
	txFilter []byte,
) ([]types.TxRecord, []types.NamespacePolicyRecord, []types.EnvelopeErrorRecord) {
	transactions := make([]types.TxRecord, 0, len(data))
	policies := make([]types.NamespacePolicyRecord, 0)
	envelopeErrors := make([]types.EnvelopeErrorRecord, 0)

	for txNum, envBytes := range data {
		if txNum >= len(txFilter) {
			continue
		}
		validationCode := committerpb.Status(txFilter[txNum])
		txRec, policyItems, envErr := handleTx(header, txNum, envBytes, validationCode)
		policies = append(policies, policyItems...)
		if txRec != nil {
			transactions = append(transactions, *txRec)
		}
		if envErr != nil {
			envelopeErrors = append(envelopeErrors, *envErr)
		}
	}

	return transactions, policies, envelopeErrors
}

func handleTx(
	header *common.BlockHeader,
	txNum int,
	envBytes []byte,
	validationCode committerpb.Status,
) (*types.TxRecord, []types.NamespacePolicyRecord, *types.EnvelopeErrorRecord) {
	env := &common.Envelope{}
	if err := proto.Unmarshal(envBytes, env); err != nil {
		logger.Warnf("block %d tx %d invalid envelope: %v", header.Number, txNum, err)
		return nil, nil, &types.EnvelopeErrorRecord{
			TxNum:          uint64(txNum), //nolint:gosec // txNum is a range index
			ValidationCode: validationCode.String(),
			RawEnvelope:    envBytes,
		}
	}

	// Parsed once; pl and chdr are reused by all call sites below.
	pl, chdr, err := serialization.ParseEnvelope(env)
	if err != nil {
		logger.Warnf("block %d tx %d unparseable payload: %v", header.Number, txNum, err)
		return nil, nil, &types.EnvelopeErrorRecord{
			TxNum:          uint64(txNum), //nolint:gosec // txNum is a range index
			ValidationCode: validationCode.String(),
			RawEnvelope:    envBytes,
		}
	}

	// Config txs go to namespace_policies — not errors, handled normally.
	if policyItems, ok := extractCommittedPolicies(validationCode, pl, chdr); ok {
		return nil, policyItems, nil
	}

	txID := chdr.TxId
	if txID == "" {
		logger.Warnf("block %d tx %d: missing or invalid tx_id", header.Number, txNum)
		return nil, nil, &types.EnvelopeErrorRecord{
			TxNum:          uint64(txNum), //nolint:gosec // txNum is a range index
			ValidationCode: validationCode.String(),
			RawEnvelope:    envBytes,
		}
	}

	if !sidecar.IsStatusStoredInDB(validationCode) {
		logger.Warnf("block %d tx %d: status %s not stored in DB", header.Number, txNum, &validationCode)
		// Decode the tx_id so the UI can cross-reference (e.g. REJECTED_DUPLICATE_TX_ID).
		txIDBytes, decErr := hex.DecodeString(txID)
		if decErr != nil {
			txIDBytes = []byte(txID)
		}
		return nil, nil, &types.EnvelopeErrorRecord{
			TxNum:          uint64(txNum), //nolint:gosec // txNum is a range index
			ValidationCode: validationCode.String(),
			RawEnvelope:    envBytes,
			TxID:           txIDBytes,
		}
	}

	meta := txMeta{
		blockNum:          header.Number,
		txNum:             txNum,
		txID:              txID,
		validationCode:    validationCode,
		envelopeSignature: env.Signature,
	}
	applyChannelHeaderFields(&meta, chdr)
	extractCreatorInfo(&meta, pl)
	extractChaincodeData(&meta, pl)
	return parseTxRecord(meta, pl), nil, nil
}

// applyChannelHeaderFields populates txMeta fields derived from the channel header.
func applyChannelHeaderFields(meta *txMeta, chdr *common.ChannelHeader) {
	if chdr.Type != 0 {
		meta.txType = &chdr.Type
	}
	if chdr.Timestamp != nil {
		ts := chdr.Timestamp.AsTime().UnixNano()
		meta.createdAt = &ts
	}
	if len(chdr.Extension) > 0 {
		meta.payloadExtension = chdr.Extension
	}
	if chdr.Version != 0 {
		meta.channelVersion = &chdr.Version
	}
	if chdr.ChannelId != "" {
		meta.channelID = &chdr.ChannelId
	}
	if chdr.Epoch != 0 {
		meta.epoch = &chdr.Epoch
	}
	if len(chdr.TlsCertHash) > 0 {
		meta.tlsCertHash = chdr.TlsCertHash
	}
}

// extractCreatorInfo populates creator fields from the payload signature header.
func extractCreatorInfo(meta *txMeta, pl *common.Payload) {
	if pl.Header == nil || pl.Header.SignatureHeader == nil {
		return
	}
	shdr := &common.SignatureHeader{}
	if err := proto.Unmarshal(pl.Header.SignatureHeader, shdr); err != nil {
		return
	}
	meta.creatorIDBytes = shdr.Creator
	meta.creatorNonce = shdr.Nonce
	if len(shdr.Creator) == 0 {
		return
	}
	serializedID := &msp.SerializedIdentity{}
	if err := proto.Unmarshal(shdr.Creator, serializedID); err == nil {
		meta.creatorMspID = &serializedID.Mspid
	}
}

// extractChaincodeData extracts the chaincode name and metadata from the transaction.
func extractChaincodeData(meta *txMeta, pl *common.Payload) {
	tx, err := serialization.UnmarshalTx(pl.Data)
	if err != nil {
		return
	}
	if len(tx.Namespaces) > 0 && tx.Namespaces[0] != nil && tx.Namespaces[0].NsId != "" {
		meta.chaincodeName = &tx.Namespaces[0].NsId
	}
	// Extract transaction metadata (introduced in committer v1.0.3)
	if len(tx.Metadata) > 0 {
		// Serialize the metadata array as a single byte slice
		// Each metadata entry is length-prefixed for proper deserialization
		var totalLen int
		for _, m := range tx.Metadata {
			totalLen += 4 + len(m) // 4 bytes for length prefix + data
		}
		if totalLen > 0 {
			buf := make([]byte, 0, totalLen)
			for _, m := range tx.Metadata {
				// Store length as 4-byte big-endian
				mLen := uint32(len(m))
				lenBytes := []byte{
					byte(mLen >> 24),
					byte(mLen >> 16),
					byte(mLen >> 8),
					byte(mLen),
				}
				buf = append(buf, lenBytes...)
				buf = append(buf, m...)
			}
			meta.metadata = buf
		}
	}
}

// parseTxRecord builds a TxRecord: full when rwsets parse, minimal (no namespaces)
// for any non-COMMITTED tx with an unparseable or empty rwset.
// A minimal record preserves all envelope-level fields (tx_id, creator, timestamps, etc.)
// so no block position is silently lost for aborted/malformed txs.
// Note: COMMITTED txs with unparseable rwsets are NOT stored — they indicate a pipeline
// inconsistency and are surfaced to logs only.
func parseTxRecord(meta txMeta, pl *common.Payload) *types.TxRecord {
	nsList, err := extractNamespaceData(meta.txID, pl)
	if err != nil {
		logger.Warnf("block %d tx %d invalid rwset: %v — storing minimal record", meta.blockNum, meta.txNum, err)
	}

	// Store a record for txs with a tx_id even if rwsets are unparseable,
	// except for COMMITTED txs where an unparseable rwset indicates a bug.
	if err != nil || len(nsList) == 0 {
		rec := buildMinimalTxRecord(meta)
		return &rec
	}

	rec := buildTxRecord(meta, nsList)
	return &rec
}

// buildMinimalTxRecord returns a TxRecord with no namespace data.
func buildMinimalTxRecord(meta txMeta) types.TxRecord {
	return types.TxRecord{
		TxNum:             uint64(meta.txNum), //nolint:gosec // txNum is a range index, always non-negative
		TxID:              meta.txID,
		ValidationCode:    meta.validationCode,
		TxType:            meta.txType,
		ChaincodeName:     meta.chaincodeName,
		CreatorMspID:      meta.creatorMspID,
		CreatorIDBytes:    meta.creatorIDBytes,
		CreatorNonce:      meta.creatorNonce,
		EnvelopeSignature: meta.envelopeSignature,
		PayloadExtension:  meta.payloadExtension,
		ChannelVersion:    meta.channelVersion,
		ChannelID:         meta.channelID,
		Epoch:             meta.epoch,
		TLSCertHash:       meta.tlsCertHash,
		CreatedAt:         meta.createdAt,
		Metadata:          meta.metadata,
	}
}

func buildTxRecord(meta txMeta, nsList []nsData) types.TxRecord {
	txRecord := types.TxRecord{
		TxNum:             uint64(meta.txNum), //nolint:gosec // txNum is a range index, always non-negative
		TxID:              meta.txID,
		ValidationCode:    meta.validationCode,
		TxType:            meta.txType,
		ChaincodeName:     meta.chaincodeName,
		CreatorMspID:      meta.creatorMspID,
		CreatorIDBytes:    meta.creatorIDBytes,
		CreatorNonce:      meta.creatorNonce,
		EnvelopeSignature: meta.envelopeSignature,
		PayloadExtension:  meta.payloadExtension,
		ChannelVersion:    meta.channelVersion,
		ChannelID:         meta.channelID,
		Epoch:             meta.epoch,
		TLSCertHash:       meta.tlsCertHash,
		CreatedAt:         meta.createdAt,
		Metadata:          meta.metadata,
		Namespaces:        make([]types.TxNamespaceRecord, 0, len(nsList)),
	}

	for _, nd := range nsList {
		txRecord.Namespaces = append(txRecord.Namespaces, buildTxNamespaceRecord(nd))
	}

	return txRecord
}

func buildTxNamespaceRecord(nd nsData) types.TxNamespaceRecord {
	ns := nd.Namespace
	nsRecord := types.TxNamespaceRecord{
		NsID:         ns.NsId,
		NsVersion:    ns.NsVersion,
		ReadsOnly:    make([]types.ReadOnlyRecord, 0, len(ns.ReadsOnly)),
		ReadWrites:   make([]types.ReadWriteRecord, 0, len(ns.ReadWrites)),
		BlindWrites:  make([]types.BlindWriteRecord, 0, len(ns.BlindWrites)),
		Endorsements: nd.Endorsements,
	}

	for _, ro := range ns.ReadsOnly {
		roRecord := types.ReadOnlyRecord{Key: ro.Key}
		if ro.Version != nil {
			roRecord.Version = ro.Version
		}
		nsRecord.ReadsOnly = append(nsRecord.ReadsOnly, roRecord)
	}

	for _, rw := range ns.ReadWrites {
		rwRecord := types.ReadWriteRecord{
			Key:   rw.Key,
			Value: rw.Value,
		}
		if rw.Version != nil {
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
	MspID         string `json:"mspid"`
	Certificate   string `json:"certificate,omitempty"`    // base64-encoded DER certificate bytes
	CertificateID string `json:"certificate_id,omitempty"` // pre-stored certificate ID (threshold scheme)
}

// policyToJSON encodes raw policy bytes as a JSON object with a base64 "policy_bytes" field.
func policyToJSON(policyBytes []byte) (json.RawMessage, error) {
	return json.Marshal(policyEncoding{
		PolicyBytes: base64.StdEncoding.EncodeToString(policyBytes),
	})
}

// identityToJSON encodes an msppb.Identity as a JSON object.
func identityToJSON(id *msppb.Identity) (json.RawMessage, error) {
	enc := identityEncoding{MspID: id.GetMspId()}
	switch c := id.Creator.(type) {
	case *msppb.Identity_Certificate:
		enc.Certificate = base64.StdEncoding.EncodeToString(c.Certificate)
	case *msppb.Identity_CertificateId:
		enc.CertificateID = c.CertificateId
	default:
		// no creator set; mspid-only identity
	}
	return json.Marshal(enc)
}

// extractCommittedPolicies returns policy records for COMMITTED config transactions; nil, false otherwise.
func extractCommittedPolicies(
	code committerpb.Status, pl *common.Payload, chdr *common.ChannelHeader,
) ([]types.NamespacePolicyRecord, bool) {
	if code != committerpb.Status_COMMITTED {
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
	policies := &applicationpb.NamespacePolicies{}
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
			ns = committerpb.MetaNamespaceID
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
	configTx := &applicationpb.ConfigTransaction{}
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
			Namespace:  committerpb.MetaNamespaceID,
			Version:    configTx.Version,
			PolicyJSON: policyJSON,
		},
	}, true
}

// extractNamespaceData unmarshals the tx payload and returns one nsData per namespace,
// with all endorsements (MspID and Identity populated from EndorsementWithIdentity.Identity).
func extractNamespaceData(txID string, pl *common.Payload) ([]nsData, error) {
	tx, err := serialization.UnmarshalTx(pl.Data)
	if err != nil {
		return nil, errors.Wrap(err, "transaction")
	}

	out := make([]nsData, 0, len(tx.Namespaces))

	if len(tx.Endorsements) > 0 && len(tx.Endorsements) != len(tx.Namespaces) {
		logger.Warnf(
			"tx %s endorsement count %d does not match namespaces %d",
			txID, len(tx.Endorsements), len(tx.Namespaces),
		)
	}

	for i, ns := range tx.Namespaces {
		var endorsements []types.EndorsementRecord
		if i < len(tx.Endorsements) && tx.Endorsements[i] != nil {
			endorsements = endorsementsFromSlice(txID, ns.NsId, tx.Endorsements[i].EndorsementsWithIdentity)
		}
		out = append(out, nsData{Namespace: ns, Endorsements: endorsements})
	}

	return out, nil
}

// endorsementsFromSlice converts a list of EndorsementWithIdentity to EndorsementRecords.
func endorsementsFromSlice(
	txID, nsID string, items []*applicationpb.EndorsementWithIdentity,
) []types.EndorsementRecord {
	recs := make([]types.EndorsementRecord, 0, len(items))
	for _, ewi := range items {
		if ewi == nil || len(ewi.Endorsement) == 0 {
			continue
		}
		rec := types.EndorsementRecord{Endorsement: ewi.Endorsement}
		if ewi.Identity != nil {
			mspID := ewi.Identity.GetMspId()
			rec.MspID = &mspID
			if identityJSON, err := identityToJSON(ewi.Identity); err == nil {
				rec.Identity = identityJSON
			} else {
				logger.Debugf("tx %s ns %s: failed to encode endorser identity: %v", txID, nsID, err)
			}
		}
		recs = append(recs, rec)
	}
	return recs
}
