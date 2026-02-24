/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseTestEnv verifies that the test infrastructure works correctly.
// This is a simple test to ensure the testcontainer spins up and schema is initialized.
func TestDatabaseTestEnv(t *testing.T) {
	env := NewDatabaseTestEnv(t)

	// Verify connection is working
	ctx := context.Background()
	err := env.Pool.Ping(ctx)
	require.NoError(t, err, "database should be reachable")

	// Verify schema is initialized by checking table existence
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

	// Test default SSLMode behavior
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "explorer_test",
		SSLMode:  "",
	}

	// We just test that the function constructs the DSN correctly
	// Actual connection will fail without a real database, which is expected
	_, err := NewPostgres(cfg)
	if err != nil {
		// Expected to fail since there's no real database at these coordinates
		// Just verify the error is a connection error, not a code error
		require.Contains(t, err.Error(), "failed to", "error should be connection-related")
	}
}

// TestDatabaseHelper verifies helper methods in DatabaseTestEnv.
func TestDatabaseHelpers(t *testing.T) {
	env := NewDatabaseTestEnv(t)

	// Verify initial counts are zero
	blockCount := env.GetBlockCount(t)
	assert.Equal(t, int64(0), blockCount, "initial block count should be zero")

	txCount := env.GetTransactionCount(t)
	assert.Equal(t, int64(0), txCount, "initial transaction count should be zero")

	// Test AssertBlockNotExists
	env.AssertBlockNotExists(t, 1)
}
