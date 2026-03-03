/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string //nolint:gosec // intentional: field holds a connection credential
	DBName   string
	SSLMode  string
	MaxConns int
}

// NewPostgres creates a new pgx connection pool using the given config.
func NewPostgres(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 20
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
		cfg.MaxConns,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pgx pool")
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Wrap(err, "failed to connect postgres")
	}

	return pool, nil
}
