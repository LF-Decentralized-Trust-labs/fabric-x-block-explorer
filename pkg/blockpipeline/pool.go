/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/connection"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// Pool orchestrates the block-processing pipeline:
//
// BlockReceiver → rawCh → BlockProcessor(s) → procCh → BlockWriter(s)
//
// Channels are created per-Start call so Pool is restartable.
type Pool struct {
	buffer   config.BufferConfig
	workers  config.WorkerConfig
	pool     *pgxpool.Pool
	streamer *sidecarstream.Streamer
	retry    connection.RetryProfile
}

// New constructs a Pool. pool supplies DB connections; streamer delivers raw blocks.
// buffer and workers are expected to already have defaults applied (e.g. via config.LoadFromFile).
func New(
	buffer config.BufferConfig,
	workers config.WorkerConfig,
	pool *pgxpool.Pool,
	streamer *sidecarstream.Streamer,
	retry connection.RetryProfile,
) *Pool {
	if pool == nil {
		panic("blockpipeline.New: pool must not be nil")
	}
	if streamer == nil {
		panic("blockpipeline.New: streamer must not be nil")
	}
	return &Pool{buffer: buffer, workers: workers, pool: pool, streamer: streamer, retry: retry}
}

// Start launches the pipeline and blocks until all goroutines have exited.
// The pipeline runs until ctx is cancelled or a fatal error is returned by a writer.
func (p *Pool) Start(ctx context.Context) error {
	rawCh := make(chan *common.Block, p.buffer.RawChannelSize)
	procCh := make(chan *types.ProcessedBlock, p.buffer.ProcChannelSize)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(rawCh)
		return p.runReceiver(gCtx, rawCh)
	})

	var procWg sync.WaitGroup
	for i := range p.workers.ProcessorCount {
		procWg.Add(1)
		g.Go(func() error {
			defer procWg.Done()
			return p.runProcessor(gCtx, i, rawCh, procCh)
		})
	}

	// Close procCh once every processor has exited.
	g.Go(func() error {
		procWg.Wait()
		close(procCh)
		return nil
	})

	for i := range p.workers.WriterCount {
		g.Go(func() error {
			return p.runWriter(gCtx, i, procCh)
		})
	}

	return g.Wait()
}

// runReceiver streams raw blocks from the sidecar into out, reconnecting on
// transient errors using p.retry backoff. It returns nil when Deliver finishes
// its range cleanly, and ctx.Err() when the context is cancelled.
func (p *Pool) runReceiver(ctx context.Context, out chan<- *common.Block) error {
	bo := p.retry.NewBackoff()
	for ctx.Err() == nil {
		err := BlockReceiver(ctx, p.streamer, out)
		if err == nil {
			logger.Info("receiver: stream ended cleanly")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logger.Warnf("receiver: stream error, retrying after backoff: %v", err)
		if waitErr := connection.WaitForNextBackOffDuration(ctx, bo); waitErr != nil {
			return waitErr
		}
	}
	return ctx.Err()
}

// runProcessor parses raw blocks from rawCh and sends them to procCh.
func (p *Pool) runProcessor(ctx context.Context, i int, rawCh <-chan *common.Block, procCh chan<- *types.ProcessedBlock) error {
	logger.Infof("processor[%d] started", i)
	err := BlockProcessor(ctx, rawCh, procCh)
	logger.Infof("processor[%d] stopped", i)
	return err
}

// runWriter acquires a DB connection and persists processed blocks from procCh.
func (p *Pool) runWriter(ctx context.Context, i int, procCh <-chan *types.ProcessedBlock) error {
	logger.Infof("writer[%d] started", i)
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	persister := db.NewBlockWriterFromConn(conn)
	defer persister.Close()
	err = BlockWriter(ctx, persister, procCh)
	logger.Infof("writer[%d] stopped: %v", i, err)
	return err
}
