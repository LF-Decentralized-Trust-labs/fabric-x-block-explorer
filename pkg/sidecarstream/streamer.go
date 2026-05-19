/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sidecarstream

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"
	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/delivercommitter"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

var logger = flogging.MustGetLogger("sidecarstream")

// Streamer wraps the fabric-x delivercommitter, resuming from a given start block.
type Streamer struct {
	clientConfig *connection.ClientConfig
	startBlk     uint64
}

// NewStreamer creates a Streamer using cfg for connection settings and startBlk
// as the first block number to request. Callers should compute startBlk from
// the database (MAX(block_num)+1) to avoid re-processing blocks on restart,
// falling back to cfg.StartBlk when the database is empty.
func NewStreamer(cfg config.SidecarConfig, startBlk uint64) *Streamer {
	logger.Infof("created streamer start=%d", startBlk)
	return &Streamer{
		clientConfig: &cfg.Connection,
		startBlk:     startBlk,
	}
}

// Deliver streams blocks to out, blocking until ctx is cancelled or a permanent error occurs.
// Reconnection on transient stream errors is handled automatically by delivercommitter.
func (s *Streamer) Deliver(ctx context.Context, out channel.Writer[*common.Block]) error {
	logger.Infof("Deliver start=%d", s.startBlk)
	// delivercommitter requires a raw channel; bridge its output to the context-backed writer.
	deliverBlockCh := make(chan *common.Block, 1)
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return delivercommitter.ToQueue(gCtx, delivercommitter.Parameters{
			ClientConfig: s.clientConfig,
			NextBlockNum: s.startBlk,
			OutputBlock:  deliverBlockCh,
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

// Close is a no-op; delivercommitter manages its own connections per Deliver call.
func (*Streamer) Close() {}
