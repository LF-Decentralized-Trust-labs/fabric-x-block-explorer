/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"encoding/hex"
	"math"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	explorerv1 "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/api/proto"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
)

// GetBlockHeight returns the current block height (highest block number in the DB).
func (s *Service) GetBlockHeight(ctx context.Context, _ *emptypb.Empty) (*explorerv1.GetBlockHeightResponse, error) {
	heightResult, err := s.q.GetBlockHeight(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var height int64
	switch v := heightResult.(type) {
	case int64:
		height = v
	case nil:
		// no blocks yet, height is 0
	default:
		return nil, status.Errorf(codes.Internal, "unexpected block height type: %T", v)
	}
	return &explorerv1.GetBlockHeightResponse{Height: height}, nil
}

// ListBlocks returns a paginated range of block summaries.
// If To is 0, it is treated as no upper bound (all blocks from From onward).
func (s *Service) ListBlocks(
	ctx context.Context, req *explorerv1.ListBlocksRequest,
) (*explorerv1.ListBlocksResponse, error) {
	if err := validateListBlocksRequest(req); err != nil {
		return nil, err
	}
	toNum := req.To
	if toNum == 0 {
		toNum = math.MaxInt64
	}
	limit := req.Limit
	if limit == 0 {
		limit = s.defaultTxLimit()
	}
	rows, err := s.q.ListBlocks(ctx, dbsqlc.ListBlocksParams{
		FromNum: req.From, ToNum: toNum, Lim: limit, Off: req.Offset,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	blocks := make([]*explorerv1.BlockSummary, len(rows))
	for i, row := range rows {
		blocks[i] = &explorerv1.BlockSummary{
			BlockNum: row.BlockNum, TxCount: row.TxCount,
			PreviousHash: row.PreviousHash, DataHash: row.DataHash,
		}
	}
	return &explorerv1.ListBlocksResponse{Blocks: blocks}, nil
}

// GetBlockDetail returns a block with its transactions and all requested sub-datasets.
func (s *Service) GetBlockDetail(
	ctx context.Context, req *explorerv1.GetBlockDetailRequest,
) (*explorerv1.BlockDetail, error) {
	if err := validateBlockDetailRequest(req); err != nil {
		return nil, err
	}
	blockRow, err := s.q.GetBlock(ctx, req.BlockNum)
	if err != nil {
		return nil, notFound(err)
	}
	txLimit := req.TxLimit
	if txLimit == 0 {
		txLimit = s.defaultTxLimit()
	}
	txRows, err := s.q.GetValidationCodeByBlock(ctx, dbsqlc.GetValidationCodeByBlockParams{
		BlockNum: req.BlockNum, Limit: txLimit, Offset: req.TxOffset,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	txDetails, err := s.loadBlockTxDetails(ctx, txRows)
	if err != nil {
		return nil, err
	}
	return &explorerv1.BlockDetail{
		BlockNum: blockRow.BlockNum, TxCount: blockRow.TxCount,
		PreviousHash: blockRow.PreviousHash, DataHash: blockRow.DataHash,
		Transactions: txDetails,
	}, nil
}

type blockTxDatasets struct {
	blindWrites  map[int64][]*explorerv1.BlindWriteRow
	endorsements map[int64][]*explorerv1.EndorsementRow
	readWrites   map[int64][]*explorerv1.ReadWriteRow
	readsOnly    map[int64][]*explorerv1.ReadOnlyRow
}

// GetTransactionDetail returns a transaction with all sub-datasets.
func (s *Service) GetTransactionDetail(
	ctx context.Context, req *explorerv1.GetTxDetailRequest,
) (*explorerv1.TxDetail, error) {
	if err := validateTxDetailRequest(req); err != nil {
		return nil, err
	}
	txID, err := hex.DecodeString(req.TxId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "tx_id must be hex-encoded")
	}
	tx, err := s.q.GetValidationCodeByTxID(ctx, txID)
	if err != nil {
		return nil, notFound(err)
	}
	return s.loadTxDetail(ctx, tx)
}

// GetNamespacePolicies returns the policies for a given namespace with decoded fields.
func (s *Service) GetNamespacePolicies(
	ctx context.Context, req *explorerv1.GetNamespacePoliciesRequest,
) (*explorerv1.GetNamespacePoliciesResponse, error) {
	if err := validateNamespacePoliciesRequest(req); err != nil {
		return nil, err
	}
	rows, err := s.q.GetNamespacePolicies(ctx, req.Namespace)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	policies := make([]*explorerv1.NamespacePolicyRow, len(rows))
	for i, row := range rows {
		decodedPol := decodePolicy(row.Policy)
		policies[i] = &explorerv1.NamespacePolicyRow{
			Namespace:     row.Namespace,
			Version:       row.Version,
			Policy:        decodedPol.PolicyExpression,
			Certificates:  decodedPol.Certificates,
			MspIds:        decodedPol.MspIDs,
			Endpoints:     decodedPol.Endpoints,
			HashAlgorithm: decodedPol.HashAlgorithm,
		}
	}
	return &explorerv1.GetNamespacePoliciesResponse{Policies: policies}, nil
}

// --- shared helpers ---

// fetchBlindWrites loads blind-write rows for a transaction.
func (s *Service) fetchBlindWrites(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.BlindWriteRow, error) {
	rows, err := s.q.GetBlindWritesByTx(ctx, dbsqlc.GetBlindWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return mapBlindWritesByTx(rows), nil
}

// fetchEndorsements loads endorsement rows for a transaction.
func (s *Service) fetchEndorsements(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.EndorsementRow, error) {
	rows, err := s.q.GetEndorsementsByTx(ctx, dbsqlc.GetEndorsementsByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return mapEndorsementsByTx(rows), nil
}

// fetchReadWrites loads read-write rows for a transaction.
func (s *Service) fetchReadWrites(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.ReadWriteRow, error) {
	rows, err := s.q.GetReadWritesByTx(ctx, dbsqlc.GetReadWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return mapReadWritesByTx(rows), nil
}

// fetchReadsOnly loads read-only rows for a transaction.
func (s *Service) fetchReadsOnly(ctx context.Context, blockNum, txNum int64) ([]*explorerv1.ReadOnlyRow, error) {
	rows, err := s.q.GetReadsOnlyByTx(ctx, dbsqlc.GetReadsOnlyByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return mapReadsOnlyByTx(rows), nil
}

// loadTxDetail fetches all sub-dataset rows for a single transaction.
func (s *Service) loadTxDetail(
	ctx context.Context, tx dbsqlc.Transaction,
) (*explorerv1.TxDetail, error) {
	detail := &explorerv1.TxDetail{
		BlockNum: tx.BlockNum, TxNum: tx.TxNum,
		TxId: hex.EncodeToString(tx.TxID), ValidationCode: int32(tx.ValidationCode),
	}

	blindWrites, err := s.fetchBlindWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.BlindWrites = blindWrites

	endorsements, err := s.fetchEndorsements(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.Endorsements = endorsements

	readWrites, err := s.fetchReadWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.ReadWrites = readWrites

	readsOnly, err := s.fetchReadsOnly(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return nil, err
	}
	detail.ReadsOnly = readsOnly

	return detail, nil
}

func (s *Service) loadBlockTxDetails(ctx context.Context, txRows []dbsqlc.Transaction) ([]*explorerv1.TxDetail, error) {
	if len(txRows) == 0 {
		return []*explorerv1.TxDetail{}, nil
	}

	datasets, err := s.fetchBlockTxDatasets(ctx, txRows)
	if err != nil {
		return nil, err
	}

	details := make([]*explorerv1.TxDetail, len(txRows))
	for i, tx := range txRows {
		details[i] = newTxDetail(tx, datasets)
	}

	return details, nil
}

func (s *Service) fetchBlockTxDatasets(ctx context.Context, txRows []dbsqlc.Transaction) (*blockTxDatasets, error) {
	blockNum, startTxNum, endTxNum := txBlockRange(txRows)

	blindWritesRows, err := s.q.GetBlindWritesByBlockTxRange(ctx, dbsqlc.GetBlindWritesByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	endorsementRows, err := s.q.GetEndorsementsByBlockTxRange(ctx, dbsqlc.GetEndorsementsByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	readWriteRows, err := s.q.GetReadWritesByBlockTxRange(ctx, dbsqlc.GetReadWritesByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	readsOnlyRows, err := s.q.GetReadsOnlyByBlockTxRange(ctx, dbsqlc.GetReadsOnlyByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	datasets := newBlockTxDatasets(txRows)

	datasets.addBlindWrites(blindWritesRows)
	datasets.addEndorsements(endorsementRows)
	datasets.addReadWrites(readWriteRows)
	datasets.addReadsOnly(readsOnlyRows)

	return datasets, nil
}

func txBlockRange(txRows []dbsqlc.Transaction) (blockNum, startTxNum, endTxNum int64) {
	blockNum = txRows[0].BlockNum
	return blockNum, txRows[0].TxNum, txRows[len(txRows)-1].TxNum + 1
}

func newTxDetail(tx dbsqlc.Transaction, datasets *blockTxDatasets) *explorerv1.TxDetail {
	return &explorerv1.TxDetail{
		BlockNum:       tx.BlockNum,
		TxNum:          tx.TxNum,
		TxId:           hex.EncodeToString(tx.TxID),
		ValidationCode: int32(tx.ValidationCode),
		BlindWrites:    datasets.blindWrites[tx.TxNum],
		Endorsements:   datasets.endorsements[tx.TxNum],
		ReadWrites:     datasets.readWrites[tx.TxNum],
		ReadsOnly:      datasets.readsOnly[tx.TxNum],
	}
}

func newBlockTxDatasets(txRows []dbsqlc.Transaction) *blockTxDatasets {
	datasets := &blockTxDatasets{
		blindWrites:  make(map[int64][]*explorerv1.BlindWriteRow, len(txRows)),
		endorsements: make(map[int64][]*explorerv1.EndorsementRow, len(txRows)),
		readWrites:   make(map[int64][]*explorerv1.ReadWriteRow, len(txRows)),
		readsOnly:    make(map[int64][]*explorerv1.ReadOnlyRow, len(txRows)),
	}
	for _, tx := range txRows {
		datasets.blindWrites[tx.TxNum] = []*explorerv1.BlindWriteRow{}
		datasets.endorsements[tx.TxNum] = []*explorerv1.EndorsementRow{}
		datasets.readWrites[tx.TxNum] = []*explorerv1.ReadWriteRow{}
		datasets.readsOnly[tx.TxNum] = []*explorerv1.ReadOnlyRow{}
	}
	return datasets
}

func (d *blockTxDatasets) addBlindWrites(rows []dbsqlc.GetBlindWritesByBlockTxRangeRow) {
	for _, row := range rows {
		d.blindWrites[row.TxNum] = append(d.blindWrites[row.TxNum], newBlindWriteRow(row.NsID, row.Key, row.Value))
	}
}

func (d *blockTxDatasets) addEndorsements(rows []dbsqlc.GetEndorsementsByBlockTxRangeRow) {
	for _, row := range rows {
		d.endorsements[row.TxNum] = append(
			d.endorsements[row.TxNum],
			newEndorsementRow(row.NsID, row.Endorsement, row.MspID, row.Identity),
		)
	}
}

func (d *blockTxDatasets) addReadWrites(rows []dbsqlc.GetReadWritesByBlockTxRangeRow) {
	for _, row := range rows {
		d.readWrites[row.TxNum] = append(
			d.readWrites[row.TxNum],
			newReadWriteRow(row.NsID, row.Key, row.ReadVersion, row.Value),
		)
	}
}

func (d *blockTxDatasets) addReadsOnly(rows []dbsqlc.GetReadsOnlyByBlockTxRangeRow) {
	for _, row := range rows {
		d.readsOnly[row.TxNum] = append(d.readsOnly[row.TxNum], newReadOnlyRow(row.NsID, row.Key, row.Version))
	}
}

func mapBlindWritesByTx(rows []dbsqlc.GetBlindWritesByTxRow) []*explorerv1.BlindWriteRow {
	result := make([]*explorerv1.BlindWriteRow, len(rows))
	for i, row := range rows {
		result[i] = newBlindWriteRow(row.NsID, row.Key, row.Value)
	}
	return result
}

func mapEndorsementsByTx(rows []dbsqlc.GetEndorsementsByTxRow) []*explorerv1.EndorsementRow {
	result := make([]*explorerv1.EndorsementRow, len(rows))
	for i, row := range rows {
		result[i] = newEndorsementRow(row.NsID, row.Endorsement, row.MspID, row.Identity)
	}
	return result
}

func mapReadWritesByTx(rows []dbsqlc.GetReadWritesByTxRow) []*explorerv1.ReadWriteRow {
	result := make([]*explorerv1.ReadWriteRow, len(rows))
	for i, row := range rows {
		result[i] = newReadWriteRow(row.NsID, row.Key, row.ReadVersion, row.Value)
	}
	return result
}

func mapReadsOnlyByTx(rows []dbsqlc.GetReadsOnlyByTxRow) []*explorerv1.ReadOnlyRow {
	result := make([]*explorerv1.ReadOnlyRow, len(rows))
	for i, row := range rows {
		result[i] = newReadOnlyRow(row.NsID, row.Key, row.Version)
	}
	return result
}

func newBlindWriteRow(nsID string, key, value []byte) *explorerv1.BlindWriteRow {
	return &explorerv1.BlindWriteRow{NsId: nsID, Key: key, Value: value}
}

func newEndorsementRow(nsID string, endorsement []byte, mspID pgtype.Text, identity []byte) *explorerv1.EndorsementRow {
	return &explorerv1.EndorsementRow{
		NsId:        nsID,
		Endorsement: endorsement,
		MspId:       pgtextPtr(mspID),
		Identity:    identity,
	}
}

func newReadWriteRow(nsID string, key []byte, readVersion pgtype.Int8, value []byte) *explorerv1.ReadWriteRow {
	return &explorerv1.ReadWriteRow{
		NsId:        nsID,
		Key:         key,
		ReadVersion: pgint8Ptr(readVersion),
		Value:       value,
	}
}

func newReadOnlyRow(nsID string, key []byte, version pgtype.Int8) *explorerv1.ReadOnlyRow {
	return &explorerv1.ReadOnlyRow{NsId: nsID, Key: key, Version: pgint8Ptr(version)}
}

// defaultTxLimit returns the configured default transaction limit, falling
// back to a safe hardcoded value if configuration is unavailable.
func (s *Service) defaultTxLimit() int32 {
	if s != nil && s.cfg != nil && s.cfg.Server.REST.DefaultTxLimit > 0 {
		return s.cfg.Server.REST.DefaultTxLimit
	}
	return config.DefaultTxLimit
}

func pgint8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func validateListBlocksRequest(req *explorerv1.ListBlocksRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.From < 0 {
		return status.Error(codes.InvalidArgument, "from must be >= 0")
	}
	if req.To < 0 {
		return status.Error(codes.InvalidArgument, "to must be >= 0")
	}
	if req.To != 0 && req.To < req.From {
		return status.Error(codes.InvalidArgument, "to must be >= from")
	}
	if req.Limit < 0 {
		return status.Error(codes.InvalidArgument, "limit must be >= 0")
	}
	if req.Offset < 0 {
		return status.Error(codes.InvalidArgument, "offset must be >= 0")
	}
	return nil
}

func validateBlockDetailRequest(req *explorerv1.GetBlockDetailRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.BlockNum < 0 {
		return status.Error(codes.InvalidArgument, "block_num must be >= 0")
	}
	if req.TxLimit < 0 {
		return status.Error(codes.InvalidArgument, "tx_limit must be >= 0")
	}
	if req.TxOffset < 0 {
		return status.Error(codes.InvalidArgument, "tx_offset must be >= 0")
	}
	return nil
}

func validateTxDetailRequest(req *explorerv1.GetTxDetailRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.TxId == "" {
		return status.Error(codes.InvalidArgument, "tx_id is required")
	}
	return nil
}

func validateNamespacePoliciesRequest(req *explorerv1.GetNamespacePoliciesRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.Namespace == "" {
		return status.Error(codes.InvalidArgument, "namespace is required")
	}
	return nil
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
