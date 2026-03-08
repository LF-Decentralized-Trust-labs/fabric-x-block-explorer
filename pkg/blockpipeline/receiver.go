/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
)

// BlockDeliverer is the interface satisfied by *sidecarstream.Streamer.
// It is defined here to allow test mocks without importing sidecarstream.
type BlockDeliverer interface {
	StartDeliver(ctx context.Context, out chan<- *common.Block)
}

// maxReconnectWait caps the reconnect delay when the backoff policy returns backoff.Stop.
const maxReconnectWait = 30 * time.Second

// BlockReceiver streams blocks from the sidecar with automatic reconnection.
// bo controls the reconnect delay; BlockReceiver takes ownership and calls Reset() before the first connect.
func BlockReceiver(ctx context.Context, streamer BlockDeliverer, bo backoff.BackOff, out chan<- *common.Block) {
	logger.Info("blockReceiver started")
	bo.Reset()

	for {
		select {
		case <-ctx.Done():
			logger.Info("blockReceiver stopping")
			return
		default:
		}

		// per-stream context so the StartDeliver goroutine is cancelled on reconnect.
		streamCtx, streamCancel := context.WithCancel(ctx)
		blockCh := make(chan *common.Block, max(cap(out), 1))
		logger.Info("blockReceiver: starting sidecar stream")
		streamer.StartDeliver(streamCtx, blockCh)

		if err := consumeBlocks(ctx, blockCh, out); err != nil {
			logger.Warnf("blockReceiver stream error: %v", err)
		}
		streamCancel() // signal the StartDeliver goroutine to stop

		wait := bo.NextBackOff()
		if wait == backoff.Stop {
			wait = maxReconnectWait
		}
		logger.Infof("blockReceiver: reconnecting after %v", wait)

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			logger.Info("blockReceiver stopping before reconnect")
			return
		case <-timer.C:
		}
	}
}

// consumeBlocks forwards blocks from blockCh to out until blockCh closes or ctx is done.
func consumeBlocks(ctx context.Context, blockCh <-chan *common.Block, out chan<- *common.Block) error {
	return drainChan(ctx, blockCh, "sidecar block", func(blk *common.Block) error {
		select {
		case <-ctx.Done():
			return nil
		case out <- blk:
			return nil
		}
	})
}
