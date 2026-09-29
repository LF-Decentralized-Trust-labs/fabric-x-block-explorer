/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

// readyzHandler checks DB liveness and pipeline freshness.
// It is stored on Service so it can access the pool and lastBlockAt.
type readyzState struct {
	// lastBlockAt is the Unix nanosecond timestamp of the last block written by
	// the pipeline. Zero means no block has been ingested since startup.
	lastBlockAt atomic.Int64
}

// RecordBlock is called by the block writer after each successful DB write.
func (r *readyzState) RecordBlock() {
	r.lastBlockAt.Store(time.Now().UnixNano())
}

// ReadyzResponse is the JSON body for /readyz.
type ReadyzResponse struct {
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
	LastBlockAgo string `json:"last_block_ago,omitempty"`
}

// handleReadyz implements GET /readyz.
//
// Returns:
//   - 200  {"status":"ready"}                  — DB reachable, pipeline fresh (or no threshold set)
//   - 503  {"status":"unavailable","reason":"db_unreachable"}
//   - 503  {"status":"degraded","reason":"pipeline_stalled","last_block_ago":"..."}
func (s *Service) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. DB ping (3 s timeout so the probe itself never hangs)
	if s.pool != nil {
		pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.pool.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(ReadyzResponse{Status: "unavailable", Reason: "db_unreachable"})
			return
		}
	}

	// 2. Pipeline freshness check (only when a threshold is configured AND at
	// least one block has been ingested — avoids false positives at startup).
	threshold := s.staleThreshold()
	if threshold > 0 && s.readyz != nil {
		lastNs := s.readyz.lastBlockAt.Load()
		if lastNs > 0 {
			ago := time.Since(time.Unix(0, lastNs))
			if ago > threshold {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(ReadyzResponse{
					Status:       "degraded",
					Reason:       "pipeline_stalled",
					LastBlockAgo: ago.Round(time.Second).String(),
				})
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ReadyzResponse{Status: "ready"})
}

// staleThreshold returns the configured stale block threshold, or 0 if unset.
func (s *Service) staleThreshold() time.Duration {
	if s != nil && s.config != nil {
		return s.config.Server.REST.StaleBlockThreshold
	}
	return 0
}

// metricsEnabled returns whether the /metrics endpoint should be registered.
func (s *Service) metricsEnabled() bool {
	if s != nil && s.config != nil {
		return s.config.Server.REST.MetricsEnabled
	}
	return config.DefaultMetricsEnabled
}
