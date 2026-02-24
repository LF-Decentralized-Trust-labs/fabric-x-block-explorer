/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dbtest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// PostgreSQL configuration
	testDBName     = "explorer_test"
	testDBUser     = "postgres"
	testDBPassword = "postgres"
)

// TestContainer holds the PostgreSQL testcontainer instance
type TestContainer struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	DSN       string
}

// PrepareTestEnv sets up a PostgreSQL testcontainer for testing.
// It checks the DB_DEPLOYMENT environment variable:
// - If set to "local", it connects to a local PostgreSQL instance
// - Otherwise, it spins up a new testcontainer
//
// This follows the fabric-x-committer pattern for flexible test environments.
func PrepareTestEnv(t *testing.T) *TestContainer {
	t.Helper()

	ctx := context.Background()

	// Check if using local database
	if os.Getenv("DB_DEPLOYMENT") == "local" {
		return prepareLocalDB(t, ctx)
	}

	// Use testcontainer
	return prepareTestContainer(t, ctx)
}

// prepareLocalDB connects to local postgres.
func prepareLocalDB(t *testing.T, ctx context.Context) *TestContainer {
	t.Helper()

	dsn := fmt.Sprintf(
		"postgres://%s:%s@localhost:5432/%s?sslmode=disable",
		testDBUser,
		testDBPassword,
		testDBName,
	)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to connect to local database")

	err = pool.Ping(ctx)
	require.NoError(t, err, "failed to ping local database")

	// Clean all tables before each test
	cleanDatabase(t, ctx, pool)

	return &TestContainer{
		Container: nil, // no container when using local
		Pool:      pool,
		DSN:       dsn,
	}
}

// cleanDatabase drops all tables for a clean test state.
func cleanDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	_, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS tx_endorsements CASCADE;
		DROP TABLE IF EXISTS tx_writes CASCADE;
		DROP TABLE IF EXISTS tx_reads CASCADE;
		DROP TABLE IF EXISTS tx_namespaces CASCADE;
		DROP TABLE IF EXISTS transactions CASCADE;
		DROP TABLE IF EXISTS namespace_policies CASCADE;
		DROP TABLE IF EXISTS blocks CASCADE;
	`)
	require.NoError(t, err, "failed to clean database")
}

// prepareTestContainer creates a PostgreSQL testcontainer.
func prepareTestContainer(t *testing.T, ctx context.Context) *TestContainer {
	t.Helper()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:14-alpine",
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create connection pool")

	err = pool.Ping(ctx)
	require.NoError(t, err, "failed to ping database")

	return &TestContainer{
		Container: postgresContainer,
		Pool:      pool,
		DSN:       dsn,
	}
}

// Close cleans up the test database resources
func (tc *TestContainer) Close(t *testing.T) {
	t.Helper()

	if tc.Pool != nil {
		tc.Pool.Close()
	}

	if tc.Container != nil {
		ctx := context.Background()
		err := tc.Container.Terminate(ctx)
		require.NoError(t, err, "failed to terminate container")
	}
}
