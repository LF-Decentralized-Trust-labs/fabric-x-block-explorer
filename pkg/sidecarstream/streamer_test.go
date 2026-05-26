/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sidecarstream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"
	"github.com/hyperledger/fabric-x-committer/utils/connection"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

// localSidecarCfg returns a SidecarConfig pointing at localhost:7052.
// delivercommitter uses lazy gRPC connections, so no live server is needed.
func localSidecarCfg() config.SidecarConfig {
	return config.SidecarConfig{
		Connection: connection.ClientConfig{
			Endpoint: &connection.Endpoint{Host: "localhost", Port: 7052},
		},
		StartBlk: 0,
	}
}

// newLocalStreamer creates a Streamer using localSidecarCfg and startBlk 0.
func newLocalStreamer(t *testing.T) *Streamer {
	t.Helper()
	s := NewStreamer(localSidecarCfg(), 0)
	require.NotNil(t, s)
	return s
}

func TestNewStreamer(t *testing.T) {
	t.Parallel()

	t.Run("creates streamer with valid config", func(t *testing.T) {
		t.Parallel()
		streamer := newLocalStreamer(t)
		require.NotNil(t, streamer.clientConfig)
		cases := []struct {
			name     string
			startBlk uint64
		}{
			{"default range", 0},
			{"specific range", 100},
			{"large range", 500},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				cfg := config.SidecarConfig{
					Connection: connection.ClientConfig{
						Endpoint: &connection.Endpoint{Host: "localhost", Port: 7052},
					},
				}
				streamer := NewStreamer(cfg, tc.startBlk)
				require.NotNil(t, streamer)
				assert.Equal(t, tc.startBlk, streamer.startBlk)
				streamer.Close()
			})
		}
	})
}

func TestStreamerClose(t *testing.T) {
	t.Parallel()

	t.Run("multiple calls must not panic", func(t *testing.T) {
		t.Parallel()
		streamer := newLocalStreamer(t)
		assert.NotPanics(t, func() { streamer.Close() })
		assert.NotPanics(t, func() { streamer.Close() })
	})

	t.Run("empty streamer must not panic", func(t *testing.T) {
		t.Parallel()
		streamer := &Streamer{}
		assert.NotPanics(t, func() { streamer.Close() })
	})
}

func TestDeliver(t *testing.T) {
	t.Parallel()
	testCtx, testCancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(testCancel)

	t.Run("does not panic on single call", func(t *testing.T) {
		t.Parallel()
		streamer := newLocalStreamer(t)
		t.Cleanup(streamer.Close)

		ctx, cancel := context.WithCancel(testCtx)
		t.Cleanup(cancel)
		// Deliver is blocking; run in a goroutine and cancel to stop it.
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = streamer.Deliver(ctx, channel.Make[*common.Block](ctx, 10))
		}()
		cancel()
		<-done
	})

	t.Run("does not panic on multiple concurrent calls", func(t *testing.T) {
		t.Parallel()
		streamer := newLocalStreamer(t)
		t.Cleanup(streamer.Close)

		ctx, cancel := context.WithCancel(testCtx)
		t.Cleanup(cancel)
		go func() { _ = streamer.Deliver(ctx, channel.Make[*common.Block](ctx, 5)) }()
		go func() { _ = streamer.Deliver(ctx, channel.Make[*common.Block](ctx, 5)) }()
	})
}
