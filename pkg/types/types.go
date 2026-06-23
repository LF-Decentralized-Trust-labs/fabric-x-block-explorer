/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package types

import (
	"encoding/json"

	"github.com/hyperledger/fabric-x-common/api/committerpb"
)

type (
	// ParsedBlockData holds parsed block data ready for persistence.
	// Transactions contains records for all envelopes that had a parseable tx_id and a
	// DB-tracked status; its length maps to blocks.tx_count.
	// EnvelopeErrors contains every envelope that could not be keyed by tx_id.
	ParsedBlockData struct {
		Transactions   []TxRecord
		Policies       []NamespacePolicyRecord
		EnvelopeErrors []EnvelopeErrorRecord
	}

	// EnvelopeErrorRecord represents a raw block envelope that could not be stored as a
	// transaction because it lacked a parseable tx_id or was flagged as a duplicate.
	// The raw envelope bytes are preserved so the UI can display all block positions.
	EnvelopeErrorRecord struct {
		TxNum          uint64
		ValidationCode string // human-readable committer status (e.g. MALFORMED_BAD_ENVELOPE)
		RawEnvelope    []byte // original protobuf bytes from block.Data.Data[tx_num]
		TxID           []byte // non-nil only when a tx_id could be extracted (e.g. REJECTED_DUPLICATE_TX_ID)
	}

	// ProcessedBlock wraps parsed block data with metadata for persistence.
	ProcessedBlock struct {
		Data      *ParsedBlockData
		BlockInfo *BlockInfo
	}

	// BlockInfo contains block header metadata.
	BlockInfo struct {
		Number             uint64
		PreviousHash       []byte
		DataHash           []byte
		BlockSize          int
		CreatedAt          *int64  // Unix timestamp in nanoseconds
		MetadataSignatures []byte  // raw BlockMetadata[0] / SIGNATURES
		LastConfigIndex    *uint64 // decoded from BlockMetadata[1] / LAST_CONFIG
		TxStatusCodes      []byte  // raw BlockMetadata[2] / TRANSACTIONS_FILTER
		CommitHash         []byte  // raw BlockMetadata[3] / COMMIT_HASH
	}
)

type (
	// TxRecord groups all data for a single transaction within a block.
	TxRecord struct {
		TxNum             uint64
		TxID              string
		ValidationCode    committerpb.Status
		TxType            *int32
		ChaincodeName     *string
		CreatorMspID      *string
		CreatorIDBytes    []byte
		CreatorNonce      []byte
		EnvelopeSignature []byte
		PayloadExtension  []byte
		ChannelVersion    *int32
		ChannelID         *string
		Epoch             *uint64
		TLSCertHash       []byte
		CreatedAt         *int64   // Unix timestamp in nanoseconds
		Metadata          [][]byte // Transaction execution metadata (Tx.Metadata field, introduced in committer v1.0.3)
		Namespaces        []TxNamespaceRecord
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
