/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-committer/utils/connection"
)

func TestDatabaseTestEnv(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	ctx := t.Context()

	t.Run("pool is reachable", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, env.Pool.Ping(ctx), "database should be reachable")
	})

	t.Run("schema is initialised", func(t *testing.T) {
		t.Parallel()
		var tableExists bool
		err := env.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = 'blocks'
			)
		`).Scan(&tableExists)
		require.NoError(t, err)
		assert.True(t, tableExists, "blocks table should exist")
	})
}

func TestNewPostgres(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)
	c := env.Pool.Config().ConnConfig

	pool, err := NewPostgres(t.Context(), Config{
		Endpoints: []*connection.Endpoint{{Host: c.Host, Port: int(c.Port)}},
		User:      c.User,
		Password:  c.Password,
		DBName:    c.Database,
	})
	require.NoError(t, err)
	defer pool.Close()
}

func TestDatabaseHelpers(t *testing.T) {
	t.Parallel()
	env := NewDatabaseTestEnv(t)

	t.Run("initial counts are zero", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, int64(0), env.GetBlockCount(t), "initial block count should be zero")
		assert.Equal(t, int64(0), env.GetTransactionCount(t), "initial transaction count should be zero")
	})

	t.Run("AssertBlockNotExists for absent block", func(t *testing.T) {
		t.Parallel()
		env.AssertBlockNotExists(t, 1)
	})
}
