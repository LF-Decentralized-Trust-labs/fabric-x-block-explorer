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
	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// validBlock builds a minimal valid Block proto for parsing.
func validBlock(number uint64, txCount int) *common.Block {
	data := make([][]byte, txCount)
	for i := range data {
		data[i] = []byte("tx")
	}
	return &common.Block{
		Header: &common.BlockHeader{Number: number},
		Data:   &common.BlockData{Data: data},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{{}, {}, {}}, // SIGNATURES, LAST_CONFIG, TX_FILTER
		},
	}
}

// processorTestEnv holds the channels and done signal for a running blockProcessor.
type processorTestEnv struct {
	in   chan *common.Block
	out  chan *types.ProcessedBlock
	done chan error
}

func startBlockProcessor(ctx context.Context) processorTestEnv {
	s := processorTestEnv{
		in:   make(chan *common.Block, 10),
		out:  make(chan *types.ProcessedBlock, 10),
		done: make(chan error, 1),
	}
	inReader := channel.NewReader(ctx, s.in)
	outWriter := channel.NewWriter(ctx, s.out)
	go func() { s.done <- blockProcessor(ctx, inReader, outWriter) }()
	return s
}

func TestBlockProcessor(t *testing.T) {
	t.Parallel()

	t.Run("processes valid block", func(t *testing.T) {
		t.Parallel()
		s := startBlockProcessor(t.Context())
		s.in <- validBlock(1, 0)
		select {
		case processedBlock := <-s.out:
			assert.Equal(t, uint64(1), processedBlock.BlockInfo.Number)
			assert.NotNil(t, processedBlock.Data)
		case err := <-s.done:
			require.Fail(t, "unexpected processor exit", err)
		}
	})

	t.Run("skips nil block", func(t *testing.T) {
		t.Parallel()
		s := startBlockProcessor(t.Context())
		s.in <- nil              // should be skipped
		s.in <- validBlock(2, 0) // should arrive
		select {
		case processedBlock := <-s.out:
			assert.Equal(t, uint64(2), processedBlock.BlockInfo.Number)
		case err := <-s.done:
			require.Fail(t, "unexpected processor exit", err)
		}
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		s := startBlockProcessor(ctx)
		cancel()
		assert.ErrorIs(t, <-s.done, context.Canceled)
	})

	t.Run("returns nil on closed channel", func(t *testing.T) {
		t.Parallel()
		s := startBlockProcessor(t.Context())
		close(s.in)
		require.NoError(t, <-s.done)
	})

	t.Run("returns error on invalid block", func(t *testing.T) {
		t.Parallel()
		s := startBlockProcessor(t.Context())
		// nil Header triggers a parse error.
		s.in <- &common.Block{Header: nil, Data: &common.BlockData{}}
		err := <-s.done
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block processing error")
	})
}
