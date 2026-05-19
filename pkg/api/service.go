/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/blockpipeline"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
)

// Service serves the block explorer REST API.
type Service struct {
	config  *config.Config
	querier dbsqlc.Querier
	ready   *channel.Ready
}

// New creates a new explorer Service.
func New(cfg *config.Config) *Service {
	return &Service{
		config: cfg,
		ready:  channel.NewReady(),
	}
}

// WaitForReady waits for the service to be ready.
func (s *Service) WaitForReady(ctx context.Context) bool {
	return s.ready.WaitForReady(ctx)
}

// Run opens the DB pool, starts the block ingestion pipeline, and runs the REST server.
func (s *Service) Run(ctx context.Context) error {
	pool, err := db.NewPostgres(ctx, db.Config{
		Endpoints:       s.config.DB.Endpoints,
		User:            s.config.DB.User,
		Password:        s.config.DB.Password,
		DBName:          s.config.DB.DBName,
		TLS:             s.config.DB.TLS,
		MaxConns:        s.config.DB.MaxConns,
		MaxConnIdleTime: s.config.DB.MaxConnIdleTime,
		MaxConnLifetime: s.config.DB.MaxConnLifetime,
		Retry:           &s.config.DB.Retry,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if err = db.ApplyMigrations(ctx, pool); err != nil {
		return err
	}

	s.querier = dbsqlc.New(pool)

	// Compute the resume block: start from MAX(block_num)+1 when the DB already
	// has data so that restarts skip already-processed blocks.
	startBlk := resumeBlockNum(ctx, pool, s.config.Sidecar.StartBlk)

	streamer := sidecarstream.NewStreamer(s.config.Sidecar, startBlk)
	defer streamer.Close()

	pipeline, err := blockpipeline.New(blockpipeline.Config{
		Buffer:   s.config.Buffer,
		Workers:  s.config.Workers,
		DB:       pool,
		Streamer: streamer,
	})
	if err != nil {
		return err
	}

	restSrv := &http.Server{
		Addr:              s.config.Server.REST.Endpoint.Address(),
		Handler:           s.newRESTRouter(),
		ReadHeaderTimeout: orDefault(s.config.Server.REST.ReadHeaderTimeout, config.DefaultReadHeaderTimeout),
		ReadTimeout:       orDefault(s.config.Server.REST.ReadTimeout, config.DefaultReadTimeout),
		WriteTimeout:      orDefault(s.config.Server.REST.WriteTimeout, config.DefaultWriteTimeout),
	}
	restLis, err := net.Listen("tcp", restSrv.Addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = restLis.Close()
	}()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.runRESTServer(restLis, restSrv) })
	g.Go(func() error { return pipeline.Start(gCtx) })

	s.ready.SignalReady()

	<-gCtx.Done()

	shutdownTimeout := orDefault(s.config.Server.REST.ShutdownTimeout, config.DefaultShutdownTimeout)
	// Use Background so shutdown completes even if the parent ctx is already cancelled.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	_ = restSrv.Shutdown(shutdownCtx) //nolint:contextcheck

	return g.Wait()
}

// resumeBlockNum finds the first block_num >= fallback that is absent from the DB.
// This guarantees no block is ever skipped after a restart, even when multiple
// concurrent writers left a gap (e.g. writer A committed block 6 before writer B
// failed on block 5 and rolled back).
//
// Behaviour:
//   - Empty table                 → returns fallback (first run).
//   - No gaps in [fallback, MAX]  → returns MAX+1 (normal restart).
//   - Gap at position N           → returns N (re-processes from the gap to heal it).
//
// All INSERT queries use ON CONFLICT DO NOTHING, so re-processing already-stored
// blocks after a gap-heal restart is a safe no-op.
func resumeBlockNum(ctx context.Context, pool *pgxpool.Pool, fallback uint64) uint64 {
	// Uses a UNION of two candidates:
	//   1. The fallback block itself (catches missing start or empty table).
	//   2. The successor of every block where the next stored block is not consecutive
	//      (catches internal gaps and returns MAX+1 when there are no gaps).
	// COALESCE(MIN(...), fallback) ensures we always return a value.
	const q = `
		SELECT COALESCE(MIN(candidate), $1) FROM (
			SELECT $1::bigint AS candidate
			WHERE NOT EXISTS (SELECT 1 FROM blocks WHERE block_num = $1)
			UNION ALL
			SELECT block_num + 1 AS candidate
			FROM (
				SELECT block_num,
				       LEAD(block_num) OVER (ORDER BY block_num) AS nxt
				FROM blocks
				WHERE block_num >= $1
			) w
			WHERE nxt IS NULL OR nxt > block_num + 1
		) candidates`

	var resume int64
	//nolint:gosec // block_num fits in int64; fallback is a config-validated uint64
	if err := pool.QueryRow(ctx, q, int64(fallback)).Scan(&resume); err != nil {
		restLogger.Warnf("resumeBlockNum: gap scan failed, using fallback %d: %v", fallback, err)
		return fallback
	}
	return uint64(resume) //nolint:gosec // result is always >= 0
}

// runRESTServer serves HTTP until lis is closed or the server shuts down.
func (*Service) runRESTServer(lis net.Listener, srv *http.Server) error {
	if err := srv.Serve(lis); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// orDefault returns d when v is zero.
func orDefault(v, d time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return d
}
