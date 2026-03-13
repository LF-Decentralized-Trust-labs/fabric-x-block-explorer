/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sidecarstream

import (
	"context"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/service/sidecar/sidecarclient"
	"github.com/hyperledger/fabric-x-committer/utils/channel"
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

// Deliver streams blocks to out, blocking until the stream ends or ctx is cancelled.
func (s *Streamer) Deliver(ctx context.Context, out channel.Writer[*common.Block]) error {
	logger.Infof("Deliver channel=%s start=%d end=%d", s.channelID, s.startBlk, s.endBlk)
	// sidecarclient requires a raw channel; bridge its output to the context-backed writer.
	deliverBlockCh := make(chan *common.Block, 1)
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.client.Deliver(gCtx, &sidecarclient.DeliverParameters{
			StartBlkNum: s.startBlk,
			EndBlkNum:   s.endBlk,
			OutputBlock: deliverBlockCh,
		})
	})
	g.Go(func() error {
		reader := channel.NewReader(ctx, deliverBlockCh)
		for {
			blk, ok := reader.Read()
			if !ok {
				return ctx.Err()
			}
			out.Write(blk)
		}
	})
	return g.Wait()
}

// Close releases the sidecar client connections.
func (s *Streamer) Close() {
	if s.client != nil {
		s.client.CloseConnections()
	}
}
