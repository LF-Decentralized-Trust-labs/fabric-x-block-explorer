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

// Config holds all dependencies and tunables for a Pool.
// DB and Streamer must not be nil; Buffer and Workers are expected to already
// have defaults applied (e.g. via config.LoadFromFile).
type Config struct {
	Buffer   config.BufferConfig
	Workers  config.WorkerConfig
	DB       *pgxpool.Pool
	Streamer *sidecarstream.Streamer
	Retry    connection.RetryProfile
}

// New constructs a Pool from cfg.
func New(cfg Config) *Pool {
	if cfg.DB == nil {
		panic("blockpipeline.New: DB pool must not be nil")
	}
	if cfg.Streamer == nil {
		panic("blockpipeline.New: streamer must not be nil")
	}
	return &Pool{
		buffer:   cfg.Buffer,
		workers:  cfg.Workers,
		pool:     cfg.DB,
		streamer: cfg.Streamer,
		retry:    cfg.Retry,
	}
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
			return runProcessor(gCtx, i, rawCh, procCh)
		})
	}

	// Close procCh once every processor has exited.
	g.Go(func() error {
		return closeWhenDone(&procWg, procCh)
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
func runProcessor(
	ctx context.Context,
	i int,
	rawCh <-chan *common.Block,
	procCh chan<- *types.ProcessedBlock,
) error {
	logger.Infof("processor[%d] started", i)
	err := BlockProcessor(ctx, rawCh, procCh)
	logger.Infof("processor[%d] stopped", i)
	return err
}

// closeWhenDone waits for wg to reach zero then closes ch,
// signalling downstream consumers that no more items will be sent.
func closeWhenDone(wg *sync.WaitGroup, ch chan<- *types.ProcessedBlock) error {
	wg.Wait()
	close(ch)
	return nil
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
