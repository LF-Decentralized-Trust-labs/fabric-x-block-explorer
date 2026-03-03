/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dbtest

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// TestDBName is the test database name.
	TestDBName = "explorer_test"
	// TestDBUser is the test database user.
	TestDBUser = "postgres"
	// TestDBPassword is the test database password.
	TestDBPassword = "postgres"
	// TestDBHost is the test database host.
	TestDBHost = "localhost"
	// TestDBPort is the default PostgreSQL port used in tests.
	TestDBPort = 5432

	testDBImage = "postgres:14-alpine"

	// TestSetupTimeout is the maximum time allowed for test database setup.
	TestSetupTimeout            = 2 * time.Minute
	testContainerStartupTimeout = 60 * time.Second
)

// TestContainer holds the PostgreSQL testcontainer instance.
type TestContainer struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	DSN       string
}

// PrepareTestEnv sets up a PostgreSQL testcontainer for testing.
// It checks the DB_DEPLOYMENT environment variable:
// - If set to "local", it connects to a local PostgreSQL instance
// - Otherwise, it spins up a new testcontainer.
func PrepareTestEnv(t *testing.T) *TestContainer {
	t.Helper()

	if os.Getenv("DB_DEPLOYMENT") == "local" {
		return prepareLocalDB(t)
	}

	return prepareTestContainer(t)
}

// prepareLocalDB connects to a local postgres instance.
// Connection parameters can be overridden via standard PostgreSQL environment
// variables: PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE.
func prepareLocalDB(t *testing.T) *TestContainer {
	t.Helper()

	host := envOrDefault("PGHOST", TestDBHost)
	port := envOrDefaultInt("PGPORT", TestDBPort)
	user := envOrDefault("PGUSER", TestDBUser)
	password := envOrDefault("PGPASSWORD", TestDBPassword)
	dbname := envOrDefault("PGDATABASE", TestDBName)

	ctx, cancel := context.WithTimeout(t.Context(), TestSetupTimeout)
	t.Cleanup(cancel)
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		dbname,
	)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to connect to local database")

	err = pool.Ping(ctx)
	require.NoError(t, err, "failed to ping local database")

	// Drop the whole schema for a clean test state.
	cleanDatabase(t, pool)

	return &TestContainer{
		Container: nil,
		Pool:      pool,
		DSN:       dsn,
	}
}

// cleanDatabase drops and recreates the public schema for a clean test state.
func cleanDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	_, err := pool.Exec(t.Context(), `
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
	`)
	require.NoError(t, err, "failed to clean database")
}

// prepareTestContainer creates a PostgreSQL testcontainer.
func prepareTestContainer(t *testing.T) *TestContainer {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), TestSetupTimeout)
	t.Cleanup(cancel)
	postgresContainer, err := postgres.Run(ctx,
		testDBImage,
		postgres.WithDatabase(TestDBName),
		postgres.WithUsername(TestDBUser),
		postgres.WithPassword(TestDBPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
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

// Close cleans up the test database resources.
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

// envOrDefault returns the value of the environment variable key,
// or fallback if the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrDefaultInt returns the integer value of the environment variable key,
// or fallback if the variable is unset, empty, or not a valid integer.
func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
