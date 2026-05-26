/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

func newProcessedBlock(num uint64) *types.ProcessedBlock {
	return &types.ProcessedBlock{
		Data:      &types.ParsedBlockData{},
		BlockInfo: &types.BlockInfo{Number: num},
	}
}

// writerTestEnv holds the channels and done signal for a running blockWriter.
type writerTestEnv struct {
	in   chan *types.ProcessedBlock
	done chan error
}

//nolint:revive // t precedes ctx to enable t.Helper() marking.
func startBlockWriter(t *testing.T, ctx context.Context) writerTestEnv {
	t.Helper()
	s := writerTestEnv{
		in:   make(chan *types.ProcessedBlock, 10),
		done: make(chan error, 1),
	}
	// db.NewBlockWriter(nil): persister is never called in flow-only tests.
	go func() { s.done <- blockWriter(ctx, db.NewBlockWriter(nil), channel.NewReader(ctx, s.in)) }()
	return s
}

func TestBlockWriter(t *testing.T) {
	t.Parallel()
	testCtx, testCancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(testCancel)

	t.Run("returns nil on closed channel", func(t *testing.T) {
		t.Parallel()
		s := startBlockWriter(t, t.Context())
		close(s.in)
		require.NoError(t, <-s.done)
	})

	t.Run("returns nil on already-closed channel", func(t *testing.T) {
		t.Parallel()
		in := make(chan *types.ProcessedBlock, 10)
		close(in)
		require.NoError(t, blockWriter(t.Context(), db.NewBlockWriter(nil), channel.NewReader(t.Context(), in)))
	})

	t.Run("skips nil block", func(t *testing.T) {
		// A nil entry must be skipped; otherwise WriteProcessedBlock would error.
		t.Parallel()
		s := startBlockWriter(t, t.Context())
		s.in <- nil
		close(s.in)
		require.NoError(t, <-s.done)
	})

	t.Run("wraps db error", func(t *testing.T) {
		// db.NewBlockWriter(nil) has no pool/conn, so beginTx returns
		// "no pool or conn available". BlockWriter must wrap it as "db write error".
		t.Parallel()
		s := startBlockWriter(t, t.Context())
		s.in <- newProcessedBlock(1)
		err := <-s.done
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db write error")
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(testCtx)
		t.Cleanup(cancel)
		s := startBlockWriter(t, ctx)
		cancel()
		assert.ErrorIs(t, <-s.done, context.Canceled)
	})
}
