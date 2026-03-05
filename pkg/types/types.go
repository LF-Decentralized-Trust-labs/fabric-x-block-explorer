/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package types

import (
	"encoding/json"

	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
)

type (
	// ParsedBlockData holds parsed block data ready for persistence.
	// Transactions contains only records that will be stored (excludes bad envelopes,
	// config txs, and untracked statuses); its length maps to blocks.tx_count.
	ParsedBlockData struct {
		Transactions []TxRecord
		Policies     []NamespacePolicyRecord
	}

	// ProcessedBlock wraps parsed block data with metadata for persistence.
	ProcessedBlock struct {
		Data      *ParsedBlockData
		BlockInfo *BlockInfo
	}

	// BlockInfo contains block header metadata.
	BlockInfo struct {
		Number       uint64
		PreviousHash []byte
		DataHash     []byte
	}
)

type (
	// TxRecord groups all data for a single transaction within a block.
	TxRecord struct {
		TxNum          uint64
		TxID           string
		ValidationCode protoblocktx.Status
		Namespaces     []TxNamespaceRecord
	}

	// TxNamespaceRecord holds all reads, writes, and endorsements for one (tx, namespace) pair.
	TxNamespaceRecord struct {
		NsID         string
		NsVersion    uint64
		ReadsOnly    []ReadOnlyRecord
		ReadWrites   []ReadWriteRecord
		BlindWrites  []BlindWriteRecord
		Endorsements []EndorsementRecord
	}
)

type (
	// ReadOnlyRecord is a key read but not written; Version is nil if the key was absent.
	ReadOnlyRecord struct {
		Key     []byte
		Version *uint64
	}

	// ReadWriteRecord is a key both read and written; ReadVersion is nil if the key was absent.
	ReadWriteRecord struct {
		Key         []byte
		ReadVersion *uint64
		Value       []byte
	}

	// BlindWriteRecord is a key written without a prior read (no MVCC version check).
	BlindWriteRecord struct {
		Key   []byte
		Value []byte
	}

	// EndorsementRecord holds raw endorsement bytes and the parsed MSP identity.
	EndorsementRecord struct {
		Endorsement []byte
		MspID       *string
		Identity    json.RawMessage
	}

	// NamespacePolicyRecord represents a policy update for a namespace.
	NamespacePolicyRecord struct {
		Namespace  string
		Version    uint64
		PolicyJSON json.RawMessage
	}
)
