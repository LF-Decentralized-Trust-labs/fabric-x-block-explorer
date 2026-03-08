/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// mockPersister records written blocks and can inject an error on demand.
type mockPersister struct {
	mu      sync.Mutex
	written []*types.ProcessedBlock
	failOn  int // fail when len(written) reaches this value (0 = never fail)
	failErr error
}

func (m *mockPersister) WriteProcessedBlock(_ context.Context, pb *types.ProcessedBlock) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn > 0 && len(m.written)+1 == m.failOn {
		if m.failErr == nil {
			panic("mockPersister: failOn set but failErr is nil")
		}
		return m.failErr
	}
	m.written = append(m.written, pb)
	return nil
}

func (m *mockPersister) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.written)
}

func newPB(num uint64) *types.ProcessedBlock {
	return &types.ProcessedBlock{
		Data:      &types.ParsedBlockData{},
		BlockInfo: &types.BlockInfo{Number: num},
	}
}

func TestBlockWriter(t *testing.T) {
	t.Parallel()

	p := &mockPersister{}
	in := make(chan *types.ProcessedBlock, 10)

	done := make(chan error, 1)
	go func() { done <- BlockWriter(t.Context(), p, in) }()

	in <- newPB(1)
	close(in)
	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
	assert.Equal(t, 1, p.count())
}

func TestBlockWriterMultipleBlocks(t *testing.T) {
	t.Parallel()

	p := &mockPersister{}
	in := make(chan *types.ProcessedBlock, 10)

	done := make(chan error, 1)
	go func() { done <- BlockWriter(t.Context(), p, in) }()

	for i := uint64(1); i <= 3; i++ {
		in <- newPB(i)
	}
	close(in)

	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
	assert.Equal(t, 3, p.count())
}

func TestBlockWriterNilBlock(t *testing.T) {
	t.Parallel()

	p := &mockPersister{}
	in := make(chan *types.ProcessedBlock, 10)

	done := make(chan error, 1)
	go func() { done <- BlockWriter(t.Context(), p, in) }()

	in <- nil      // skipped
	in <- newPB(1) // written
	close(in)

	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
	assert.Equal(t, 1, p.count())
}

func TestBlockWriterContextCancellation(t *testing.T) {
	t.Parallel()

	p := &mockPersister{}
	in := make(chan *types.ProcessedBlock, 10)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- BlockWriter(ctx, p, in) }()

	cancel()
	err := <-done
	assert.NoError(t, err)
}

func TestBlockWriterChannelClosed(t *testing.T) {
	t.Parallel()

	p := &mockPersister{}
	in := make(chan *types.ProcessedBlock, 10)
	close(in)

	err := BlockWriter(t.Context(), p, in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel closed")
}

func TestBlockWriterPersistError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("storage unavailable")
	p := &mockPersister{failOn: 1, failErr: writeErr}
	in := make(chan *types.ProcessedBlock, 10)

	done := make(chan error, 1)
	go func() { done <- BlockWriter(t.Context(), p, in) }()

	in <- newPB(1)

	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db write error")
}
