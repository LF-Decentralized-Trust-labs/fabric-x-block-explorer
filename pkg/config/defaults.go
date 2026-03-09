/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import "time"

// Shared runtime defaults used across configuration loading and package-level fallbacks.
const (
	DefaultRawChannelSize  = 200
	DefaultProcChannelSize = 200
	DefaultProcessorCount  = 4
	DefaultWriterCount     = 4
	DefaultDBMaxConns      = 20
	DefaultTxLimit         = 50
)

// DefaultMaxReconnectWait bounds reconnect backoff when no explicit value is configured.
const DefaultMaxReconnectWait = 30 * time.Second
