//go:build db

/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-committer/utils/testdb"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
)

func TestMain(m *testing.M) {
	for k, v := range map[string]string{
		"DB_DEPLOYMENT": "local",
		"DB_TYPE":       testdb.PostgresDBType,
	} {
		if err := os.Setenv(k, v); err != nil {
			log.Fatal(err) //nolint:revive // TestMain exit semantics
		}
	}
	os.Exit(m.Run())
}

// newTestPool creates an isolated Postgres DB for one test.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	conn := testdb.PrepareTestEnv(t)
	pool, err := db.NewPostgres(t.Context(), db.Config{
		Endpoints: conn.Endpoints,
		User:      conn.User,
		Password:  conn.Password,
		DBName:    conn.Database,
		TLS:       conn.TLS,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.ApplyMigrations(t.Context(), pool))
	return pool
}

// insertBlock inserts a minimal block row directly so resumeBlockNum has data to
// query without needing the full write pipeline.
func insertBlock(t *testing.T, pool *pgxpool.Pool, blockNum int64) {
	t.Helper()
	_, err := pool.Exec(t.Context(),
		`INSERT INTO blocks(block_num, tx_count) VALUES($1, 0) ON CONFLICT DO NOTHING`,
		blockNum,
	)
	require.NoError(t, err)
}

// ── resumeBlockNum ────────────────────────────────────────────────────────────

func TestResumeBlockNum(t *testing.T) {
	t.Parallel()

	t.Run("empty_table_returns_fallback", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		assert.Equal(t, uint64(0), resumeBlockNum(context.Background(), pool, 0))
	})

	t.Run("contiguous_blocks_returns_max_plus_one", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		for i := int64(0); i <= 4; i++ {
			insertBlock(t, pool, i)
		}
		assert.Equal(t, uint64(5), resumeBlockNum(context.Background(), pool, 0))
	})

	t.Run("gap_at_middle_returns_gap_position", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		// Blocks 0,1,3 — gap at 2
		for _, n := range []int64{0, 1, 3} {
			insertBlock(t, pool, n)
		}
		assert.Equal(t, uint64(2), resumeBlockNum(context.Background(), pool, 0))
	})

	t.Run("gap_at_start_returns_fallback", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		// Blocks 2,3 — fallback=0 is absent, so returns 0
		for _, n := range []int64{2, 3} {
			insertBlock(t, pool, n)
		}
		assert.Equal(t, uint64(0), resumeBlockNum(context.Background(), pool, 0))
	})

	t.Run("fallback_already_in_db_returns_max_plus_one", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		for i := int64(5); i <= 7; i++ {
			insertBlock(t, pool, i)
		}
		assert.Equal(t, uint64(8), resumeBlockNum(context.Background(), pool, 5))
	})

	t.Run("multi_writer_gap_returns_lowest_gap", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(t)
		// Simulate two writers leaving gaps at 2 and 5
		for _, n := range []int64{0, 1, 3, 4, 6} {
			insertBlock(t, pool, n)
		}
		assert.Equal(t, uint64(2), resumeBlockNum(context.Background(), pool, 0))
	})
}
