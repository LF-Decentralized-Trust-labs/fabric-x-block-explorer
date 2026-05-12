/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

// REST API response types

// BlockHeightResponse returns the current block height.
type BlockHeightResponse struct {
	Height int64 `json:"height"`
}

// BlockSummary contains basic block information.
type BlockSummary struct {
	BlockNum     int64  `json:"block_num"`
	TxCount      int32  `json:"tx_count"`
	PreviousHash []byte `json:"previous_hash"`
	DataHash     []byte `json:"data_hash"`
}

// ListBlocksResponse contains a list of block summaries.
type ListBlocksResponse struct {
	Blocks []BlockSummary `json:"blocks"`
}

// Block contains full block information with transactions.
type Block struct {
	BlockNum     int64         `json:"block_num"`
	TxCount      int32         `json:"tx_count"`
	PreviousHash []byte        `json:"previous_hash"`
	DataHash     []byte        `json:"data_hash"`
	Transactions []Transaction `json:"transactions"`
}

// Transaction contains full transaction information.
type Transaction struct {
	TxID           string           `json:"tx_id"` // hex-encoded
	ValidationCode string           `json:"validation_code"`
	BlindWrites    []BlindWriteRow  `json:"blind_writes,omitempty"`
	Endorsements   []EndorsementRow `json:"endorsements,omitempty"`
	ReadWrites     []ReadWriteRow   `json:"read_writes,omitempty"`
	ReadsOnly      []ReadOnlyRow    `json:"reads_only,omitempty"`

	// Internal fields - not exposed in JSON
	BlockNum int64 `json:"-"`
	TxNum    int64 `json:"-"`
}

// BlindWriteRow represents a key written without a prior read.
type BlindWriteRow struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`

	// Internal field - not exposed in JSON
	NsID string `json:"-"`
}

// EndorsementRow represents an endorsement signature.
type EndorsementRow struct {
	Endorsement []byte  `json:"endorsement"`
	MspID       *string `json:"msp_id,omitempty"`
	Identity    []byte  `json:"identity,omitempty"`

	// Internal field - not exposed in JSON
	NsID string `json:"-"`
}

// ReadWriteRow represents a key that was both read and written.
type ReadWriteRow struct {
	Key         []byte `json:"key"`
	ReadVersion *int64 `json:"read_version,omitempty"`
	Value       []byte `json:"value"`

	// Internal field - not exposed in JSON
	NsID string `json:"-"`
}

// ReadOnlyRow represents a key that was only read.
type ReadOnlyRow struct {
	Key     []byte `json:"key"`
	Version *int64 `json:"version,omitempty"`

	// Internal field - not exposed in JSON
	NsID string `json:"-"`
}

// NamespacePolicyRow contains decoded policy information.
type NamespacePolicyRow struct {
	Namespace     string   `json:"namespace"`
	Version       int64    `json:"version"`
	Policy        string   `json:"policy"`         // human-readable policy expression
	Certificates  []string `json:"certificates"`   // PEM-encoded X.509 certificates
	MspIDs        []string `json:"msp_ids"`        // e.g. ["Org1MSP", "OrdererMSP"]
	Endpoints     []string `json:"endpoints"`      // e.g. ["localhost:7050"]
	HashAlgorithm string   `json:"hash_algorithm"` // e.g. "SHA256"
}

// NamespacePoliciesResponse contains a list of namespace policies.
type NamespacePoliciesResponse struct {
	Policies []NamespacePolicyRow `json:"policies"`
}
