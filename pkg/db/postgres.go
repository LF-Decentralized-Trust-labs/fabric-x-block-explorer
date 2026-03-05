/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/dbconn"
)

const defaultMaxConns = 20

// Config holds PostgreSQL connection configuration.
type Config struct {
	Endpoints []*connection.Endpoint
	User      string
	Password  string //nolint:gosec // intentional: field holds a connection credential
	DBName    string
	TLS       dbconn.DatabaseTLSConfig
	MaxConns  int32
	// MaxConnIdleTime is the maximum time a connection may sit idle; 0 uses the default (5m).
	MaxConnIdleTime time.Duration
	// MaxConnLifetime is the maximum total lifetime of a connection; 0 uses the default (1h).
	MaxConnLifetime time.Duration
	// Retry controls connection retries; when nil a single attempt is made.
	Retry *connection.RetryProfile
}

// NewPostgres creates a new pgx connection pool using the given config.
func NewPostgres(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = defaultMaxConns
	}

	dsn, err := dbconn.DataSourceName(dbconn.DataSourceNameParams{
		Username:        cfg.User,
		Password:        cfg.Password,
		EndpointsString: connection.AddressString(cfg.Endpoints...),
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
	applyPoolLimits(poolCfg, cfg)

	pool, err := connectWithRetry(ctx, poolCfg, cfg.Retry)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to postgres")
	}
	return pool, nil
}

func applyPoolLimits(poolCfg *pgxpool.Config, cfg Config) {
	poolCfg.MaxConns = cfg.MaxConns
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	} else {
		poolCfg.MaxConnIdleTime = 5 * time.Minute
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	} else {
		poolCfg.MaxConnLifetime = time.Hour
	}
}

func connectWithRetry(
	ctx context.Context, poolCfg *pgxpool.Config, retry *connection.RetryProfile,
) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	connect := func() error {
		p, err := pgxpool.NewWithConfig(ctx, poolCfg)
		if err != nil {
			return err
		}
		if err = p.Ping(ctx); err != nil {
			p.Close()
			return err
		}
		pool = p
		return nil
	}
	if retry != nil {
		return pool, retry.Execute(ctx, connect)
	}
	return pool, connect()
}
