/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package types

import (
	"encoding/json"
)

type (
	// ParsedBlockData contains all parsed data from a block, organized by transaction.
	ParsedBlockData struct {
		Transactions []TxRecord
		Policies     []NamespacePolicyRecord
	}

	// ProcessedBlock wraps parsed block data with metadata for persistence.
	ProcessedBlock struct {
		Txns      int
		Data      any
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
		BlockNum       uint64
		TxNum          uint64
		TxID           string
		ValidationCode int32
		Namespaces     []TxNamespaceRecord
	}

	// TxNamespaceRecord represents a single (transaction, namespace) pair entry,
	// containing all reads, writes, and endorsements for that namespace.
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
	// ReadOnlyRecord represents a read-only operation within a namespace.
	ReadOnlyRecord struct {
		Key     string
		Version *uint64
	}

	// ReadWriteRecord represents a read-write operation within a namespace.
	ReadWriteRecord struct {
		Key         string
		ReadVersion *uint64
		Value       []byte
	}

	// BlindWriteRecord represents a blind write operation within a namespace.
	BlindWriteRecord struct {
		Key   string
		Value []byte
	}

	// EndorsementRecord represents a signature endorsement for a namespace.
	EndorsementRecord struct {
		Endorsement []byte
		MspID       *string
		Identity    []byte
	}

	// NamespacePolicyRecord represents a policy update for a namespace.
	NamespacePolicyRecord struct {
		Namespace  string
		Version    uint64
		PolicyJSON json.RawMessage
	}
)
