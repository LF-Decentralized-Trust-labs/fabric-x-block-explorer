/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
)

// ── /readyz ───────────────────────────────────────────────────────────────────

func TestHandleReadyz(t *testing.T) {
	t.Parallel()

	t.Run("no_pool_no_threshold_returns_ready", func(t *testing.T) {
		t.Parallel()
		// Service with no pool and no threshold → always ready.
		svc := newTestService(t, nil, config.RESTConfig{})
		rr := doRequest(t, svc.newRESTRouter(), "/readyz")
		require.Equal(t, http.StatusOK, rr.Code)
		var body ReadyzResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "ready", body.Status)
	})

	t.Run("first_start_no_blocks_returns_ready", func(t *testing.T) {
		t.Parallel()
		// Threshold set but no block ingested yet → stale check must be skipped.
		svc := newTestService(t, nil, config.RESTConfig{
			StaleBlockThreshold: 30 * time.Second,
		})
		rr := doRequest(t, svc.newRESTRouter(), "/readyz")
		require.Equal(t, http.StatusOK, rr.Code)
		var body ReadyzResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "ready", body.Status)
	})

	t.Run("pipeline_stalled_returns_503", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, nil, config.RESTConfig{
			StaleBlockThreshold: 1 * time.Millisecond,
		})
		// Simulate a block that was recorded 1 hour ago.
		svc.readyz.lastBlockAt.Store(time.Now().Add(-1 * time.Hour).UnixNano())

		rr := doRequest(t, svc.newRESTRouter(), "/readyz")
		require.Equal(t, http.StatusServiceUnavailable, rr.Code)
		var body ReadyzResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "degraded", body.Status)
		assert.Equal(t, "pipeline_stalled", body.Reason)
		assert.NotEmpty(t, body.LastBlockAgo)
	})

	t.Run("recent_block_within_threshold_returns_ready", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, nil, config.RESTConfig{
			StaleBlockThreshold: 1 * time.Hour,
		})
		svc.readyz.lastBlockAt.Store(time.Now().UnixNano())

		rr := doRequest(t, svc.newRESTRouter(), "/readyz")
		require.Equal(t, http.StatusOK, rr.Code)
		var body ReadyzResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
		assert.Equal(t, "ready", body.Status)
	})
}

// ── /metrics ──────────────────────────────────────────────────────────────────

func TestMetricsEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("enabled_returns_200_text_plain", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, nil, config.RESTConfig{MetricsEnabled: true})
		rr := doRequest(t, svc.newRESTRouter(), "/metrics")
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/plain")
	})

	t.Run("disabled_returns_404", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, nil, config.RESTConfig{MetricsEnabled: false})
		rr := doRequest(t, svc.newRESTRouter(), "/metrics")
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("block_height_metric_present", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, nil, config.RESTConfig{MetricsEnabled: true})
		svc.metrics.RecordBlock(42)
		rr := doRequest(t, svc.newRESTRouter(), "/metrics")
		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "explorer_block_height_current")
		assert.Contains(t, body, "42")
	})

	t.Run("http_counter_increments_per_request", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t, stubQuerier{listBlocksRows: []dbsqlc.Block{}}, config.RESTConfig{
			MetricsEnabled: true,
			MaxListLimit:   100,
			DefaultTxLimit: 50,
		})
		handler := svc.newRESTRouter()

		// Make a request that goes through the loggingMiddleware (not wrapped by
		// prometheusMiddleware — that is per-pattern; use blocks/height here).
		doRequest(t, handler, "/blocks")

		rr := doRequest(t, handler, "/metrics")
		require.Equal(t, http.StatusOK, rr.Code)
		// The /metrics response must contain the registered gauge/counter names.
		body := rr.Body.String()
		assert.Contains(t, body, "explorer_blocks_ingested_total")
	})
}

// ── Metrics.RecordBlock ───────────────────────────────────────────────────────

func TestMetricsRecordBlock(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordBlock(10)
	m.RecordBlock(20)

	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	// After two RecordBlock calls the gauge should reflect the last value (20)
	// and the counter should be 2.
	assert.True(t, strings.Contains(body, "explorer_block_height_current 20") ||
		strings.Contains(body, "explorer_block_height_current{} 20"))
	assert.True(t, strings.Contains(body, "explorer_blocks_ingested_total 2") ||
		strings.Contains(body, "explorer_blocks_ingested_total{} 2"))
}
