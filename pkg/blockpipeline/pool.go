/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"
	"github.com/hyperledger/fabric-x-committer/utils/connection"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// Service orchestrates the block-processing pipeline:
//
// blockReceiver → inputBlockCh → blockProcessor(s) → processedBlockCh → blockWriter(s)
//
// Channels are created per-Start call so Service is restartable.
type Service struct {
	buffer   config.BufferConfig
	workers  config.WorkerConfig
	pool     *pgxpool.Pool
	streamer *sidecarstream.Streamer
	retry    connection.RetryProfile
}

// Config holds all dependencies and tunables for a Service.
// DB and Streamer must not be nil; Buffer and Workers are expected to already
// have defaults applied (e.g. via config.LoadFromFile).
type Config struct {
	Buffer   config.BufferConfig
	Workers  config.WorkerConfig
	DB       *pgxpool.Pool
	Streamer *sidecarstream.Streamer
	Retry    connection.RetryProfile
}

// New constructs a Service from cfg.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil {
		return nil, errors.New("blockpipeline: DB pool must not be nil")
	}
	if cfg.Streamer == nil {
		return nil, errors.New("blockpipeline: streamer must not be nil")
	}
	return &Service{
		buffer:   cfg.Buffer,
		workers:  cfg.Workers,
		pool:     cfg.DB,
		streamer: cfg.Streamer,
		retry:    cfg.Retry,
	}, nil
}

// Start launches the pipeline and blocks until all goroutines have exited.
// The pipeline runs until ctx is cancelled or a fatal error is returned by a writer.
func (p *Service) Start(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	inputBlockCh := channel.Make[*common.Block](gCtx, p.buffer.RawChannelSize)
	processedBlockCh := channel.Make[*types.ProcessedBlock](gCtx, p.buffer.ProcChannelSize)

	g.Go(func() error {
		return p.runBlockReceiver(gCtx, inputBlockCh)
	})

	for i := range p.workers.ProcessorCount {
		g.Go(func() error {
			return runBlockProcessor(gCtx, i, inputBlockCh, processedBlockCh)
		})
	}

	for i := range p.workers.WriterCount {
		g.Go(func() error {
			return p.runBlockWriter(gCtx, i, processedBlockCh)
		})
	}

	return g.Wait()
}

// runBlockReceiver streams raw blocks from the sidecar into out, reconnecting on
// transient errors using p.retry backoff. It returns nil when Deliver finishes
// its range cleanly, and ctx.Err() when the context is cancelled.
func (p *Service) runBlockReceiver(ctx context.Context, out channel.Writer[*common.Block]) error {
	bo := p.retry.NewBackoff()
	for ctx.Err() == nil {
		err := blockReceiver(ctx, p.streamer, out)
		if err == nil {
			logger.Info("receiver: stream ended cleanly")
			return nil
		}
		logger.Warnf("receiver: stream error, retrying after backoff: %v", err)
		if waitErr := connection.WaitForNextBackOffDuration(ctx, bo); waitErr != nil {
			return waitErr
		}
	}
	return ctx.Err()
}

// runBlockProcessor parses raw blocks from in and sends them to out.
func runBlockProcessor(
	ctx context.Context,
	i int,
	in channel.Reader[*common.Block],
	out channel.Writer[*types.ProcessedBlock],
) error {
	logger.Infof("processor[%d] started", i)
	err := blockProcessor(ctx, in, out)
	logger.Infof("processor[%d] stopped", i)
	return err
}

// runBlockWriter acquires a DB connection and persists processed blocks from in.
func (p *Service) runBlockWriter(ctx context.Context, i int, in channel.Reader[*types.ProcessedBlock]) error {
	logger.Infof("writer[%d] started", i)
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	persister := db.NewBlockWriterFromConn(conn)
	defer persister.Close()
	err = blockWriter(ctx, persister, in)
	logger.Infof("writer[%d] stopped: %v", i, err)
	return err
}
