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

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

func newPB(num uint64) *types.ProcessedBlock {
	return &types.ProcessedBlock{
		Data:      &types.ParsedBlockData{},
		BlockInfo: &types.BlockInfo{Number: num},
	}
}

// writerSetup holds the channels and done signal for a running BlockWriter.
type writerSetup struct {
	in   chan *types.ProcessedBlock
	done chan error
}

func startBlockWriter(ctx context.Context) writerSetup {
	s := writerSetup{
		in:   make(chan *types.ProcessedBlock, 10),
		done: make(chan error, 1),
	}
	// db.NewBlockWriter(nil): persister is never called in flow-only tests.
	go func() { s.done <- BlockWriter(ctx, db.NewBlockWriter(nil), s.in) }()
	return s
}

func TestBlockWriter(t *testing.T) {
	t.Parallel()

	t.Run("returns nil on closed channel", func(t *testing.T) {
		t.Parallel()
		s := startBlockWriter(t.Context())
		close(s.in)
		require.NoError(t, <-s.done)
	})

	t.Run("returns nil on already-closed channel", func(t *testing.T) {
		t.Parallel()
		in := make(chan *types.ProcessedBlock, 10)
		close(in)
		require.NoError(t, BlockWriter(t.Context(), db.NewBlockWriter(nil), in))
	})

	t.Run("skips nil block", func(t *testing.T) {
		// Sends nil then closes. If nil were NOT skipped, WriteProcessedBlock
		// would be called and return "processed block is nil" — an error.
		t.Parallel()
		s := startBlockWriter(t.Context())
		s.in <- nil
		close(s.in)
		require.NoError(t, <-s.done)
	})

	t.Run("wraps db error", func(t *testing.T) {
		// db.NewBlockWriter(nil) has no pool/conn, so beginTx returns
		// "no pool or conn available". BlockWriter must wrap it as "db write error".
		t.Parallel()
		s := startBlockWriter(t.Context())
		s.in <- newPB(1)
		err := <-s.done
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db write error")
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		s := startBlockWriter(ctx)
		cancel()
		assert.ErrorIs(t, <-s.done, context.Canceled)
	})
}
