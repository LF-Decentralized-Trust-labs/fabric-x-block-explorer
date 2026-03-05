/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	committerdbtest "github.com/hyperledger/fabric-x-committer/service/vc/dbtest"

	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
)

//go:embed schema.sql
var schemaSQL string

// DatabaseTestEnv provides a test database environment.
type DatabaseTestEnv struct {
	Pool    *pgxpool.Pool
	Queries *dbsqlc.Queries
}

// NewDatabaseTestEnv creates a new test environment backed by a fresh isolated
// PostgreSQL database. It uses the fabric-x-committer test infrastructure to
// spin up a container (or connect to a local instance via DB_DEPLOYMENT=local)
// and creates a uniquely-named database per test so tests can run in parallel.
// The database is dropped and resources are released via t.Cleanup.
func NewDatabaseTestEnv(t *testing.T) *DatabaseTestEnv {
	t.Helper()

	conn := committerdbtest.PrepareTestEnv(t)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)

	pool, err := NewPostgres(ctx, Config{
		Endpoints: conn.Endpoints,
		User:      conn.User,
		Password:  conn.Password,
		DBName:    conn.Database,
		TLS:       conn.TLS,
	})
	require.NoError(t, err, "failed to create connection pool")
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, schemaSQL)
	require.NoError(t, err, "failed to initialize database schema")

	return &DatabaseTestEnv{
		Pool:    pool,
		Queries: dbsqlc.New(pool),
	}
}

// AssertBlockExists checks that a block exists.
func (env *DatabaseTestEnv) AssertBlockExists(t *testing.T, blockNum int64) {
	t.Helper()

	ctx := t.Context()
	block, err := env.Queries.GetBlock(ctx, blockNum)
	require.NoError(t, err, "block %d should exist", blockNum)
	require.Equal(t, blockNum, block.BlockNum)
}

// AssertBlockNotExists checks that a block does not exist.
func (env *DatabaseTestEnv) AssertBlockNotExists(t *testing.T, blockNum int64) {
	t.Helper()

	ctx := t.Context()
	_, err := env.Queries.GetBlock(ctx, blockNum)
	require.Error(t, err, "block %d should not exist", blockNum)
}

// GetBlockCount returns the number of blocks.
func (env *DatabaseTestEnv) GetBlockCount(t *testing.T) int64 {
	t.Helper()

	ctx := t.Context()
	var count int64
	err := env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM blocks").Scan(&count)
	require.NoError(t, err, "failed to count blocks")
	return count
}

// GetTransactionCount returns the total number of transactions in the database.
func (env *DatabaseTestEnv) GetTransactionCount(t *testing.T) int64 {
	t.Helper()

	ctx := t.Context()
	var count int64
	err := env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions").Scan(&count)
	require.NoError(t, err, "failed to count transactions")
	return count
}
