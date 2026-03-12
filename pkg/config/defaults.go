/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"math"

	"github.com/spf13/viper"
)

// Pipeline and DB default values.  These constants are also used by
// pkg/blockpipeline to validate and fill in a zero-value Config.
const (
	DefaultRawChannelSize  = 200
	DefaultProcChannelSize = 200
	DefaultProcessorCount  = 4
	DefaultWriterCount     = 4
	DefaultDBMaxConns      = 20
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
	v.SetDefault("sidecar.end_block", uint64(math.MaxUint64)) // stream indefinitely
	return v
}
