/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
)

// mockDeliverer implements BlockDeliverer for unit tests.
type mockDeliverer struct {
	blocks []*common.Block
}

func (m *mockDeliverer) StartDeliver(ctx context.Context, out chan<- *common.Block) {
	go func() {
		defer close(out)
		for _, blk := range m.blocks {
			select {
			case <-ctx.Done():
				return
			case out <- blk:
			}
		}
	}()
}

// noRetryBackoff stops immediately so reconnect loops exit fast in tests.
func noRetryBackoff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = time.Nanosecond
	return b
}

func TestConsumeBlocks(t *testing.T) {
	t.Parallel()

	blockCh := make(chan *common.Block, 10)
	out := make(chan *common.Block, 10)

	go func() {
		for i := 1; i <= 3; i++ {
			blockCh <- &common.Block{Header: &common.BlockHeader{Number: uint64(i)}}
		}
		close(blockCh) // signal end-of-stream so consumeBlocks returns
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- consumeBlocks(t.Context(), blockCh, out) }()

	for i := 1; i <= 3; i++ {
		blk := <-out
		assert.Equal(t, uint64(i), blk.Header.Number)
	}
	// blockCh was closed while ctx is still active, so consumeBlocks returns an error.
	require.Error(t, <-errCh)
}

func TestConsumeBlocksNilBlock(t *testing.T) {
	t.Parallel()

	blockCh := make(chan *common.Block, 10)
	out := make(chan *common.Block, 10)

	go func() {
		blockCh <- nil
		blockCh <- &common.Block{Header: &common.BlockHeader{Number: 1}}
		close(blockCh) // signal end-of-stream so consumeBlocks returns
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- consumeBlocks(t.Context(), blockCh, out) }()

	blk := <-out
	assert.Equal(t, uint64(1), blk.Header.Number)
	// blockCh was closed while ctx is still active, so consumeBlocks returns an error.
	require.Error(t, <-errCh)
}

func TestConsumeBlocksChannelClosed(t *testing.T) {
	t.Parallel()

	blockCh := make(chan *common.Block, 10)
	out := make(chan *common.Block, 10)
	close(blockCh)

	err := consumeBlocks(t.Context(), blockCh, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
}

func TestConsumeBlocksContextCancellation(t *testing.T) {
	t.Parallel()

	blockCh := make(chan *common.Block)
	out := make(chan *common.Block, 10)

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- consumeBlocks(ctx, blockCh, out) }()
	cancel()

	err := <-errCh
	assert.NoError(t, err)
}

func TestBlockReceiverContextCancellation(t *testing.T) {
	t.Parallel()

	out := make(chan *common.Block, 10)
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel before BlockReceiver even starts

	done := make(chan struct{})
	go func() {
		defer close(done)
		BlockReceiver(ctx, &mockDeliverer{}, noRetryBackoff(), out)
	}()

	<-done // must return cleanly
}

func TestBlockReceiverDeliversBlocks(t *testing.T) {
	t.Parallel()

	blocks := []*common.Block{
		{Header: &common.BlockHeader{Number: 1}},
		{Header: &common.BlockHeader{Number: 2}},
		{Header: &common.BlockHeader{Number: 3}},
	}
	deliverer := &mockDeliverer{blocks: blocks}
	out := make(chan *common.Block, 10)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		BlockReceiver(ctx, deliverer, noRetryBackoff(), out)
	}()

	for i := 1; i <= 3; i++ {
		blk := <-out
		assert.Equal(t, uint64(i), blk.Header.Number)
	}
	cancel()
	<-done // wait for BlockReceiver to exit cleanly
}
