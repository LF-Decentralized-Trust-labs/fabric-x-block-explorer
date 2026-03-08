/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sidecarstream

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/service/sidecar/sidecarclient"
	"github.com/hyperledger/fabric-x-committer/utils/logging"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

var logger = logging.New("sidecarstream")

// Streamer wraps the fabric sidecar client, adding start/end block range.
type Streamer struct {
	client    *sidecarclient.Client
	channelID string
	startBlk  int64
	endBlk    uint64
}

// NewStreamer creates a Streamer from the given SidecarConfig.
func NewStreamer(cfg config.SidecarConfig) (*Streamer, error) {
	params := &sidecarclient.Parameters{
		Client:    &cfg.Connection,
		ChannelID: cfg.ChannelID,
	}
	client, err := sidecarclient.New(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create sidecar client")
	}
	logger.Infof("created streamer channel=%s start=%d end=%d",
		cfg.ChannelID, cfg.StartBlk, cfg.EndBlk)
	return &Streamer{
		client:    client,
		channelID: cfg.ChannelID,
		startBlk:  int64(cfg.StartBlk), //nolint:gosec // StartBlk fits in int64
		endBlk:    cfg.EndBlk,
	}, nil
}

// StartDeliver begins streaming blocks to out in a background goroutine.
// It closes out when the stream ends (either clean finish or error) so that
// the receiver can detect the EOF and trigger a reconnect.
func (s *Streamer) StartDeliver(ctx context.Context, out chan<- *common.Block) {
	logger.Infof("StartDeliver channel=%s start=%d end=%d", s.channelID, s.startBlk, s.endBlk)
	go func() {
		defer close(out) // signal EOF so consumeBlocks returns and BlockReceiver can reconnect
		params := &sidecarclient.DeliverParameters{
			StartBlkNum: s.startBlk,
			EndBlkNum:   s.endBlk,
			OutputBlock: out,
		}
		if err := s.client.Deliver(ctx, params); err != nil {
			logger.Warnf("Deliver returned error: %v", err)
		}
	}()
}

// Close releases the sidecar client connections.
func (s *Streamer) Close() {
	if s.client != nil {
		s.client.CloseConnections()
	}
}
