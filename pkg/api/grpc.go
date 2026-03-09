/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	explorerv1 "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/api/proto"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
)

const defaultTxLimit = 50

// GetBlockHeight returns the current block height (highest block number in the DB).
func (s *Service) GetBlockHeight(ctx context.Context, _ *emptypb.Empty) (*explorerv1.GetBlockHeightResponse, error) {
	h, err := s.q.GetBlockHeight(ctx)
	if err != nil {
		return nil, err
	}
	var height int64
	if v, ok := h.(int64); ok {
		height = v
	}
	return &explorerv1.GetBlockHeightResponse{Height: height}, nil
}

// ListBlocks returns a paginated range of block summaries.
// If To is 0, it is treated as no upper bound (all blocks from From onward).
func (s *Service) ListBlocks(
	ctx context.Context, req *explorerv1.ListBlocksRequest,
) (*explorerv1.ListBlocksResponse, error) {
	toNum := req.To
	if toNum == 0 {
		toNum = math.MaxInt64
	}
	lim := req.Limit
	if lim == 0 {
		lim = defaultTxLimit
	}
	rows, err := s.q.ListBlocks(ctx, dbsqlc.ListBlocksParams{
		FromNum: req.From, ToNum: toNum, Lim: lim, Off: req.Offset,
	})
	if err != nil {
		return nil, err
	}
	blocks := make([]*explorerv1.BlockSummary, len(rows))
	for i, r := range rows {
		blocks[i] = &explorerv1.BlockSummary{
			BlockNum: r.BlockNum, TxCount: r.TxCount,
			PreviousHash: r.PreviousHash, DataHash: r.DataHash,
		}
	}
	return &explorerv1.ListBlocksResponse{Blocks: blocks}, nil
}

// GetBlockDetail returns a block with its transactions and all requested sub-datasets.
func (s *Service) GetBlockDetail(
	ctx context.Context, req *explorerv1.GetBlockDetailRequest,
) (*explorerv1.BlockDetail, error) {
	b, err := s.q.GetBlock(ctx, req.BlockNum)
	if err != nil {
		return nil, notFound(err)
	}
	txLim := req.TxLimit
	if txLim == 0 {
		txLim = defaultTxLimit
	}
	txRows, err := s.q.GetValidationCodeByBlock(ctx, dbsqlc.GetValidationCodeByBlockParams{
		BlockNum: req.BlockNum, Limit: txLim, Offset: req.TxOffset,
	})
	if err != nil {
		return nil, err
	}
	txDetails := make([]*explorerv1.TxDetail, len(txRows))
	for i, tx := range txRows {
		txDetails[i], err = s.loadTxDetail(ctx, tx)
		if err != nil {
			return nil, err
		}
	}
	return &explorerv1.BlockDetail{
		BlockNum: b.BlockNum, TxCount: b.TxCount,
		PreviousHash: b.PreviousHash, DataHash: b.DataHash,
		Transactions: txDetails,
	}, nil
}

// GetTransactionDetail returns a transaction with all sub-datasets.
func (s *Service) GetTransactionDetail(
	ctx context.Context, req *explorerv1.GetTxDetailRequest,
) (*explorerv1.TxDetail, error) {
	tx, err := s.q.GetValidationCodeByTxID(ctx, req.TxId)
	if err != nil {
		return nil, notFound(err)
	}
	return s.loadTxDetail(ctx, tx)
}

// GetNamespacePolicies returns the policies for a given namespace with decoded fields.
func (s *Service) GetNamespacePolicies(
	ctx context.Context, req *explorerv1.GetNamespacePoliciesRequest,
) (*explorerv1.GetNamespacePoliciesResponse, error) {
	rows, err := s.q.GetNamespacePolicies(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	out := make([]*explorerv1.NamespacePolicyRow, len(rows))
	for i, r := range rows {
		dec := decodePolicy(r.Policy)
		out[i] = &explorerv1.NamespacePolicyRow{
			Namespace:     r.Namespace,
			Version:       r.Version,
			Policy:        r.Policy,
			Certificates:  dec.Certificates,
			MspIds:        dec.MspIDs,
			Endpoints:     dec.Endpoints,
			HashAlgorithm: dec.HashAlgorithm,
			RawText:       dec.RawText,
		}
	}
	return &explorerv1.GetNamespacePoliciesResponse{Policies: out}, nil
}

// --- shared helpers ---

// fetchBlindWrites loads blind-write rows for a transaction.
func (s *Service) fetchBlindWrites(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.BlindWriteRow, error) {
	rows, err := s.q.GetBlindWritesByTx(ctx, dbsqlc.GetBlindWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	out := make([]*explorerv1.BlindWriteRow, len(rows))
	for i, r := range rows {
		out[i] = &explorerv1.BlindWriteRow{NsId: r.NsID, Key: r.Key, Value: r.Value}
	}
	return out, nil
}

// fetchEndorsements loads endorsement rows for a transaction.
func (s *Service) fetchEndorsements(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.EndorsementRow, error) {
	rows, err := s.q.GetEndorsementsByTx(ctx, dbsqlc.GetEndorsementsByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	out := make([]*explorerv1.EndorsementRow, len(rows))
	for i, r := range rows {
		out[i] = &explorerv1.EndorsementRow{
			NsId: r.NsID, Endorsement: r.Endorsement,
			MspId: pgtextPtr(r.MspID), Identity: r.Identity,
		}
	}
	return out, nil
}

// fetchReadWrites loads read-write rows for a transaction.
func (s *Service) fetchReadWrites(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.ReadWriteRow, error) {
	rows, err := s.q.GetReadWritesByTx(ctx, dbsqlc.GetReadWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	out := make([]*explorerv1.ReadWriteRow, len(rows))
	for i, r := range rows {
		out[i] = &explorerv1.ReadWriteRow{
			NsId: r.NsID, Key: r.Key,
			ReadVersion: pgint8Ptr(r.ReadVersion), Value: r.Value,
		}
	}
	return out, nil
}

// fetchReadsOnly loads read-only rows for a transaction.
func (s *Service) fetchReadsOnly(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.ReadOnlyRow, error) {
	rows, err := s.q.GetReadsOnlyByTx(ctx, dbsqlc.GetReadsOnlyByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	out := make([]*explorerv1.ReadOnlyRow, len(rows))
	for i, r := range rows {
		out[i] = &explorerv1.ReadOnlyRow{NsId: r.NsID, Key: r.Key, Version: pgint8Ptr(r.Version)}
	}
	return out, nil
}

// loadTxDetail fetches all sub-dataset rows for a single transaction.
func (s *Service) loadTxDetail(
	ctx context.Context, tx dbsqlc.Transaction,
) (*explorerv1.TxDetail, error) {
	detail := &explorerv1.TxDetail{
		BlockNum: tx.BlockNum, TxNum: tx.TxNum,
		TxId: tx.TxID, ValidationCode: int32(tx.ValidationCode),
	}

	bw, err := s.fetchBlindWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.BlindWrites = bw

	en, err := s.fetchEndorsements(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.Endorsements = en

	rw, err := s.fetchReadWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.ReadWrites = rw

	ro, err := s.fetchReadsOnly(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.ReadsOnly = ro

	return detail, nil
}

func pgint8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

// notFound maps pgx.ErrNoRows to a gRPC NotFound status; other errors pass through.
func notFound(err error) error {
	if pgx.ErrNoRows == err { //nolint:errorlint // pgx.ErrNoRows is a sentinel, direct comparison is correct
		return status.Error(codes.NotFound, err.Error())
	}
	return err
}

func pgtextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
