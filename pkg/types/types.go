/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package types

import (
	"encoding/json"
)

// ParsedBlockData contains all parsed data from a block, organized by transaction
type ParsedBlockData struct {
	Transactions []TxRecord
	Policies     []NamespacePolicyRecord
}

// ProcessedBlock wraps parsed block data with metadata for persistence.
type ProcessedBlock struct {
	Number    uint64
	Txns      int
	Data      any
	BlockInfo *BlockInfo
}

// BlockInfo contains block header metadata.
type BlockInfo struct {
	Number       uint64
	PreviousHash []byte
	DataHash     []byte
}

// TxRecord groups all data for a single transaction within a block.
type TxRecord struct {
	BlockNum       uint64
	TxNum          uint64
	TxID           string
	ValidationCode int32
	Namespaces     []TxNamespaceRecord
}

// TxNamespaceRecord represents a single (transaction, namespace) pair entry,
// containing all reads, writes, and endorsements for that namespace.
type TxNamespaceRecord struct {
	NsID         string
	NsVersion    uint64
	Reads        []ReadRecord
	Writes       []WriteRecord
	Endorsements []EndorsementRecord
}

// ReadRecord represents a single read operation in a transaction namespace.
type ReadRecord struct {
	Key         string
	Version     *uint64
	IsReadWrite bool
}

// WriteRecord represents a single write or delete in the world state.
type WriteRecord struct {
	Key          string
	Value        []byte
	IsBlindWrite bool
	ReadVersion  *uint64
}

// EndorsementRecord represents a signature endorsement for a namespace.
type EndorsementRecord struct {
	Endorsement []byte
	MspID       *string
	Identity    []byte
}

// NamespacePolicyRecord represents a policy update for a namespace.
type NamespacePolicyRecord struct {
	Namespace  string
	Version    uint64
	PolicyJSON json.RawMessage
}
