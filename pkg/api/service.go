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
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/hyperledger/fabric-x-committer/utils/channel"
	"github.com/hyperledger/fabric-x-committer/utils/connection"

	explorerv1 "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/api/proto"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/blockpipeline"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
)

// Service serves the block explorer gRPC and REST APIs.
// It implements connection.Service so it integrates with connection.StartService.
type Service struct {
	explorerv1.UnimplementedBlockExplorerServiceServer
	cfg    *config.Config
	q      dbsqlc.Querier
	ready  *channel.Ready
	health *health.Server
}

// New creates a new explorer Service.
func New(cfg *config.Config) *Service {
	return &Service{
		cfg:    cfg,
		ready:  channel.NewReady(),
		health: connection.DefaultHealthCheckService(),
	}
}

// WaitForReady implements connection.Service.
func (s *Service) WaitForReady(ctx context.Context) bool {
	return s.ready.WaitForReady(ctx)
}

// RegisterService implements connection.Service — registers the gRPC server.
func (s *Service) RegisterService(server *grpc.Server) {
	explorerv1.RegisterBlockExplorerServiceServer(server, s)
	healthgrpc.RegisterHealthServer(server, s.health)
	reflection.Register(server)
}

// Run implements connection.Service — opens the DB pool, starts the block
// ingestion pipeline, and runs the REST server.
func (s *Service) Run(ctx context.Context) error {
	pool, err := db.NewPostgres(ctx, db.Config{
		Endpoints:       s.cfg.DB.Endpoints,
		User:            s.cfg.DB.User,
		Password:        s.cfg.DB.Password,
		DBName:          s.cfg.DB.DBName,
		TLS:             s.cfg.DB.TLS,
		MaxConns:        s.cfg.DB.MaxConns,
		MaxConnIdleTime: s.cfg.DB.MaxConnIdleTime,
		MaxConnLifetime: s.cfg.DB.MaxConnLifetime,
		Retry:           &s.cfg.DB.Retry,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if err = db.ApplySchema(ctx, pool); err != nil {
		return err
	}

	s.q = dbsqlc.New(pool)

	streamer, err := sidecarstream.NewStreamer(s.cfg.Sidecar)
	if err != nil {
		return err
	}
	defer streamer.Close()

	wp, err := blockpipeline.New(blockpipeline.Config{
		Buffer:   s.cfg.Buffer,
		Workers:  s.cfg.Workers,
		DB:       pool,
		Streamer: streamer,
		Retry:    s.cfg.Sidecar.Retry,
	})
	if err != nil {
		return err
	}

	restSrv := &http.Server{
		Addr:              s.cfg.Server.REST.Endpoint.Address(),
		Handler:           s.newRESTRouter(),
		ReadHeaderTimeout: s.cfg.Server.REST.ReadHeaderTimeout,
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
	g.Go(func() error { return wp.Start(gCtx) })

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
