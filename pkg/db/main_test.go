/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"os"
	"testing"

	committerdbtest "github.com/hyperledger/fabric-x-committer/service/vc/dbtest"
)

// TestMain sets package-wide environment variables before any test runs.
// Both vars must be set here rather than via t.Setenv because t.Setenv is
// incompatible with t.Parallel (Go 1.26+).
//
// DB_DEPLOYMENT=local: skip container spawning and connect to a pre-running
// Postgres instance on localhost:5433. Locally run: make start-db.
// In CI a postgres service is mapped to the same port via docker services.
//
// DB_TYPE=postgres: select PostgreSQL (committer default is YugabyteDB).
func TestMain(m *testing.M) {
	for k, v := range map[string]string{
		"DB_DEPLOYMENT": "local",
		"DB_TYPE":       committerdbtest.PostgresDBType,
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	m.Run()
}
