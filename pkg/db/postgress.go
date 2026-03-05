/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/dbconn"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string //nolint:gosec // intentional: field holds a connection credential
	DBName   string
	TLS      dbconn.DatabaseTLSConfig
	MaxConns int
}

// NewPostgres creates a new pgx connection pool using the given config.
func NewPostgres(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 20
	}

	endpoint := &connection.Endpoint{Host: cfg.Host, Port: cfg.Port}
	dsn, err := dbconn.DataSourceName(dbconn.DataSourceNameParams{
		Username:        cfg.User,
		Password:        cfg.Password,
		EndpointsString: endpoint.Address(),
		Database:        cfg.DBName,
		TLS:             cfg.TLS,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to build DSN")
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse pool config")
	}
	poolCfg.MaxConns = int32(cfg.MaxConns) //nolint:gosec // MaxConns is validated above

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pgx pool")
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Wrap(err, "failed to connect postgres")
	}

	return pool, nil
}
