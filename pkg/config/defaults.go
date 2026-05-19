/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
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
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultRollbackTimeout   = 5 * time.Second
	DefaultReadTimeout       = 60 * time.Second
	DefaultWriteTimeout      = 60 * time.Second
	DefaultShutdownTimeout   = 30 * time.Second

	// Retry back-off defaults (used for database connection retry).
	DefaultRetryInitialInterval = 500 * time.Millisecond
	DefaultRetryMaxInterval     = 10 * time.Second
	DefaultRetryMaxElapsedTime  = 15 * time.Minute
	DefaultRetryRandomization   = 0.5
	DefaultRetryMultiplier      = 1.5
)

// newViperWithDefaults returns a viper instance pre-loaded with sensible
// defaults for all Config fields. Environment variables with the EXPLORER_
// prefix override any file-based value.
func newViperWithDefaults() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(envKeyReplacer)
	v.AutomaticEnv()
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
	v.SetDefault("server.rest.default_tx_limit", DefaultTxLimit)
	v.SetDefault("server.rest.read_header_timeout", DefaultReadHeaderTimeout)
	v.SetDefault("server.rest.read_timeout", DefaultReadTimeout)
	v.SetDefault("server.rest.write_timeout", DefaultWriteTimeout)
	v.SetDefault("server.rest.shutdown_timeout", DefaultShutdownTimeout)
	return v
}
