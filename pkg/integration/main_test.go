/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package integration_test runs end-to-end tests against a live
// fabric-x-committer stack (via the all-in-one test node Docker image) and
// validates the explorer REST API.
//
// Prerequisites:
//   - The committer test node image must be available locally.
//     Build it first with: make build-test-node
//   - A local PostgreSQL instance must be running on localhost:5433.
//     Start it with: make start-db
//
// Run via: make test-integration
package integration_test

import (
	"log"
	"os"
	"os/exec"
	"testing"

	"github.com/hyperledger/fabric-x-committer/utils/testdb"
)

// TestMain configures the shared test environment before any test runs.
//
// It mirrors the committer's docker/test/main_test.go but skips the
// image build step: the image is expected to be pre-built via
// `make build-test-node`.
func TestMain(m *testing.M) {
	// Require the committer test node image to be available locally.
	// If it is missing, fail immediately with a clear message.
	if err := exec.Command("docker", "image", "inspect", testNodeImage()).Run(); err != nil {
		//nolint:revive,nolintlint // false positive.
		log.Fatalf("committer test node image %q not found — build it with: make build-test-node\n",
			testNodeImage())
	}

	// Use the pre-running local Postgres (started by make start-db / make ensure-db).
	// DB_DEPLOYMENT=local: skip container spawning and connect directly.
	// DB_TYPE=postgres: select PostgreSQL credentials instead of YugabyteDB defaults.
	// These must be set in TestMain (not t.Setenv) because t.Setenv is incompatible
	// with t.Parallel in Go 1.26+.
	for k, v := range map[string]string{
		"DB_DEPLOYMENT": "local",
		"DB_TYPE":       testdb.PostgresDBType,
	} {
		if err := os.Setenv(k, v); err != nil {
			log.Fatal(err) //nolint:revive,nolintlint // false positive.
		}
	}

	m.Run() //nolint:revive,nolintlint // false positive.
}
