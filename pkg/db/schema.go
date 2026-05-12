/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	_ "embed"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// ApplySchema creates all tables and indexes defined in schema.sql if they do
// not already exist. The schema is fully idempotent — safe to call on every startup.
func ApplySchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return errors.Wrap(err, "failed to apply schema")
	}
	return nil
}
