/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// newTestService builds a minimal Service wired to the given querier and cfg.
// Passing nil for querier is safe — tests that don't hit the DB can omit it.
func newTestService(t *testing.T, querier dbsqlc.Querier, restCfg config.RESTConfig) *Service {
	t.Helper()
	svc := New(&config.Config{Server: config.ServerConfig{REST: restCfg}})
	if querier != nil {
		svc.querier = querier
	}
	return svc
}

func doRequest(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func doRequestWithOrigin(
	t *testing.T, handler http.Handler, method, origin string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ── /healthz ─────────────────────────────────────────────────────────────────

func TestHandleHealthz(t *testing.T) {
	t.Parallel()
	svc := newTestService(t, nil, config.RESTConfig{})
	handler := svc.newRESTRouter()

	rr := doRequest(t, handler, "/healthz")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "fabric-x-block-explorer", body["service"])
}

// ── CORS middleware ───────────────────────────────────────────────────────────

func TestCORSMiddleware(t *testing.T) {
	t.Parallel()

	// Trivial inner handler that just returns 200.
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("wildcard_default_allows_any_origin", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{"*"})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodGet, "https://random.example.com")
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("empty_list_treated_as_wildcard", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodGet, "https://random.example.com")
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("origin_in_allowlist_is_reflected", func(t *testing.T) {
		t.Parallel()
		allowed := "https://explorer.example.com"
		h := corsMiddleware([]string{allowed})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodGet, allowed)
		assert.Equal(t, allowed, rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "Origin", rr.Header().Get("Vary"))
	})

	t.Run("origin_not_in_allowlist_is_not_reflected", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{"https://explorer.example.com"})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodGet, "https://evil.example.com")
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("no_origin_header_no_cors_header", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{"https://explorer.example.com"})(inner)
		rr := doRequest(t, h, "/")
		// Non-open policy: no origin header → no ACAO header.
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("preflight_OPTIONS_returns_204", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{"*"})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodOptions, "https://any.example.com")
		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("preflight_OPTIONS_restricted_allowed_origin", func(t *testing.T) {
		t.Parallel()
		allowed := "https://ui.example.com"
		h := corsMiddleware([]string{allowed})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodOptions, allowed)
		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, allowed, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("preflight_OPTIONS_restricted_unknown_origin", func(t *testing.T) {
		t.Parallel()
		h := corsMiddleware([]string{"https://ui.example.com"})(inner)
		rr := doRequestWithOrigin(t, h, http.MethodOptions, "https://evil.example.com")
		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})
}

// ── stubQuerier ───────────────────────────────────────────────────────────────

// stubQuerier is a hand-written stub that implements the subset of dbsqlc.Querier
// used by the handlers under test. Methods not overridden panic on call.
type stubQuerier struct {
	dbsqlc.Querier
	blockHeight     any
	blockHeightErr  error
	listBlocksRows  []dbsqlc.Block
	listBlocksErr   error
	getBlockRow     dbsqlc.Block
	getBlockErr     error
	getTxRow        dbsqlc.Transaction
	getTxErr        error
	allPoliciesRows []dbsqlc.NamespacePolicy
	allPoliciesErr  error
	nsPoliciesRows  []dbsqlc.NamespacePolicy
	nsPoliciesErr   error
}

func (s stubQuerier) GetBlockHeight(_ context.Context) (any, error) {
	return s.blockHeight, s.blockHeightErr
}

func (s stubQuerier) ListBlocks(
	_ context.Context, _ dbsqlc.ListBlocksParams,
) ([]dbsqlc.Block, error) {
	return s.listBlocksRows, s.listBlocksErr
}

func (s stubQuerier) GetBlock(_ context.Context, _ int64) (dbsqlc.Block, error) {
	return s.getBlockRow, s.getBlockErr
}

func (stubQuerier) GetValidationCodeByBlock(
	_ context.Context, _ dbsqlc.GetValidationCodeByBlockParams,
) ([]dbsqlc.Transaction, error) {
	return nil, nil
}

func (stubQuerier) GetEnvelopeErrorsByBlock(
	_ context.Context, _ int64,
) ([]dbsqlc.BlockEnvelopeError, error) {
	return nil, nil
}

func (s stubQuerier) GetValidationCodeByTxID(_ context.Context, _ []byte) (dbsqlc.Transaction, error) {
	return s.getTxRow, s.getTxErr
}

func (stubQuerier) GetNamespacesByTx(
	_ context.Context, _ dbsqlc.GetNamespacesByTxParams,
) ([]dbsqlc.GetNamespacesByTxRow, error) {
	return nil, nil
}

func (stubQuerier) GetBlindWritesByTx(
	_ context.Context, _ dbsqlc.GetBlindWritesByTxParams,
) ([]dbsqlc.GetBlindWritesByTxRow, error) {
	return nil, nil
}

func (stubQuerier) GetEndorsementsByTx(
	_ context.Context, _ dbsqlc.GetEndorsementsByTxParams,
) ([]dbsqlc.GetEndorsementsByTxRow, error) {
	return nil, nil
}

func (stubQuerier) GetReadWritesByTx(
	_ context.Context, _ dbsqlc.GetReadWritesByTxParams,
) ([]dbsqlc.GetReadWritesByTxRow, error) {
	return nil, nil
}

func (stubQuerier) GetReadsOnlyByTx(
	_ context.Context, _ dbsqlc.GetReadsOnlyByTxParams,
) ([]dbsqlc.GetReadsOnlyByTxRow, error) {
	return nil, nil
}

func (s stubQuerier) ListAllNamespacePolicies(_ context.Context) ([]dbsqlc.NamespacePolicy, error) {
	return s.allPoliciesRows, s.allPoliciesErr
}

func (s stubQuerier) GetNamespacePolicies(
	_ context.Context, _ string,
) ([]dbsqlc.NamespacePolicy, error) {
	return s.nsPoliciesRows, s.nsPoliciesErr
}

// ── /blocks/height ────────────────────────────────────────────────────────────

func TestHandleGetBlockHeight(t *testing.T) {
	t.Parallel()

	t.Run("returns_height_200", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{blockHeight: int64(42)}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/height")
		require.Equal(t, http.StatusOK, rr.Code)
		var body BlockHeightResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, int64(42), body.Height)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{blockHeightErr: errors.New("db down")}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/height")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("context_cancelled_returns_499", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{blockHeightErr: context.Canceled}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/height")
		assert.Equal(t, 499, rr.Code)
	})
}

// ── /blocks ───────────────────────────────────────────────────────────────────

func TestHandleListBlocks_HappyPath(t *testing.T) {
	t.Parallel()

	rows := []dbsqlc.Block{
		{BlockNum: 0, TxCount: 1},
		{BlockNum: 1, TxCount: 2},
	}
	svc := newTestService(t, stubQuerier{
		listBlocksRows: rows,
		blockHeight:    int64(2),
	}, config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50})
	handler := svc.newRESTRouter()

	t.Run("returns_200_with_blocks_slice", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks")
		require.Equal(t, http.StatusOK, rr.Code)
		var body ListBlocksResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Len(t, body.Blocks, 2)
		assert.Equal(t, int64(0), body.Blocks[0].BlockNum)
		assert.Equal(t, int64(1), body.Blocks[1].BlockNum)
	})

	t.Run("empty_db_returns_200_empty_slice", func(t *testing.T) {
		t.Parallel()
		svc2 := newTestService(t,
			stubQuerier{listBlocksRows: []dbsqlc.Block{}},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc2.newRESTRouter(), "/blocks")
		require.Equal(t, http.StatusOK, rr.Code)
		var body ListBlocksResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Empty(t, body.Blocks)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc3 := newTestService(t,
			stubQuerier{listBlocksErr: errors.New("db error")},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc3.newRESTRouter(), "/blocks")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("context_cancelled_returns_499", func(t *testing.T) {
		t.Parallel()
		svc4 := newTestService(t,
			stubQuerier{listBlocksErr: context.Canceled},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc4.newRESTRouter(), "/blocks")
		assert.Equal(t, 499, rr.Code)
	})
}

// ── request size cap ─────────────────────────────────────────────────────────

func TestHandleListBlocks_LimitCap(t *testing.T) {
	t.Parallel()

	svc := newTestService(t, stubQuerier{listBlocksRows: []dbsqlc.Block{}}, config.RESTConfig{
		MaxListLimit:   100,
		DefaultTxLimit: 50,
	})
	handler := svc.newRESTRouter()

	t.Run("limit_over_max_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=101")
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var body ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Contains(t, body.Error, "limit must be <=")
	})

	t.Run("limit_at_max_returns_200", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=100")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("limit_zero_uses_default_returns_200", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=0")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("limit_negative_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("limit_non_integer_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=abc")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("from_gt_to_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?from=10&to=5")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// ── default max limit (no explicit config) ───────────────────────────────────

func TestHandleListBlocks_DefaultMaxLimit(t *testing.T) {
	t.Parallel()
	// Zero MaxListLimit falls back to DefaultMaxListLimit (500).
	svc := newTestService(t, stubQuerier{listBlocksRows: []dbsqlc.Block{}}, config.RESTConfig{})
	handler := svc.newRESTRouter()

	t.Run("limit_501_over_default_max_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=501")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("limit_500_at_default_max_returns_200", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?limit=500")
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// ── handleListBlocks additional validation ────────────────────────────────────

func TestHandleListBlocks_ParamValidation(t *testing.T) {
	t.Parallel()

	svc := newTestService(t,
		stubQuerier{listBlocksRows: []dbsqlc.Block{}},
		config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
	)
	handler := svc.newRESTRouter()

	t.Run("negative_from_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?from=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("negative_to_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?to=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("negative_offset_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks?offset=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// ── /blocks/{block_num} ───────────────────────────────────────────────────────

func TestHandleGetBlockByNumber(t *testing.T) {
	t.Parallel()

	t.Run("found_returns_200", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{getBlockRow: dbsqlc.Block{BlockNum: 1, TxCount: 0}},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/1")
		require.Equal(t, http.StatusOK, rr.Code)
		var body Block
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, int64(1), body.BlockNum)
	})

	t.Run("not_found_returns_404", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{getBlockErr: pgx.ErrNoRows},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/999")
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{getBlockErr: errors.New("db error")},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/1")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("context_cancelled_returns_499", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{getBlockErr: context.Canceled},
			config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/blocks/1")
		assert.Equal(t, 499, rr.Code)
	})
}

func TestHandleGetBlockByNumber_TxLimitCap(t *testing.T) {
	t.Parallel()

	svc := newTestService(t, stubQuerier{getBlockErr: pgx.ErrNoRows}, config.RESTConfig{
		MaxListLimit:   100,
		DefaultTxLimit: 50,
	})
	handler := svc.newRESTRouter()

	t.Run("tx_limit_over_max_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks/1?tx_limit=101")
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var body ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Contains(t, body.Error, "tx_limit must be <=")
	})

	t.Run("tx_limit_at_max_hits_404_not_400", func(t *testing.T) {
		t.Parallel()
		// stubQuerier returns ErrNoRows for GetBlock — expect 404, meaning limit
		// validation passed and we reached the DB lookup.
		rr := doRequest(t, handler, "/blocks/1?tx_limit=100")
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("negative_block_num_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks/-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("non_integer_block_num_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks/abc")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// ── handleGetBlockByNumber additional validation ──────────────────────────────

func TestHandleGetBlockByNumber_ParamValidation(t *testing.T) {
	t.Parallel()

	svc := newTestService(t,
		stubQuerier{getBlockErr: pgx.ErrNoRows},
		config.RESTConfig{MaxListLimit: 100, DefaultTxLimit: 50},
	)
	handler := svc.newRESTRouter()

	t.Run("negative_tx_offset_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks/1?tx_offset=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("negative_tx_limit_returns_400", func(t *testing.T) {
		t.Parallel()
		rr := doRequest(t, handler, "/blocks/1?tx_limit=-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// ── /transactions/{tx_id} ────────────────────────────────────────────────────

func TestHandleGetTxByID(t *testing.T) {
	t.Parallel()

	validTxIDHex := "deadbeefdeadbeef"

	t.Run("not_found_returns_404", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{getTxErr: pgx.ErrNoRows}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/transactions/"+validTxIDHex)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("non_hex_tx_id_returns_400", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/transactions/not-valid-hex!!")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{getTxErr: errors.New("db error")}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/transactions/"+validTxIDHex)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("context_cancelled_returns_499", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{getTxErr: context.Canceled}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/transactions/"+validTxIDHex)
		assert.Equal(t, 499, rr.Code)
	})
}

// ── /namespaces/policies ──────────────────────────────────────────────────────

func TestHandleListAllNamespacePolicies(t *testing.T) {
	t.Parallel()

	t.Run("returns_policies_200", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{
			allPoliciesRows: []dbsqlc.NamespacePolicy{
				{Namespace: "ns1", Version: 1},
			},
		}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/namespaces/policies")
		require.Equal(t, http.StatusOK, rr.Code)
		var body NamespacePoliciesResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Len(t, body.Policies, 1)
		assert.Equal(t, "ns1", body.Policies[0].Namespace)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{allPoliciesErr: errors.New("db error")},
			config.RESTConfig{},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/namespaces/policies")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// ── /namespaces/{namespace}/policies ─────────────────────────────────────────

func TestHandleGetNamespacePolicies(t *testing.T) {
	t.Parallel()

	t.Run("found_returns_200", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{
			nsPoliciesRows: []dbsqlc.NamespacePolicy{
				{Namespace: "myns", Version: 2},
			},
		}, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/namespaces/myns/policies")
		require.Equal(t, http.StatusOK, rr.Code)
		var body NamespacePoliciesResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Len(t, body.Policies, 1)
		assert.Equal(t, "myns", body.Policies[0].Namespace)
	})

	t.Run("empty_result_returns_200_empty_slice", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{nsPoliciesRows: []dbsqlc.NamespacePolicy{}},
			config.RESTConfig{},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/namespaces/myns/policies")
		require.Equal(t, http.StatusOK, rr.Code)
		var body NamespacePoliciesResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Empty(t, body.Policies)
	})

	t.Run("db_error_returns_500", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t,
			stubQuerier{nsPoliciesErr: errors.New("db error")},
			config.RESTConfig{},
		)
		rr := doRequest(t, svc.newRESTRouter(), "/namespaces/myns/policies")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// ── respondError ─────────────────────────────────────────────────────────────

func TestRespondError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		wantCode int
		wantBody string
	}{
		{"not_found", pgx.ErrNoRows, http.StatusNotFound, "not found"},
		{"context_cancelled", context.Canceled, 499, "request cancelled"},
		{"context_deadline", context.DeadlineExceeded, 499, "request cancelled"},
		{"validation_error", newValidationError("bad param"), http.StatusBadRequest, "bad param"},
		{"internal_error", errors.New("mystery"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			respondError(rr, tc.err)
			assert.Equal(t, tc.wantCode, rr.Code)
			var body ErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
			assert.Contains(t, body.Error, tc.wantBody)
		})
	}
}

// ── corsOrigins fallback ──────────────────────────────────────────────────────

func TestCorsOrigins(t *testing.T) {
	t.Parallel()

	t.Run("nil_config_returns_wildcard", func(t *testing.T) {
		t.Parallel()
		var s *Service
		assert.Equal(t, []string{"*"}, s.corsOrigins())
	})

	t.Run("nil_service_config_field_returns_wildcard", func(t *testing.T) {
		t.Parallel()
		s := &Service{config: nil}
		assert.Equal(t, []string{"*"}, s.corsOrigins())
	})

	t.Run("empty_allowed_origins_returns_wildcard", func(t *testing.T) {
		t.Parallel()
		s := &Service{config: &config.Config{
			Server: config.ServerConfig{REST: config.RESTConfig{CORSAllowedOrigins: []string{}}},
		}}
		assert.Equal(t, []string{"*"}, s.corsOrigins())
	})

	t.Run("configured_origins_are_returned", func(t *testing.T) {
		t.Parallel()
		origins := []string{"https://ui.example.com"}
		s := &Service{config: &config.Config{
			Server: config.ServerConfig{REST: config.RESTConfig{CORSAllowedOrigins: origins}},
		}}
		assert.Equal(t, origins, s.corsOrigins())
	})
}

// ── originAllowed ─────────────────────────────────────────────────────────────

func TestOriginAllowed(t *testing.T) {
	t.Parallel()

	t.Run("exact_match_allowed", func(t *testing.T) {
		t.Parallel()
		assert.True(t, originAllowed("https://ui.example.com", []string{"https://ui.example.com"}))
	})

	t.Run("no_match_not_allowed", func(t *testing.T) {
		t.Parallel()
		assert.False(t, originAllowed("https://evil.com", []string{"https://ui.example.com"}))
	})

	t.Run("empty_list_not_allowed", func(t *testing.T) {
		t.Parallel()
		assert.False(t, originAllowed("https://ui.example.com", []string{}))
	})

	t.Run("multiple_origins_second_matches", func(t *testing.T) {
		t.Parallel()
		assert.True(t, originAllowed("https://b.example.com",
			[]string{"https://a.example.com", "https://b.example.com"}))
	})
}
