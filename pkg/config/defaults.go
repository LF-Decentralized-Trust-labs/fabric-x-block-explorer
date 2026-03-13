/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"math"
	"time"

	"github.com/spf13/viper"
)

// Pipeline and DB default values. These constants are also used by
// pkg/blockpipeline to validate and fill in a zero-value Config.
const (
	DefaultRawChannelSize    = 200
	DefaultProcChannelSize   = 200
	DefaultProcessorCount    = 4
	DefaultWriterCount       = 4
	DefaultDBMaxConns        = 20
	DefaultDBMaxConnIdleTime = 5 * time.Minute
	DefaultDBMaxConnLifetime = 1 * time.Hour
	DefaultTxLimit           = 50
	DefaultMaxReconnectWait  = 30 * time.Second
	DefaultReadHeaderTimeout = 5 * time.Second

	// Retry back-off defaults.
	DefaultRetryInitialInterval = 500 * time.Millisecond
	DefaultRetryMaxInterval     = 10 * time.Second
	DefaultRetryMaxElapsedTime  = 15 * time.Minute
	DefaultRetryRandomization   = 0.5
	DefaultRetryMultiplier      = 1.5
)

// newViperWithDefaults returns a viper instance pre-loaded with sensible
// defaults for all Config fields.
func newViperWithDefaults() *viper.Viper {
	v := viper.New()
	v.SetDefault("buffer.raw_channel_size", DefaultRawChannelSize)
	v.SetDefault("buffer.proc_channel_size", DefaultProcChannelSize)
	v.SetDefault("workers.processor_count", DefaultProcessorCount)
	v.SetDefault("workers.writer_count", DefaultWriterCount)
	v.SetDefault("database.max_conns", DefaultDBMaxConns)
	v.SetDefault("database.max_conn_idle_time", DefaultDBMaxConnIdleTime)
	v.SetDefault("database.max_conn_lifetime", DefaultDBMaxConnLifetime)
	v.SetDefault("database.retry.initial-interval", DefaultRetryInitialInterval)
	v.SetDefault("database.retry.randomization-factor", DefaultRetryRandomization)
	v.SetDefault("database.retry.multiplier", DefaultRetryMultiplier)
	v.SetDefault("database.retry.max-interval", DefaultRetryMaxInterval)
	v.SetDefault("database.retry.max-elapsed-time", DefaultRetryMaxElapsedTime)
	v.SetDefault("sidecar.end_block", uint64(math.MaxUint64)) // stream indefinitely
	v.SetDefault("sidecar.max_reconnect_wait", DefaultMaxReconnectWait)
	v.SetDefault("sidecar.retry.initial-interval", DefaultRetryInitialInterval)
	v.SetDefault("sidecar.retry.randomization-factor", DefaultRetryRandomization)
	v.SetDefault("sidecar.retry.multiplier", DefaultRetryMultiplier)
	v.SetDefault("sidecar.retry.max-interval", DefaultRetryMaxInterval)
	v.SetDefault("sidecar.retry.max-elapsed-time", DefaultRetryMaxElapsedTime)
	v.SetDefault("server.rest.default_tx_limit", DefaultTxLimit)
	v.SetDefault("server.rest.read_header_timeout", DefaultReadHeaderTimeout)
	return v
}
