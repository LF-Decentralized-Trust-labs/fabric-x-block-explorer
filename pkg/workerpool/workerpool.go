/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package workerpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/logging"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/blockpipeline"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = logging.New("workerpool")

// Config controls goroutine counts and channel buffer sizes.
type Config struct {
	ProcessorCount   int
	WriterCount      int
	RawChannelSize   int
	ProcChannelSize  int
	MaxReconnectWait time.Duration
}

// Pool orchestrates the block-processing pipeline:
//
// BlockReceiver → rawCh → BlockProcessor(s) → procCh → BlockWriter(s)
//
// Channels are created per-Start call so Pool is restartable.
type Pool struct {
	cfg      Config
	pool     *pgxpool.Pool
	streamer blockpipeline.BlockDeliverer
	retry    connection.RetryProfile
}

// New constructs a Pool. pool supplies DB connections; streamer delivers raw blocks.
func New(
	cfg Config,
	pool *pgxpool.Pool,
	streamer blockpipeline.BlockDeliverer,
	retry connection.RetryProfile,
) (*Pool, error) {
	if pool == nil {
		return nil, errors.New("workerpool.New: pool must not be nil")
	}
	if streamer == nil {
		return nil, errors.New("workerpool.New: streamer must not be nil")
	}
	if cfg.RawChannelSize <= 0 {
		cfg.RawChannelSize = config.DefaultRawChannelSize
	}
	if cfg.ProcChannelSize <= 0 {
		cfg.ProcChannelSize = config.DefaultProcChannelSize
	}
	if cfg.ProcessorCount <= 0 {
		cfg.ProcessorCount = config.DefaultProcessorCount
	}
	if cfg.WriterCount <= 0 {
		cfg.WriterCount = config.DefaultWriterCount
	}
	if cfg.MaxReconnectWait <= 0 {
		cfg.MaxReconnectWait = config.DefaultMaxReconnectWait
	}
	return &Pool{cfg: cfg, pool: pool, streamer: streamer, retry: retry}, nil
}

// Start launches the pipeline and returns an errgroup that is done when all
// goroutines have exited.  The pipeline runs until ctx is cancelled or a
// fatal error is returned by a writer.
func (p *Pool) Start(ctx context.Context) *errgroup.Group {
	rawCh := make(chan *common.Block, p.cfg.RawChannelSize)
	procCh := make(chan *types.ProcessedBlock, p.cfg.ProcChannelSize)

	g, gCtx := errgroup.WithContext(ctx)

	// Receiver — streams raw blocks from the sidecar into rawCh.
	// Closes rawCh on return so processors drain and exit naturally.
	g.Go(func() error {
		defer close(rawCh)
		blockpipeline.BlockReceiver(gCtx, p.streamer, blockpipeline.ReceiverConfig{
			Backoff:          p.retry.NewBackoff(),
			Out:              rawCh,
			MaxReconnectWait: p.cfg.MaxReconnectWait,
		})
		return nil
	})

	// Processors — parse raw blocks; each exits when rawCh is closed.
	var procWg sync.WaitGroup
	for i := range p.cfg.ProcessorCount {
		procWg.Add(1)
		g.Go(func() error {
			defer procWg.Done()
			logger.Infof("processor[%d] started", i)
			err := blockpipeline.BlockProcessor(gCtx, rawCh, procCh)
			logger.Infof("processor[%d] stopped", i)
			return err
		})
	}

	// Close procCh once every processor has exited.
	g.Go(func() error {
		procWg.Wait()
		close(procCh)
		return nil
	})

	// Writers — persist processed blocks; each exits when procCh is closed.
	for i := range p.cfg.WriterCount {
		g.Go(func() error {
			logger.Infof("writer[%d] started", i)

			conn, err := p.pool.Acquire(gCtx)
			if err != nil {
				return err
			}
			persister := db.NewBlockWriterFromConn(conn)
			defer persister.Close()

			err = blockpipeline.BlockWriter(gCtx, persister, procCh)
			logger.Infof("writer[%d] stopped: %v", i, err)
			return err
		})
	}

	return g
}
