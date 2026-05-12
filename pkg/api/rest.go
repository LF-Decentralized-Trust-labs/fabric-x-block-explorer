/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"

	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
)

// newRESTRouter registers all REST endpoints.
//
//	GET /blocks/height
//	GET /blocks                      ?from=&to=&limit=&offset=
//	GET /blocks/{block_num}          ?tx_limit=&tx_offset=
//	GET /transactions/{tx_id}        (tx_id is hex)
//	GET /namespaces/{namespace}/policies
func (s *Service) newRESTRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blocks/height", s.handleGetBlockHeight)
	mux.HandleFunc("GET /blocks", s.handleListBlocks)
	mux.HandleFunc("GET /blocks/{block_num}", s.handleGetBlockByNumber)
	mux.HandleFunc("GET /transactions/{tx_id}", s.handleGetTxByID)
	mux.HandleFunc("GET /namespaces/{namespace}/policies", s.handleGetNamespacePolicies)
	return mux
}

func (s *Service) handleGetBlockHeight(w http.ResponseWriter, r *http.Request) {
	heightResult, err := s.querier.GetBlockHeight(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}

	var height int64
	switch v := heightResult.(type) {
	case int64:
		height = v
	case nil:
		// no blocks yet, height is 0
		height = 0
	default:
		respondError(w, errors.Errorf("unexpected block height type: %T", v))
		return
	}

	respondJSON(w, BlockHeightResponse{Height: height})
}

func (s *Service) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	from, err := queryOptionalInt64(r, "from")
	if err != nil {
		respondError(w, err)
		return
	}
	to, err := queryOptionalInt64(r, "to")
	if err != nil {
		respondError(w, err)
		return
	}
	limit, err := queryOptionalInt32(r, "limit")
	if err != nil {
		respondError(w, err)
		return
	}
	offset, err := queryOptionalInt32(r, "offset")
	if err != nil {
		respondError(w, err)
		return
	}

	// Validation
	if from < 0 {
		respondError(w, errors.New("from must be >= 0"))
		return
	}
	if to < 0 {
		respondError(w, errors.New("to must be >= 0"))
		return
	}
	if to != 0 && to < from {
		respondError(w, errors.New("to must be >= from"))
		return
	}
	if limit < 0 {
		respondError(w, errors.New("limit must be >= 0"))
		return
	}
	if offset < 0 {
		respondError(w, errors.New("offset must be >= 0"))
		return
	}

	toNum := to
	if toNum == 0 {
		toNum = math.MaxInt64
	}
	if limit == 0 {
		limit = s.defaultTxLimit()
	}

	rows, err := s.querier.ListBlocks(r.Context(), dbsqlc.ListBlocksParams{
		FromNum: from, ToNum: toNum, Lim: limit, Off: offset,
	})
	if err != nil {
		respondError(w, err)
		return
	}

	blocks := make([]BlockSummary, len(rows))
	for i, row := range rows {
		blocks[i] = BlockSummary{
			BlockNum:     row.BlockNum,
			TxCount:      row.TxCount,
			PreviousHash: row.PreviousHash,
			DataHash:     row.DataHash,
		}
	}

	respondJSON(w, ListBlocksResponse{Blocks: blocks})
}

func (s *Service) handleGetBlockByNumber(w http.ResponseWriter, r *http.Request) {
	blockNum, err := pathInt64(r, "block_num")
	if err != nil {
		respondError(w, err)
		return
	}
	txLimit, err := queryOptionalInt32(r, "tx_limit")
	if err != nil {
		respondError(w, err)
		return
	}
	txOffset, err := queryOptionalInt32(r, "tx_offset")
	if err != nil {
		respondError(w, err)
		return
	}

	// Validation
	if blockNum < 0 {
		respondError(w, errors.New("block_num must be >= 0"))
		return
	}
	if txLimit < 0 {
		respondError(w, errors.New("tx_limit must be >= 0"))
		return
	}
	if txOffset < 0 {
		respondError(w, errors.New("tx_offset must be >= 0"))
		return
	}

	blockRow, err := s.querier.GetBlock(r.Context(), blockNum)
	if err != nil {
		respondError(w, err)
		return
	}

	if txLimit == 0 {
		txLimit = s.defaultTxLimit()
	}

	txRows, err := s.querier.GetValidationCodeByBlock(r.Context(), dbsqlc.GetValidationCodeByBlockParams{
		BlockNum: blockNum, Limit: txLimit, Offset: txOffset,
	})
	if err != nil {
		respondError(w, err)
		return
	}

	txDetails, err := s.loadBlockTransactions(r.Context(), txRows)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, Block{
		BlockNum:     blockRow.BlockNum,
		TxCount:      blockRow.TxCount,
		PreviousHash: blockRow.PreviousHash,
		DataHash:     blockRow.DataHash,
		Transactions: txDetails,
	})
}

func (s *Service) handleGetTxByID(w http.ResponseWriter, r *http.Request) {
	txIDHex := r.PathValue("tx_id")
	if txIDHex == "" {
		respondError(w, errors.New("tx_id is required"))
		return
	}

	txID, err := hex.DecodeString(txIDHex)
	if err != nil {
		respondError(w, errors.New("tx_id must be hex-encoded"))
		return
	}

	tx, err := s.querier.GetValidationCodeByTxID(r.Context(), txID)
	if err != nil {
		respondError(w, err)
		return
	}

	txDetail, err := s.loadTransaction(r.Context(), tx)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, txDetail)
}

func (s *Service) handleGetNamespacePolicies(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	if namespace == "" {
		respondError(w, errors.New("namespace is required"))
		return
	}

	rows, err := s.querier.GetNamespacePolicies(r.Context(), namespace)
	if err != nil {
		respondError(w, err)
		return
	}

	policies := make([]NamespacePolicyRow, len(rows))
	for i, row := range rows {
		decodedPol := decodePolicy(row.Policy)
		policies[i] = NamespacePolicyRow{
			Namespace:     row.Namespace,
			Version:       row.Version,
			Policy:        decodedPol.PolicyExpression,
			Certificates:  decodedPol.Certificates,
			MspIDs:        decodedPol.MspIDs,
			Endpoints:     decodedPol.Endpoints,
			HashAlgorithm: decodedPol.HashAlgorithm,
		}
	}

	respondJSON(w, NamespacePoliciesResponse{Policies: policies})
}

// --- Helper functions ---

func (s *Service) loadTransaction(ctx context.Context, tx dbsqlc.Transaction) (Transaction, error) {
	detail := Transaction{
		BlockNum:       tx.BlockNum,
		TxNum:          tx.TxNum,
		TxID:           hex.EncodeToString(tx.TxID),
		ValidationCode: protoblocktx.Status(tx.ValidationCode).String(),
	}

	blindWrites, err := s.fetchBlindWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return Transaction{}, err
	}
	detail.BlindWrites = blindWrites

	endorsements, err := s.fetchEndorsements(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return Transaction{}, err
	}
	detail.Endorsements = endorsements

	readWrites, err := s.fetchReadWrites(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return Transaction{}, err
	}
	detail.ReadWrites = readWrites

	readsOnly, err := s.fetchReadsOnly(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return Transaction{}, err
	}
	detail.ReadsOnly = readsOnly

	return detail, nil
}

func (s *Service) loadBlockTransactions(ctx context.Context, txRows []dbsqlc.Transaction) ([]Transaction, error) {
	if len(txRows) == 0 {
		return []Transaction{}, nil
	}

	datasets, err := s.fetchBlockTxDatasets(ctx, txRows)
	if err != nil {
		return nil, err
	}

	details := make([]Transaction, len(txRows))
	for i, tx := range txRows {
		details[i] = Transaction{
			BlockNum:       tx.BlockNum,
			TxNum:          tx.TxNum,
			TxID:           hex.EncodeToString(tx.TxID),
			ValidationCode: protoblocktx.Status(tx.ValidationCode).String(),
			BlindWrites:    datasets.blindWrites[tx.TxNum],
			Endorsements:   datasets.endorsements[tx.TxNum],
			ReadWrites:     datasets.readWrites[tx.TxNum],
			ReadsOnly:      datasets.readsOnly[tx.TxNum],
		}
	}

	return details, nil
}

type blockTxDatasets struct {
	blindWrites  map[int64][]BlindWriteRow
	endorsements map[int64][]EndorsementRow
	readWrites   map[int64][]ReadWriteRow
	readsOnly    map[int64][]ReadOnlyRow
}

func (s *Service) fetchBlockTxDatasets(ctx context.Context, txRows []dbsqlc.Transaction) (*blockTxDatasets, error) {
	blockNum := txRows[0].BlockNum
	startTxNum := txRows[0].TxNum
	endTxNum := txRows[len(txRows)-1].TxNum + 1

	blindWritesRows, err := s.querier.GetBlindWritesByBlockTxRange(ctx, dbsqlc.GetBlindWritesByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, err
	}

	endorsementRows, err := s.querier.GetEndorsementsByBlockTxRange(ctx, dbsqlc.GetEndorsementsByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, err
	}

	readWriteRows, err := s.querier.GetReadWritesByBlockTxRange(ctx, dbsqlc.GetReadWritesByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, err
	}

	readsOnlyRows, err := s.querier.GetReadsOnlyByBlockTxRange(ctx, dbsqlc.GetReadsOnlyByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, err
	}

	datasets := &blockTxDatasets{
		blindWrites:  make(map[int64][]BlindWriteRow, len(txRows)),
		endorsements: make(map[int64][]EndorsementRow, len(txRows)),
		readWrites:   make(map[int64][]ReadWriteRow, len(txRows)),
		readsOnly:    make(map[int64][]ReadOnlyRow, len(txRows)),
	}

	for _, tx := range txRows {
		datasets.blindWrites[tx.TxNum] = []BlindWriteRow{}
		datasets.endorsements[tx.TxNum] = []EndorsementRow{}
		datasets.readWrites[tx.TxNum] = []ReadWriteRow{}
		datasets.readsOnly[tx.TxNum] = []ReadOnlyRow{}
	}

	for _, row := range blindWritesRows {
		datasets.blindWrites[row.TxNum] = append(datasets.blindWrites[row.TxNum], BlindWriteRow{
			NsID:  row.NsID,
			Key:   row.Key,
			Value: row.Value,
		})
	}

	for _, row := range endorsementRows {
		datasets.endorsements[row.TxNum] = append(datasets.endorsements[row.TxNum], EndorsementRow{
			NsID:        row.NsID,
			Endorsement: row.Endorsement,
			MspID:       util.NullableStringToPtr(row.MspID),
			Identity:    row.Identity,
		})
	}

	for _, row := range readWriteRows {
		datasets.readWrites[row.TxNum] = append(datasets.readWrites[row.TxNum], ReadWriteRow{
			NsID:        row.NsID,
			Key:         row.Key,
			ReadVersion: util.NullableInt64ToPtr(row.ReadVersion),
			Value:       row.Value,
		})
	}

	for _, row := range readsOnlyRows {
		datasets.readsOnly[row.TxNum] = append(datasets.readsOnly[row.TxNum], ReadOnlyRow{
			NsID:    row.NsID,
			Key:     row.Key,
			Version: util.NullableInt64ToPtr(row.Version),
		})
	}

	return datasets, nil
}

func (s *Service) fetchBlindWrites(ctx context.Context, blockNum, txNum int64) ([]BlindWriteRow, error) {
	rows, err := s.querier.GetBlindWritesByTx(ctx, dbsqlc.GetBlindWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]BlindWriteRow, len(rows))
	for i, row := range rows {
		result[i] = BlindWriteRow{NsID: row.NsID, Key: row.Key, Value: row.Value}
	}
	return result, nil
}

func (s *Service) fetchEndorsements(ctx context.Context, blockNum, txNum int64) ([]EndorsementRow, error) {
	rows, err := s.querier.GetEndorsementsByTx(ctx, dbsqlc.GetEndorsementsByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]EndorsementRow, len(rows))
	for i, row := range rows {
		result[i] = EndorsementRow{
			NsID:        row.NsID,
			Endorsement: row.Endorsement,
			MspID:       util.NullableStringToPtr(row.MspID),
			Identity:    row.Identity,
		}
	}
	return result, nil
}

func (s *Service) fetchReadWrites(ctx context.Context, blockNum, txNum int64) ([]ReadWriteRow, error) {
	rows, err := s.querier.GetReadWritesByTx(ctx, dbsqlc.GetReadWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]ReadWriteRow, len(rows))
	for i, row := range rows {
		result[i] = ReadWriteRow{
			NsID:        row.NsID,
			Key:         row.Key,
			ReadVersion: util.NullableInt64ToPtr(row.ReadVersion),
			Value:       row.Value,
		}
	}
	return result, nil
}

func (s *Service) fetchReadsOnly(ctx context.Context, blockNum, txNum int64) ([]ReadOnlyRow, error) {
	rows, err := s.querier.GetReadsOnlyByTx(ctx, dbsqlc.GetReadsOnlyByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]ReadOnlyRow, len(rows))
	for i, row := range rows {
		result[i] = ReadOnlyRow{NsID: row.NsID, Key: row.Key, Version: util.NullableInt64ToPtr(row.Version)}
	}
	return result, nil
}

func (s *Service) defaultTxLimit() int32 {
	if s != nil && s.config != nil && s.config.Server.REST.DefaultTxLimit > 0 {
		return s.config.Server.REST.DefaultTxLimit
	}
	return 100 // default fallback
}

// --- HTTP helpers ---

func respondJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func respondError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		http.Error(w, "request cancelled", 499)
		return
	}
	// Default to bad request for validation errors, internal server error for others
	if isValidationError(err) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func isValidationError(err error) bool {
	msg := err.Error()
	return errors.Is(err, strconv.ErrSyntax) ||
		errors.Is(err, strconv.ErrRange) ||
		contains(msg, "must be") ||
		contains(msg, "required") ||
		contains(msg, "invalid")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func pathInt64(r *http.Request, key string) (int64, error) {
	v := r.PathValue(key)
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, errors.Errorf("%s must be an integer: %q", key, v)
	}
	if parsed < 0 {
		return 0, errors.Errorf("%s must be >= 0: %q", key, v)
	}
	return parsed, nil
}

func queryOptionalInt32(r *http.Request, key string) (int32, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, errors.Errorf("%s must be an int32: %q", key, v)
	}
	if parsed < 0 {
		return 0, errors.Errorf("%s must be >= 0: %q", key, v)
	}
	return int32(parsed), nil
}

func queryOptionalInt64(r *http.Request, key string) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, errors.Errorf("%s must be an int64: %q", key, v)
	}
	if parsed < 0 {
		return 0, errors.Errorf("%s must be >= 0: %q", key, v)
	}
	return parsed, nil
}
