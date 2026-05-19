/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-common/api/committerpb"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
)

var restLogger = flogging.MustGetLogger("api.rest")

// errValidationMark is the sentinel used to classify errors as client-side
// validation failures (HTTP 400). Create validation errors with newValidationError.
var errValidationMark = errors.New("validation error")

// newValidationError returns a 400-class error marked with errValidationMark.
func newValidationError(format string, args ...any) error {
	return errors.Mark(errors.Newf(format, args...), errValidationMark)
}

// newRESTRouter registers all REST endpoints.
//
//	GET /blocks/height
//	GET /blocks                      ?from=&to=&limit=&offset=
//	GET /blocks/{block_num}          ?tx_limit=&tx_offset=
//	GET /transactions/{tx_id}        (tx_id is hex)
//	GET /namespaces/policies         (list all namespaces)
//	GET /namespaces/{namespace}/policies
func (s *Service) newRESTRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blocks/height", s.handleGetBlockHeight)
	mux.HandleFunc("GET /blocks", s.handleListBlocks)
	mux.HandleFunc("GET /blocks/{block_num}", s.handleGetBlockByNumber)
	mux.HandleFunc("GET /transactions/{tx_id}", s.handleGetTxByID)
	mux.HandleFunc("GET /namespaces/policies", s.handleListAllNamespacePolicies)
	mux.HandleFunc("GET /namespaces/{namespace}/policies", s.handleGetNamespacePolicies)
	mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPISpec)
	mux.HandleFunc("GET /docs", handleSwaggerUI)
	mux.HandleFunc("OPTIONS /", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return corsMiddleware(loggingMiddleware(mux))
}

// loggingMiddleware logs each HTTP request with method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		restLogger.Infof("%s %s %d %s", r.Method, r.URL.RequestURI(), lrw.status, duration)
	})
}

// corsMiddleware adds permissive CORS headers so the REST API and Swagger UI
// can be used from any browser origin (e.g. VS Code Simple Browser, localhost
// dev tools, or a separate front-end dev server).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingResponseWriter captures the status code written by a handler.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Unwrap allows http.ResponseController and interface-type-assertions (e.g. Flusher,
// Hijacker) to reach the underlying ResponseWriter through the middleware wrapper.
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}

func (s *Service) handleGetBlockHeight(w http.ResponseWriter, r *http.Request) {
	heightResult, err := s.querier.GetBlockHeight(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}

	// GetBlockHeight uses COALESCE(MAX(block_num), 0) so the result is always a
	// non-nil int64. The type assertion is guarded to avoid a panic on any future
	// driver-level change.
	height, ok := heightResult.(int64)
	if !ok {
		respondError(w, errors.Errorf("unexpected block height type: %T", heightResult))
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
		respondError(w, newValidationError("from must be >= 0"))
		return
	}
	if to < 0 {
		respondError(w, newValidationError("to must be >= 0"))
		return
	}
	if to != 0 && to < from {
		respondError(w, newValidationError("to must be >= from"))
		return
	}
	if limit < 0 {
		respondError(w, newValidationError("limit must be >= 0"))
		return
	}
	if offset < 0 {
		respondError(w, newValidationError("offset must be >= 0"))
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
			BlockHeader: BlockHeader{
				BlockNum:           row.BlockNum,
				TxCount:            row.TxCount,
				PreviousHash:       row.PreviousHash,
				DataHash:           row.DataHash,
				BlockSize:          util.NullableInt4ToInt32Ptr(row.BlockSize),
				CreatedAt:          util.NullableTimestampToTimePtr(row.CreatedAt),
				MetadataSignatures: row.MetadataSignatures,
				LastConfigIndex:    util.NullableInt64ToPtr(row.LastConfigIndex),
				TxStatusCodes:      decodeTxStatusCodes(row.TxStatusCodes),
				CommitHash:         row.CommitHash,
			},
		}
	}

	respondJSON(w, ListBlocksResponse{
		Blocks:  blocks,
		Offset:  offset,
		Limit:   limit,
		HasMore: int32(len(rows)) == limit, //nolint:gosec // len fits in int32 for reasonable limits
	})
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
		respondError(w, newValidationError("block_num must be >= 0"))
		return
	}
	if txLimit < 0 {
		respondError(w, newValidationError("tx_limit must be >= 0"))
		return
	}
	if txOffset < 0 {
		respondError(w, newValidationError("tx_offset must be >= 0"))
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

	eeRows, err := s.querier.GetEnvelopeErrorsByBlock(r.Context(), blockNum)
	if err != nil {
		respondError(w, err)
		return
	}
	envelopeErrors := make([]EnvelopeError, len(eeRows))
	for i, row := range eeRows {
		envelopeErrors[i] = EnvelopeError{
			TxNum:          row.TxNum,
			ValidationCode: row.ValidationCode,
			RawEnvelope:    row.RawEnvelope,
			TxID:           hex.EncodeToString(row.TxID),
		}
	}

	respondJSON(w, Block{
		BlockHeader: BlockHeader{
			BlockNum:           blockRow.BlockNum,
			TxCount:            blockRow.TxCount,
			PreviousHash:       blockRow.PreviousHash,
			DataHash:           blockRow.DataHash,
			BlockSize:          util.NullableInt4ToInt32Ptr(blockRow.BlockSize),
			CreatedAt:          util.NullableTimestampToTimePtr(blockRow.CreatedAt),
			MetadataSignatures: blockRow.MetadataSignatures,
			LastConfigIndex:    util.NullableInt64ToPtr(blockRow.LastConfigIndex),
			TxStatusCodes:      decodeTxStatusCodes(blockRow.TxStatusCodes),
			CommitHash:         blockRow.CommitHash,
		},
		Transactions:   txDetails,
		EnvelopeErrors: envelopeErrors,
	})
}

func (s *Service) handleGetTxByID(w http.ResponseWriter, r *http.Request) {
	txIDHex := r.PathValue("tx_id")
	if txIDHex == "" {
		respondError(w, newValidationError("tx_id is required"))
		return
	}

	txID, err := hex.DecodeString(txIDHex)
	if err != nil {
		respondError(w, newValidationError("tx_id must be hex-encoded"))
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

func (s *Service) handleListAllNamespacePolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.querier.ListAllNamespacePolicies(r.Context())
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

func (s *Service) handleGetNamespacePolicies(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	if namespace == "" {
		respondError(w, newValidationError("namespace is required"))
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
		BlockNum:          tx.BlockNum,
		TxNum:             tx.TxNum,
		TxID:              hex.EncodeToString(tx.TxID),
		ValidationCode:    tx.ValidationCode,
		TxType:            util.NullableStringToPtr(tx.TxType),
		ChaincodeName:     util.NullableStringToPtr(tx.ChaincodeName),
		CreatorMspID:      util.NullableStringToPtr(tx.CreatorMspID),
		CreatorIdentity:   decodeCreatorIdentity(tx.CreatorIDBytes),
		CreatorNonce:      tx.CreatorNonce,
		EnvelopeSignature: tx.EnvelopeSignature,
		PayloadExtension:  decodePayloadExtension(tx.PayloadExtension),
		ChannelVersion:    util.NullableInt4ToInt32Ptr(tx.ChannelVersion),
		ChannelID:         util.NullableStringToPtr(tx.ChannelID),
		Epoch:             util.NullableInt64ToPtr(tx.Epoch),
		TLSCertHash:       tx.TlsCertHash,
		CreatedAt:         util.NullableTimestampToTimePtr(tx.CreatedAt),
	}

	namespaces, err := s.fetchNamespaces(ctx, tx.BlockNum, tx.TxNum)
	if err != nil {
		return Transaction{}, err
	}
	detail.Namespaces = namespaces

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
			BlockNum:          tx.BlockNum,
			TxNum:             tx.TxNum,
			TxID:              hex.EncodeToString(tx.TxID),
			ValidationCode:    tx.ValidationCode,
			TxType:            util.NullableStringToPtr(tx.TxType),
			ChaincodeName:     util.NullableStringToPtr(tx.ChaincodeName),
			CreatorMspID:      util.NullableStringToPtr(tx.CreatorMspID),
			CreatorIdentity:   decodeCreatorIdentity(tx.CreatorIDBytes),
			CreatorNonce:      tx.CreatorNonce,
			EnvelopeSignature: tx.EnvelopeSignature,
			PayloadExtension:  decodePayloadExtension(tx.PayloadExtension),
			ChannelVersion:    util.NullableInt4ToInt32Ptr(tx.ChannelVersion),
			ChannelID:         util.NullableStringToPtr(tx.ChannelID),
			Epoch:             util.NullableInt64ToPtr(tx.Epoch),
			TLSCertHash:       tx.TlsCertHash,
			CreatedAt:         util.NullableTimestampToTimePtr(tx.CreatedAt),
			Namespaces:        datasets.namespaces[tx.TxNum],
			BlindWrites:       datasets.blindWrites[tx.TxNum],
			Endorsements:      datasets.endorsements[tx.TxNum],
			ReadWrites:        datasets.readWrites[tx.TxNum],
			ReadsOnly:         datasets.readsOnly[tx.TxNum],
		}
	}

	return details, nil
}

type blockTxDatasets struct {
	namespaces   map[int64][]NamespaceRow
	blindWrites  map[int64][]BlindWriteRow
	endorsements map[int64][]EndorsementRow
	readWrites   map[int64][]ReadWriteRow
	readsOnly    map[int64][]ReadOnlyRow
}

func (s *Service) fetchBlockTxDatasets(ctx context.Context, txRows []dbsqlc.Transaction) (*blockTxDatasets, error) {
	blockNum := txRows[0].BlockNum
	startTxNum := txRows[0].TxNum
	endTxNum := txRows[len(txRows)-1].TxNum + 1

	namespacesRows, err := s.querier.GetNamespacesByBlockTxRange(ctx, dbsqlc.GetNamespacesByBlockTxRangeParams{
		BlockNum: blockNum,
		TxNum:    startTxNum,
		TxNum_2:  endTxNum,
	})
	if err != nil {
		return nil, err
	}

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
		namespaces:   make(map[int64][]NamespaceRow, len(txRows)),
		blindWrites:  make(map[int64][]BlindWriteRow, len(txRows)),
		endorsements: make(map[int64][]EndorsementRow, len(txRows)),
		readWrites:   make(map[int64][]ReadWriteRow, len(txRows)),
		readsOnly:    make(map[int64][]ReadOnlyRow, len(txRows)),
	}

	for _, tx := range txRows {
		datasets.namespaces[tx.TxNum] = []NamespaceRow{}
		datasets.blindWrites[tx.TxNum] = []BlindWriteRow{}
		datasets.endorsements[tx.TxNum] = []EndorsementRow{}
		datasets.readWrites[tx.TxNum] = []ReadWriteRow{}
		datasets.readsOnly[tx.TxNum] = []ReadOnlyRow{}
	}

	for _, row := range namespacesRows {
		datasets.namespaces[row.TxNum] = append(datasets.namespaces[row.TxNum], NamespaceRow{
			NsID:      row.NsID,
			NsVersion: row.NsVersion,
		})
	}

	for _, row := range blindWritesRows {
		datasets.blindWrites[row.TxNum] = append(datasets.blindWrites[row.TxNum], BlindWriteRow{
			NsID:   row.NsID,
			SeqNum: row.SeqNum,
			Key:    row.Key,
			Value:  row.Value,
		})
	}

	for _, row := range endorsementRows {
		datasets.endorsements[row.TxNum] = append(datasets.endorsements[row.TxNum], EndorsementRow{
			NsID:        row.NsID,
			SeqNum:      row.SeqNum,
			Endorsement: row.Endorsement,
			MspID:       util.NullableStringToPtr(row.MspID),
			Identity:    row.Identity,
		})
	}

	for _, row := range readWriteRows {
		datasets.readWrites[row.TxNum] = append(datasets.readWrites[row.TxNum], ReadWriteRow{
			NsID:        row.NsID,
			SeqNum:      row.SeqNum,
			Key:         row.Key,
			ReadVersion: util.NullableInt64ToPtr(row.ReadVersion),
			Value:       row.Value,
		})
	}

	for _, row := range readsOnlyRows {
		datasets.readsOnly[row.TxNum] = append(datasets.readsOnly[row.TxNum], ReadOnlyRow{
			NsID:    row.NsID,
			SeqNum:  row.SeqNum,
			Key:     row.Key,
			Version: util.NullableInt64ToPtr(row.Version),
		})
	}

	return datasets, nil
}

func (s *Service) fetchNamespaces(ctx context.Context, blockNum, txNum int64) ([]NamespaceRow, error) {
	rows, err := s.querier.GetNamespacesByTx(ctx, dbsqlc.GetNamespacesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]NamespaceRow, len(rows))
	for i, row := range rows {
		result[i] = NamespaceRow{NsID: row.NsID, NsVersion: row.NsVersion}
	}
	return result, nil
}

func (s *Service) fetchBlindWrites(ctx context.Context, blockNum, txNum int64) ([]BlindWriteRow, error) {
	rows, err := s.querier.GetBlindWritesByTx(ctx, dbsqlc.GetBlindWritesByTxParams{BlockNum: blockNum, TxNum: txNum})
	if err != nil {
		return nil, err
	}
	result := make([]BlindWriteRow, len(rows))
	for i, row := range rows {
		result[i] = BlindWriteRow{NsID: row.NsID, SeqNum: row.SeqNum, Key: row.Key, Value: row.Value}
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
			SeqNum:      row.SeqNum,
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
			SeqNum:      row.SeqNum,
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
		result[i] = ReadOnlyRow{
			NsID:    row.NsID,
			SeqNum:  row.SeqNum,
			Key:     row.Key,
			Version: util.NullableInt64ToPtr(row.Version),
		}
	}
	return result, nil
}

func (s *Service) defaultTxLimit() int32 {
	if s != nil && s.config != nil && s.config.Server.REST.DefaultTxLimit > 0 {
		return s.config.Server.REST.DefaultTxLimit
	}
	return config.DefaultTxLimit
}

// --- HTTP helpers ---

func respondJSON(w http.ResponseWriter, data any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		restLogger.Errorf("failed to encode JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "internal server error"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = buf.WriteTo(w)
}

func respondError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "not found"})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		w.WriteHeader(499)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "request cancelled"})
	case isValidationError(err):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
	default:
		restLogger.Errorf("internal server error: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "internal server error"})
	}
}

func isValidationError(err error) bool {
	return errors.Is(err, errValidationMark)
}

// --- Decode helpers ---

// decodeTxStatusCodes converts raw BlockMetadata[TRANSACTIONS_FILTER] bytes into
// a slice of human-readable committer status strings (one per block envelope position).
func decodeTxStatusCodes(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	codes := make([]string, len(raw))
	for i, b := range raw {
		codes[i] = committerpb.Status(b).String()
	}
	return codes
}

// decodeCreatorIdentity decodes the serialised msp.SerializedIdentity proto stored
// in creator_id_bytes into a human-readable CreatorIdentity. Returns nil on failure.
func decodeCreatorIdentity(raw []byte) *CreatorIdentity {
	if len(raw) == 0 {
		return nil
	}
	id := &msp.SerializedIdentity{}
	if err := proto.Unmarshal(raw, id); err != nil {
		return nil
	}
	return &CreatorIdentity{
		MspID:          id.Mspid,
		CertificatePEM: string(id.IdBytes),
	}
}

// decodePayloadExtension decodes the serialised peer.ChaincodeHeaderExtension proto
// stored in payload_extension. Returns nil for non-chaincode txs or on decode failure.
func decodePayloadExtension(raw []byte) *PayloadExtension {
	if len(raw) == 0 {
		return nil
	}
	ext := &peer.ChaincodeHeaderExtension{}
	if err := proto.Unmarshal(raw, ext); err != nil || ext.ChaincodeId == nil {
		return nil
	}
	return &PayloadExtension{
		ChaincodeID: &ChaincodeID{
			Name:    ext.ChaincodeId.Name,
			Path:    ext.ChaincodeId.Path,
			Version: ext.ChaincodeId.Version,
		},
	}
}

func pathInt64(r *http.Request, key string) (int64, error) {
	v := r.PathValue(key)
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, newValidationError("%s must be an integer: %q", key, v)
	}
	if parsed < 0 {
		return 0, newValidationError("%s must be >= 0: %q", key, v)
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
		return 0, newValidationError("%s must be an int32: %q", key, v)
	}
	if parsed < 0 {
		return 0, newValidationError("%s must be >= 0: %q", key, v)
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
		return 0, newValidationError("%s must be an int64: %q", key, v)
	}
	if parsed < 0 {
		return 0, newValidationError("%s must be >= 0: %q", key, v)
	}
	return parsed, nil
}
