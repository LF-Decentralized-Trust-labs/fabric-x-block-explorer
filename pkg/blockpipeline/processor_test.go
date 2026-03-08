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

func TestBlockProcessor(t *testing.T) {
	t.Parallel()

	in := make(chan *common.Block, 10)
	out := make(chan *types.ProcessedBlock, 10)
	done := make(chan error, 1)
	go func() { done <- BlockProcessor(t.Context(), in, out) }()

	in <- validBlock(1, 0)

	select {
	case pb := <-out:
		assert.Equal(t, uint64(1), pb.BlockInfo.Number)
		assert.NotNil(t, pb.Data)
	case err := <-done:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockProcessorNilBlock(t *testing.T) {
	t.Parallel()

	in := make(chan *common.Block, 10)
	out := make(chan *types.ProcessedBlock, 10)
	done := make(chan error, 1)
	go func() { done <- BlockProcessor(t.Context(), in, out) }()

	in <- nil              // should be skipped
	in <- validBlock(2, 0) // should arrive

	select {
	case pb := <-out:
		assert.Equal(t, uint64(2), pb.BlockInfo.Number)
	case err := <-done:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockProcessorContextCancellation(t *testing.T) {
	t.Parallel()

	in := make(chan *common.Block, 10)
	out := make(chan *types.ProcessedBlock, 10)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- BlockProcessor(ctx, in, out) }()
	cancel()

	err := <-done
	assert.NoError(t, err)
}

func TestBlockProcessorChannelClosed(t *testing.T) {
	t.Parallel()

	in := make(chan *common.Block, 10)
	out := make(chan *types.ProcessedBlock, 10)
	done := make(chan error, 1)
	go func() { done <- BlockProcessor(t.Context(), in, out) }()
	close(in)

	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
}

func TestBlockProcessorInvalidBlock(t *testing.T) {
	t.Parallel()

	in := make(chan *common.Block, 10)
	out := make(chan *types.ProcessedBlock, 10)
	done := make(chan error, 1)
	go func() { done <- BlockProcessor(t.Context(), in, out) }()

	// nil Header triggers a parse error.
	in <- &common.Block{Header: nil, Data: &common.BlockData{}}

	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block processing error")
}

func TestProcessBlock(t *testing.T) {
	t.Parallel()

	processed, err := processBlock(validBlock(5, 2))
	require.NoError(t, err)

	assert.Equal(t, uint64(5), processed.BlockInfo.Number)
	// The two data entries are not valid protobuf envelopes so the parser
	// silently drops them; Transactions will be empty but that is expected.
	assert.NotNil(t, processed.Data)
}

func TestProcessBlockNilHeader(t *testing.T) {
	t.Parallel()

	_, err := processBlock(&common.Block{Header: nil, Data: &common.BlockData{}})
	assert.Error(t, err)
}
