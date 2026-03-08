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

	streamer, err := NewStreamer(localSidecarCfg())
	require.NoError(t, err)
	require.NotNil(t, streamer)
	assert.NotNil(t, streamer.client)

	streamer.Close()
}

func TestNewStreamerConfiguration(t *testing.T) {
	t.Parallel()

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

	streamer, err := NewStreamer(localSidecarCfg())
	require.NoError(t, err)
	require.NotNil(t, streamer)

	// Multiple calls must not panic.
	assert.NotPanics(t, func() { streamer.Close() })
	assert.NotPanics(t, func() { streamer.Close() })
}

func TestStreamerCloseNilClient(t *testing.T) {
	t.Parallel()

	streamer := &Streamer{client: nil}

	assert.NotPanics(t, func() { streamer.Close() })
}

func TestStartDeliver(t *testing.T) {
	t.Parallel()

	streamer, err := NewStreamer(localSidecarCfg())
	require.NoError(t, err)
	require.NotNil(t, streamer)
	defer streamer.Close()

	ctx, cancel := context.WithCancel(t.Context())
	blockCh := make(chan *common.Block, 10)

	// StartDeliver launches a goroutine; cancel immediately to stop it.
	assert.NotPanics(t, func() {
		streamer.StartDeliver(ctx, blockCh)
	})
	cancel()
}

func TestStartDeliverMultipleCalls(t *testing.T) {
	t.Parallel()

	streamer, err := NewStreamer(localSidecarCfg())
	require.NoError(t, err)
	require.NotNil(t, streamer)
	defer streamer.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	blockCh1 := make(chan *common.Block, 5)
	blockCh2 := make(chan *common.Block, 5)

	assert.NotPanics(t, func() {
		streamer.StartDeliver(ctx, blockCh1)
		streamer.StartDeliver(ctx, blockCh2)
	})
}
