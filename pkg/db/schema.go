/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"embed"
	"io/fs"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const createSchemaVersionsTable = `CREATE TABLE IF NOT EXISTS schema_migrations` +
	` (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`

// ApplyMigrations runs all pending SQL migrations in version order.
// Each migration is executed inside its own transaction and recorded in the
// schema_migrations table. Already-applied versions are skipped, making
// repeated calls fully idempotent.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, createSchemaVersionsTable); err != nil {
		return errors.Wrap(err, "failed to create schema_migrations table")
	}

	applied, err := loadAppliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	names, err := migrationFileNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			logger.Debugf("migration %s already applied, skipping", name)
			continue
		}
		logger.Infof("applying migration %s", name)
		if err := runMigration(ctx, pool, name); err != nil {
			return errors.Wrapf(err, "migration %s failed", name)
		}
		logger.Infof("migration %s applied successfully", name)
	}
	return nil
}

// loadAppliedVersions returns the set of migration filenames already recorded
// in the schema_migrations table.
func loadAppliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, errors.Wrap(err, "failed to query applied migrations")
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, errors.Wrap(err, "failed to scan migration version")
		}
		applied[v] = true
	}
	return applied, errors.Wrap(rows.Err(), "error iterating migration rows")
}

// migrationFileNames returns all embedded *.sql filenames sorted ascending.
func migrationFileNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read embedded migrations directory")
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// runMigration executes a single migration file inside a transaction and records
// the version in schema_migrations on success.
func runMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
	if err != nil {
		return errors.Wrapf(err, "failed to read migration file %s", name)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin migration transaction")
	}
	defer rollbackMigrationTx(tx) //nolint:contextcheck // rollback uses Background so it survives ctx cancellation

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return errors.Wrap(err, "failed to execute migration SQL")
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", name); err != nil {
		return errors.Wrap(err, "failed to record migration version")
	}
	return errors.Wrap(tx.Commit(ctx), "failed to commit migration transaction")
}

// rollbackMigrationTx attempts to roll back tx using a fresh context so that the
// rollback succeeds even when the original caller context is already cancelled.
func rollbackMigrationTx(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultRollbackTimeout)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		logger.Warnf("migration rollback error: %v", err)
	}
}
