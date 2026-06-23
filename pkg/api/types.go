/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"encoding/hex"
	"encoding/json"
	"time"
)

// HexBytes is a byte slice that JSON-marshals as a lowercase hex string.
// It provides a human-readable representation for all binary fields (hashes,
// signatures, keys, values, etc.) without the padding ambiguity of base64.
type HexBytes []byte

// MarshalJSON implements json.Marshaler.
func (h HexBytes) MarshalJSON() ([]byte, error) {
	if h == nil {
		return []byte("null"), nil
	}
	return json.Marshal(hex.EncodeToString(h))
}

// HexBytesArray is a slice of byte slices that JSON-marshals as an array of lowercase hex strings.
type HexBytesArray [][]byte

// MarshalJSON implements json.Marshaler.
func (h HexBytesArray) MarshalJSON() ([]byte, error) {
	if h == nil {
		return []byte("null"), nil
	}
	hexStrings := make([]string, len(h))
	for i, b := range h {
		hexStrings[i] = hex.EncodeToString(b)
	}
	return json.Marshal(hexStrings)
}

// ErrorResponse is the JSON body returned on all error responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// BlockHeightResponse returns the current number of stored blocks.
type BlockHeightResponse struct {
	Height int64 `json:"height"`
}

// BlockHeader contains all block-level fields shared between BlockSummary and Block.
// Embedding it eliminates duplication and ensures the two types stay in sync.
type BlockHeader struct {
	BlockNum  int64      `json:"block_num"`
	TxCount   int32      `json:"tx_count"`
	BlockSize *int32     `json:"block_size"`
	CreatedAt *time.Time `json:"created_at"`
	// PreviousHash is the SHA-256 hash of the previous block header (hex).
	PreviousHash HexBytes `json:"previous_hash"`
	// DataHash is the Merkle hash of the block's transaction data (hex).
	DataHash HexBytes `json:"data_hash"`
	// MetadataSignatures is the raw bytes from BlockMetadata[SIGNATURES]
	// (a serialised common.Metadata proto containing orderer signatures).
	MetadataSignatures HexBytes `json:"metadata_signatures"`
	// LastConfigIndex is the block number of the most recent config block,
	// decoded from BlockMetadata[LAST_CONFIG].
	LastConfigIndex *int64 `json:"last_config_index"`
	// TxStatusCodes is decoded from BlockMetadata[TRANSACTIONS_FILTER].
	// Each entry corresponds to a block envelope position and contains the
	// human-readable committer status string (e.g. "COMMITTED", "ABORTED_MVCC_CONFLICT").
	TxStatusCodes []string `json:"tx_status_codes"`
	// CommitHash is the raw bytes from BlockMetadata[COMMIT_HASH] (hex).
	CommitHash HexBytes `json:"commit_hash"`
}

// BlockSummary is a lightweight block record returned by the list endpoint.
type BlockSummary struct {
	BlockHeader
}

// ListBlocksResponse wraps the list-blocks result with pagination metadata.
type ListBlocksResponse struct {
	Blocks  []BlockSummary `json:"blocks"`
	Offset  int32          `json:"offset"`
	Limit   int32          `json:"limit"`
	HasMore bool           `json:"has_more"`
}

// Block contains full block information including transactions and envelope errors.
type Block struct {
	BlockHeader
	Transactions   []Transaction   `json:"transactions"`
	EnvelopeErrors []EnvelopeError `json:"envelope_errors"`
}

// EnvelopeError represents a block position that could not be stored as a regular
// transaction (missing tx_id, bad proto, or a duplicate flagged by the committer).
// Every block position not in `transactions` has a corresponding entry here.
type EnvelopeError struct {
	TxNum          int64  `json:"tx_num"`
	ValidationCode string `json:"validation_code"`
	// TxID is the hex-encoded transaction ID when one could be extracted
	// (e.g. REJECTED_DUPLICATE_TX_ID). Empty string when absent.
	TxID        string   `json:"tx_id"`
	RawEnvelope HexBytes `json:"raw_envelope"`
}

// CreatorIdentity is the decoded msp.SerializedIdentity from the transaction
// signature header. It exposes the MSP identifier and the PEM-encoded X.509
// certificate of the transaction signer in human-readable form.
type CreatorIdentity struct {
	MspID string `json:"msp_id"`
	// CertificatePEM is the PEM-encoded X.509 certificate of the signer.
	CertificatePEM string `json:"certificate_pem"`
}

// ChaincodeID identifies a chaincode by name, path, and version.
type ChaincodeID struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// PayloadExtension is the decoded peer.ChaincodeHeaderExtension from the
// transaction channel header. It exposes the invoking chaincode identity.
type PayloadExtension struct {
	ChaincodeID *ChaincodeID `json:"chaincode_id"`
}

// NamespaceRow contains namespace identity and version for a transaction.
type NamespaceRow struct {
	NsID      string `json:"ns_id"`
	NsVersion int64  `json:"ns_version"`
}

// Transaction contains full transaction information with all fields decoded into
// human-readable form. Binary fields are hex-encoded; structured proto fields are
// decoded into typed sub-objects.
type Transaction struct {
	BlockNum       int64  `json:"block_num"`
	TxNum          int64  `json:"tx_num"`
	TxID           string `json:"tx_id"` // hex-encoded
	ValidationCode string `json:"validation_code"`
	// TxType is the decoded Fabric HeaderType string (e.g. "ENDORSER_TRANSACTION", "CONFIG").
	TxType        *string `json:"tx_type"`
	ChaincodeName *string `json:"chaincode_name"`
	// CreatorMspID is the MSP identifier of the transaction signer.
	CreatorMspID *string `json:"creator_msp_id"`
	// CreatorIdentity is the decoded msp.SerializedIdentity containing the signer's
	// MSP ID and PEM certificate. Nil when the signature header cannot be decoded.
	CreatorIdentity *CreatorIdentity `json:"creator_identity"`
	CreatorNonce    HexBytes         `json:"creator_nonce"`
	// EnvelopeSignature is the outer envelope signature bytes (hex).
	EnvelopeSignature HexBytes `json:"envelope_signature"`
	// PayloadExtension is the decoded peer.ChaincodeHeaderExtension with chaincode identity.
	// Nil for non-chaincode transactions or when the extension cannot be decoded.
	PayloadExtension *PayloadExtension `json:"payload_extension"`
	ChannelVersion   *int32            `json:"channel_version"`
	ChannelID        *string           `json:"channel_id"`
	Epoch            *int64            `json:"epoch"`
	// TLSCertHash is the TLS certificate hash from the channel header (hex).
	TLSCertHash HexBytes   `json:"tls_cert_hash"`
	CreatedAt   *time.Time `json:"created_at"`
	// Metadata contains transaction execution metadata (array of hex-encoded byte arrays,
	// introduced in committer v1.0.3).
	// This field contains additional execution information that does not affect the world state.
	Metadata     HexBytesArray    `json:"metadata,omitempty"`
	Namespaces   []NamespaceRow   `json:"namespaces"`
	BlindWrites  []BlindWriteRow  `json:"blind_writes"`
	Endorsements []EndorsementRow `json:"endorsements"`
	ReadWrites   []ReadWriteRow   `json:"read_writes"`
	ReadsOnly    []ReadOnlyRow    `json:"reads_only"`
}

// BlindWriteRow represents a key written without a prior read (hex-encoded key/value).
type BlindWriteRow struct {
	NsID   string   `json:"ns_id"`
	SeqNum int32    `json:"seq_num"`
	Key    HexBytes `json:"key"`
	Value  HexBytes `json:"value"`
}

// EndorsementRow represents an endorser signature with decoded identity.
type EndorsementRow struct {
	NsID        string   `json:"ns_id"`
	SeqNum      int32    `json:"seq_num"`
	Endorsement HexBytes `json:"endorsement"`
	MspID       *string  `json:"msp_id"`
	// Identity is the decoded endorser identity as a JSON object
	// with fields mspid, certificate (base64 DER), or certificate_id.
	Identity json.RawMessage `json:"identity"`
}

// ReadWriteRow represents a key that was both read and written (hex-encoded key/value).
type ReadWriteRow struct {
	NsID        string   `json:"ns_id"`
	SeqNum      int32    `json:"seq_num"`
	Key         HexBytes `json:"key"`
	ReadVersion *int64   `json:"read_version"`
	Value       HexBytes `json:"value"`
}

// ReadOnlyRow represents a key that was only read, not written (hex-encoded key).
type ReadOnlyRow struct {
	NsID    string   `json:"ns_id"`
	SeqNum  int32    `json:"seq_num"`
	Key     HexBytes `json:"key"`
	Version *int64   `json:"version"`
}

// NamespacePolicyRow is a namespace policy record returned by the policies endpoint.
type NamespacePolicyRow struct {
	Namespace     string   `json:"namespace"`
	Version       int64    `json:"version"`
	Policy        string   `json:"policy"`
	Certificates  []string `json:"certificates"`
	MspIDs        []string `json:"msp_ids"`
	Endpoints     []string `json:"endpoints"`
	HashAlgorithm string   `json:"hash_algorithm"`
}

// NamespacePoliciesResponse wraps the list of namespace policies.
type NamespacePoliciesResponse struct {
	Policies []NamespacePolicyRow `json:"policies"`
}
