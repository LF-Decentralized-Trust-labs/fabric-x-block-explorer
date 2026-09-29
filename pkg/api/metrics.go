/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus instruments for the block explorer.
// A private registry is used so tests do not conflict with each other or with
// the default global registry.
type Metrics struct {
	registry *prometheus.Registry

	// BlockHeightCurrent is the latest ingested block number.
	BlockHeightCurrent prometheus.Gauge
	// BlocksIngestedTotal counts all blocks written to the DB.
	BlocksIngestedTotal prometheus.Counter
	// HTTPRequestsTotal counts REST requests by method, path, and status.
	HTTPRequestsTotal *prometheus.CounterVec
	// HTTPRequestDuration records REST request latency by method and path.
	HTTPRequestDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers all instruments against a fresh private registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		BlockHeightCurrent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "explorer_block_height_current",
			Help: "Latest ingested block number.",
		}),
		BlocksIngestedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "explorer_blocks_ingested_total",
			Help: "Total number of blocks written to the database.",
		}),
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "explorer_http_requests_total",
			Help: "Total REST API requests partitioned by method, path, and status.",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "explorer_http_request_duration_seconds",
			Help:    "REST API request latency partitioned by method and path.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}
	reg.MustRegister(
		m.BlockHeightCurrent,
		m.BlocksIngestedTotal,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
	)
	return m
}

// Handler returns the promhttp handler for this private registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// RecordBlock updates the block height gauge and increments the ingestion counter.
func (m *Metrics) RecordBlock(blockNum int64) {
	m.BlockHeightCurrent.Set(float64(blockNum))
	m.BlocksIngestedTotal.Inc()
}
