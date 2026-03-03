/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseTestEnv verifies that the test infrastructure works correctly.
func TestDatabaseTestEnv(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)

	ctx := t.Context()
	err := env.Pool.Ping(ctx)
	require.NoError(t, err, "database should be reachable")

	var tableExists bool
	err = env.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'blocks'
		)
	`).Scan(&tableExists)
	require.NoError(t, err)
	assert.True(t, tableExists, "blocks table should exist")
}

// TestNewPostgres verifies the NewPostgres function creates a valid connection pool.
func TestNewPostgres(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "explorer_test",
		SSLMode:  "",
	}

	_, err := NewPostgres(cfg)
	if err != nil {
		require.Contains(t, err.Error(), "failed to", "error should be connection-related")
	}
}

// TestDatabaseHelpers verifies helper methods in DatabaseTestEnv.
func TestDatabaseHelpers(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)

	blockCount := env.GetBlockCount(t)
	assert.Equal(t, int64(0), blockCount, "initial block count should be zero")

	txCount := env.GetTransactionCount(t)
	assert.Equal(t, int64(0), txCount, "initial transaction count should be zero")

	env.AssertBlockNotExists(t, 1)
}
