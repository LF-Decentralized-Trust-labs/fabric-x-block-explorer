/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"net"
	"net/http"

	"github.com/cockroachdb/errors"
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

	if err = db.ApplySchema(ctx, pool); err != nil {
		return err
	}

	s.querier = dbsqlc.New(pool)

	streamer, err := sidecarstream.NewStreamer(s.config.Sidecar)
	if err != nil {
		return err
	}
	defer streamer.Close()

	pipeline, err := blockpipeline.New(blockpipeline.Config{
		Buffer:   s.config.Buffer,
		Workers:  s.config.Workers,
		DB:       pool,
		Streamer: streamer,
		Retry:    s.config.Sidecar.Retry,
	})
	if err != nil {
		return err
	}

	restSrv := &http.Server{
		Addr:              s.config.Server.REST.Endpoint.Address(),
		Handler:           s.newRESTRouter(),
		ReadHeaderTimeout: s.config.Server.REST.ReadHeaderTimeout,
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
	_ = restSrv.Shutdown(context.Background()) //nolint:contextcheck
	return g.Wait()
}

// runRESTServer serves HTTP until lis is closed or the server shuts down.
func (*Service) runRESTServer(lis net.Listener, srv *http.Server) error {
	if err := srv.Serve(lis); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
