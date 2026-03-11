/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

// Pipeline and DB default values.  These constants are also used by
// pkg/pipeline to validate and fill in a zero-value Config.
const (
	DefaultRawChannelSize  = 200
	DefaultProcChannelSize = 200
	DefaultProcessorCount  = 4
	DefaultWriterCount     = 4
	DefaultDBMaxConns      = 20
)
