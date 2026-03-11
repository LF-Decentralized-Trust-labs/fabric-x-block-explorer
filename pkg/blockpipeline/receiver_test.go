/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
)

func TestConsumeBlocks(t *testing.T) {
	t.Parallel()

	t.Run("forwards all blocks and returns nil on clean close", func(t *testing.T) {
		t.Parallel()
		blockCh := make(chan *common.Block, 10)
		out := make(chan *common.Block, 10)
		go func() {
			for i := 1; i <= 3; i++ {
				blockCh <- &common.Block{Header: &common.BlockHeader{Number: uint64(i)}}
			}
			close(blockCh) // EOF signal from Deliver
		}()
		errCh := make(chan error, 1)
		go func() { errCh <- consumeBlocks(t.Context(), blockCh, out) }()

		for i := 1; i <= 3; i++ {
			blk := <-out
			assert.Equal(t, uint64(i), blk.Header.Number)
		}
		// closed blockCh is the normal EOF signal — consumeBlocks returns nil.
		require.NoError(t, <-errCh)
	})

	t.Run("skips nil block", func(t *testing.T) {
		t.Parallel()
		blockCh := make(chan *common.Block, 10)
		out := make(chan *common.Block, 10)
		go func() {
			blockCh <- nil
			blockCh <- &common.Block{Header: &common.BlockHeader{Number: 1}}
			close(blockCh) // EOF signal from Deliver
		}()
		errCh := make(chan error, 1)
		go func() { errCh <- consumeBlocks(t.Context(), blockCh, out) }()

		blk := <-out
		assert.Equal(t, uint64(1), blk.Header.Number)
		require.NoError(t, <-errCh)
	})

	t.Run("returns nil on already-closed channel", func(t *testing.T) {
		t.Parallel()
		blockCh := make(chan *common.Block, 10)
		out := make(chan *common.Block, 10)
		close(blockCh)
		require.NoError(t, consumeBlocks(t.Context(), blockCh, out))
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		t.Parallel()
		blockCh := make(chan *common.Block)
		out := make(chan *common.Block, 10)
		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() { errCh <- consumeBlocks(ctx, blockCh, out) }()
		cancel()
		assert.ErrorIs(t, <-errCh, context.Canceled)
	})
}
