/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
)

// BlockReceiver runs one streaming session: it concurrently drives Deliver and
// forwards every block it produces to out via consumeBlocks.
//
// It returns nil when Deliver exits cleanly (e.g. EndBlk reached) and a
// non-nil error on failure or context cancellation.
// Reconnection on transient errors is the caller's responsibility.
func BlockReceiver(ctx context.Context, streamer *sidecarstream.Streamer, out chan<- *common.Block) error {
	logger.Info("blockReceiver started")
	blockCh := make(chan *common.Block, max(cap(out), 1))

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(blockCh)
		return streamer.Deliver(gCtx, blockCh)
	})
	g.Go(func() error {
		return consumeBlocks(gCtx, blockCh, out)
	})

	err := g.Wait()
	if err != nil && ctx.Err() == nil {
		logger.Warnf("blockReceiver stopped with error: %v", err)
	} else {
		logger.Info("blockReceiver stopping")
	}
	return err
}

// consumeBlocks forwards blocks from blockCh to out until blockCh closes or ctx is done.
// A closed blockCh is the expected EOF signal from Deliver; it returns ctx.Err() which
// is nil on a clean close and non-nil when ctx was already cancelled.
func consumeBlocks(ctx context.Context, blockCh <-chan *common.Block, out chan<- *common.Block) error {
	reader := channel.NewReader(ctx, blockCh)
	writer := channel.NewWriter(ctx, out)
	for {
		blk, ok := reader.Read()
		if !ok {
			// blockCh closed (Deliver EOF) or ctx cancelled.
			return ctx.Err()
		}
		if blk == nil {
			continue
		}
		if !writer.Write(blk) {
			return ctx.Err()
		}
	}
}
