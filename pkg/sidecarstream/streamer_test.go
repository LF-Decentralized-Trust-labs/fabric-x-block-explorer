/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sidecarstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/connection"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

// localSidecarCfg returns a SidecarConfig pointing at localhost:7052.
// sidecarclient.New uses lazy gRPC connections, so no live server is needed.
func localSidecarCfg() config.SidecarConfig {
	return config.SidecarConfig{
		Connection: connection.ClientConfig{
			Endpoint: &connection.Endpoint{Host: "localhost", Port: 7052},
		},
		ChannelID: "testchannel",
		StartBlk:  0,
		EndBlk:    1000,
	}
}

func TestNewStreamer(t *testing.T) {
	t.Parallel()

	t.Run("creates streamer with valid config", func(t *testing.T) {
		t.Parallel()
		streamer, err := NewStreamer(localSidecarCfg())
		require.NoError(t, err)
		require.NotNil(t, streamer)
		assert.NotNil(t, streamer.client)
		streamer.Close()
	})

	cases := []struct {
		name      string
		channelID string
		startBlk  uint64
		endBlk    uint64
	}{
		{"default range", "mychannel", 0, 1000},
		{"specific range", "businesschannel", 100, 200},
		{"large range", "ledger1", 500, 5000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.SidecarConfig{
				Connection: connection.ClientConfig{
					Endpoint: &connection.Endpoint{Host: "localhost", Port: 7052},
				},
				ChannelID: tc.channelID,
				StartBlk:  tc.startBlk,
				EndBlk:    tc.endBlk,
			}
			streamer, err := NewStreamer(cfg)
			require.NoError(t, err)
			require.NotNil(t, streamer)
			assert.Equal(t, tc.channelID, streamer.channelID)
			assert.Equal(t, int64(tc.startBlk), streamer.startBlk) //nolint:gosec
			assert.Equal(t, tc.endBlk, streamer.endBlk)
			streamer.Close()
		})
	}
}

func TestStreamerClose(t *testing.T) {
	t.Parallel()

	t.Run("multiple calls must not panic", func(t *testing.T) {
		t.Parallel()
		streamer, err := NewStreamer(localSidecarCfg())
		require.NoError(t, err)
		require.NotNil(t, streamer)
		assert.NotPanics(t, func() { streamer.Close() })
		assert.NotPanics(t, func() { streamer.Close() })
	})

	t.Run("nil client must not panic", func(t *testing.T) {
		t.Parallel()
		streamer := &Streamer{client: nil}
		assert.NotPanics(t, func() { streamer.Close() })
	})
}

func TestDeliver(t *testing.T) {
	t.Parallel()

	t.Run("does not panic on single call", func(t *testing.T) {
		t.Parallel()
		streamer, err := NewStreamer(localSidecarCfg())
		require.NoError(t, err)
		require.NotNil(t, streamer)
		defer streamer.Close()

		ctx, cancel := context.WithCancel(t.Context())
		// Deliver is blocking; run in a goroutine and cancel to stop it.
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = streamer.Deliver(ctx, make(chan *common.Block, 10))
		}()
		cancel()
		<-done
	})

	t.Run("does not panic on multiple concurrent calls", func(t *testing.T) {
		t.Parallel()
		streamer, err := NewStreamer(localSidecarCfg())
		require.NoError(t, err)
		require.NotNil(t, streamer)
		defer streamer.Close()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() { _ = streamer.Deliver(ctx, make(chan *common.Block, 5)) }()
		go func() { _ = streamer.Deliver(ctx, make(chan *common.Block, 5)) }()
	})
}
