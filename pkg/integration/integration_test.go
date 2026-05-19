/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/api"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/testdb"
)

const (
	testContainerName    = "sc_test_explorer_integration"
	sidecarContainerPort = "4001"
	committerModule      = "github.com/hyperledger/fabric-x-committer"
	testNodeImageBase    = "docker.io/hyperledger/committer-test-node"
)

// testNodeImage returns the committer test node image tag that matches the
// version declared in go.mod, e.g. "docker.io/hyperledger/committer-test-node:v1.0.0-alpha.2".
// This ensures the integration test always uses the same committer version the
// module was compiled against.
func testNodeImage() string {
	return testNodeImageBase + ":" + committerVersion()
}

// committerVersion walks up the directory tree to find go.mod and extracts
// the version of committerModule declared there.
func committerVersion() string {
	dir, err := filepath.Abs(".")
	if err != nil {
		panic("committerVersion: cannot determine working directory: " + err.Error())
	}
	for {
		if v, found := versionFromGoMod(filepath.Join(dir, "go.mod")); found {
			return v
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	panic("committerVersion: go.mod not found")
}

// versionFromGoMod parses a single go.mod file and returns the version for
// committerModule. The second return value is false when the file does not
// exist or the module is not listed as a requirement.
func versionFromGoMod(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		panic("versionFromGoMod: cannot parse " + path + ": " + err.Error())
	}
	for _, req := range f.Require {
		if req.Mod.Path == committerModule {
			return req.Mod.Version, true
		}
	}
	panic("versionFromGoMod: " + committerModule + " not found in " + path)
}

// ── Response body types ──────────────────────────────────────────────────────

type blockHeightBody struct {
	Height int64 `json:"height"`
}

type blockSummary struct {
	BlockNum  int64      `json:"block_num"`
	BlockSize int64      `json:"block_size"`
	CreatedAt *time.Time `json:"created_at"`
}

type listBlocksBody struct {
	Blocks []blockSummary `json:"blocks"`
}

type txSummary struct {
	TxID       string    `json:"tx_id"`
	ReadWrites []rwEntry `json:"read_writes"`
}

type rwEntry struct {
	NsID string `json:"ns_id"`
}

type blockDetailBody struct {
	BlockNum     int64       `json:"block_num"`
	Transactions []txSummary `json:"transactions"`
}

type txDetailBody struct {
	TxID           string     `json:"tx_id"`
	ValidationCode string     `json:"validation_code"`
	ChannelID      string     `json:"channel_id"`
	CreatedAt      *time.Time `json:"created_at"`
	BlindWrites    []any      `json:"blind_writes"`
	ReadWrites     []rwEntry  `json:"read_writes"`
	ReadsOnly      []any      `json:"reads_only"`
	Endorsements   []any      `json:"endorsements"`
	Namespaces     []any      `json:"namespaces"`
}

type policyEntry struct {
	Namespace string `json:"namespace"`
	Version   int64  `json:"version"`
	Policy    string `json:"policy"`
}

type namespacePoliciesBody struct {
	Policies []policyEntry `json:"policies"`
}

// ── HTTP helper functions ─────────────────────────────────────────────────────

// requireJSON does a GET to url, asserts HTTP 200, and decodes the JSON body
// into dst using require (test fails immediately on any error).
func requireJSON(t *testing.T, url string, dst any) {
	t.Helper()
	//nolint:gosec,noctx // test-only: URL is always localhost
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable in tests
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s status", url)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, dst), "decode JSON from GET %s", url)
}

// assertJSON is the assert-flavoured variant used inside EventuallyWithT.
// Uses assert (not require) because ct is *assert.CollectT, not *testing.T.
func assertJSON(ct *assert.CollectT, url string, dst any) {
	//nolint:gosec,noctx // test-only: URL is always localhost
	resp, err := http.Get(url)
	if !assert.NoError(ct, err, "GET %s", url) { //nolint:testifylint // must use assert inside CollectT
		return
	}
	defer resp.Body.Close() //nolint:errcheck
	if !assert.Equal(ct, http.StatusOK, resp.StatusCode, "GET %s status", url) {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if !assert.NoError(ct, err) {
		return
	}
	assert.NoError(ct, json.Unmarshal(body, dst), "decode JSON from GET %s", url)
}

// TestExplorerWithCommitterTestNode is the main integration test.
//
// It mirrors scripts/test-live.sh exactly, covering all 12 REST smoke tests:
//  1. GET /blocks/height
//  2. GET /blocks?limit=5  — summary fields, block_size, created_at
//  3. GET /blocks/0        — config block detail
//  4. GET /blocks/1        — first application block detail
//  5. GET /transactions/{tx_id}   — from block 1
//  6. Write-set fields on the same transaction
//  7. GET /blocks pagination (limit + from)
//  8. GET /transactions/<unknown> → 404
//  9. Write-set variety across blocks 1..N
//
// 10. GET /namespaces/_meta/policies
// 11. GET /openapi.yaml
// 12. GET /docs (Swagger UI)
//
//nolint:paralleltest,gocognit,maintidx // integration: 12-subtest coverage justifies complexity.
func TestExplorerWithCommitterTestNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	t.Cleanup(cancel)

	// Step 1: start the committer test node container.
	sidecarPort := startTestNodeContainer(t)
	t.Logf("committer test node ready — sidecar mapped to host port %s", sidecarPort)

	// Step 2: provision a fresh DB for the explorer.
	dbConn := testdb.PrepareTestEnv(t)
	dbEndpoint := dbConn.Endpoints[0]

	// Step 3: find a free port for the explorer REST server.
	restPort := freePort(t)

	// Step 4: build the explorer config and start the service in-process.
	sidecarPortInt := mustParsePort(t, sidecarPort)
	cfg := &config.Config{
		DB: config.DBConfig{
			Endpoints: []*connection.Endpoint{dbEndpoint},
			User:      dbConn.User,
			Password:  dbConn.Password,
			DBName:    dbConn.Database,
		},
		Sidecar: config.SidecarConfig{
			Connection: connection.ClientConfig{
				Endpoint: &connection.Endpoint{Host: "localhost", Port: sidecarPortInt},
				TLS:      connection.TLSConfig{Mode: connection.NoneTLSMode},
			},
		},
		Buffer: config.BufferConfig{
			RawChannelSize:  config.DefaultRawChannelSize,
			ProcChannelSize: config.DefaultProcChannelSize,
		},
		Workers: config.WorkerConfig{
			ProcessorCount: config.DefaultProcessorCount,
			WriterCount:    config.DefaultWriterCount,
		},
		Server: config.ServerConfig{
			REST: config.RESTConfig{
				Endpoint: connection.Endpoint{Host: "127.0.0.1", Port: restPort},
			},
		},
	}

	svc := api.New(cfg)
	go func() {
		if err := svc.Run(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("explorer service exited unexpectedly: %v", err)
		}
	}()
	require.True(t, svc.WaitForReady(ctx), "explorer service did not become ready within timeout")

	base := fmt.Sprintf("http://127.0.0.1:%d", restPort)

	// Wait for the explorer REST server to be ready (mirrors wait_http in test-live.sh).
	t.Log("waiting for explorer REST server to be ready...")
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		resp, err := http.Get(base + "/blocks/height") //nolint:noctx
		if !assert.NoError(ct, err) {
			return
		}
		_ = resp.Body.Close()
		assert.Equal(ct, http.StatusOK, resp.StatusCode)
	}, 2*time.Minute, time.Second, "explorer REST server never became ready")

	// Wait for height > 0 (mirrors wait_height_gt0 in test-live.sh).
	t.Log("waiting for explorer to ingest at least one application block...")
	var initialHeight int64
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		var body blockHeightBody
		assertJSON(ct, base+"/blocks/height", &body)
		assert.Positive(ct, body.Height, "expected height > 0")
		initialHeight = body.Height
	}, 5*time.Minute, time.Second, "explorer never ingested any blocks")
	t.Logf("initial block height: %d", initialHeight)

	// ── Test 1: GET /blocks/height ──────────────────────────────────────────
	t.Run("1_GET_blocks_height", func(t *testing.T) {
		var body blockHeightBody
		requireJSON(t, base+"/blocks/height", &body)
		require.Positive(t, body.Height)
		t.Logf("height = %d", body.Height)
	})

	// ── Test 2: GET /blocks?limit=5 — summary fields ────────────────────────
	t.Run("2_GET_blocks_list", func(t *testing.T) {
		var body listBlocksBody
		requireJSON(t, base+"/blocks?limit=5", &body)
		require.NotEmpty(t, body.Blocks, "no blocks returned")

		// Find first application block (block_num > 0).
		for _, blk := range body.Blocks {
			if blk.BlockNum > 0 {
				require.NotZero(t, blk.BlockSize, "app block.block_size is missing")
				require.NotNil(t, blk.CreatedAt, "app block.created_at is missing")
				// Verify RFC 3339 format — Go's time.Time from JSON already guarantees this.
				t.Logf("app block.created_at = %s", blk.CreatedAt.Format(time.RFC3339))
				break
			}
		}

		// Config block (block 0) must always have block_size.
		for _, blk := range body.Blocks {
			if blk.BlockNum == 0 {
				require.NotZero(t, blk.BlockSize, "block0.block_size is missing")
				break
			}
		}
	})

	// ── Test 3: GET /blocks/0 — config block detail ─────────────────────────
	t.Run("3_GET_blocks_0", func(t *testing.T) {
		var body blockDetailBody
		requireJSON(t, base+"/blocks/0", &body)
		require.Equal(t, int64(0), body.BlockNum)
		require.NotNil(t, body.Transactions, "block0.transactions field missing")
	})

	// ── Test 4: GET /blocks/1 — first application block ─────────────────────
	t.Run("4_GET_blocks_1", func(t *testing.T) {
		var body blockDetailBody
		requireJSON(t, base+"/blocks/1", &body)
		require.Equal(t, int64(1), body.BlockNum)
		t.Logf("block1 has %d transaction(s)", len(body.Transactions))
	})

	// ── Test 5: GET /transactions/{tx_id} — pick tx from block 1 ───────────
	t.Run("5_GET_transaction_by_id", func(t *testing.T) {
		var blk1 blockDetailBody
		requireJSON(t, base+"/blocks/1", &blk1)
		require.NotEmpty(t, blk1.Transactions, "block 1 has no transactions")

		txID := blk1.Transactions[0].TxID
		t.Logf("using tx_id: %s", txID)

		var tx txDetailBody
		requireJSON(t, base+"/transactions/"+txID, &tx)
		require.Equal(t, txID, tx.TxID)
		require.NotEmpty(t, tx.ValidationCode)
		require.NotEmpty(t, tx.ChannelID)
		require.NotNil(t, tx.CreatedAt)
		// Nullable fields — just check they are present in the JSON.
		t.Logf("tx.validation_code = %s, tx.channel_id = %s", tx.ValidationCode, tx.ChannelID)
	})

	// ── Test 6: write-set fields on the same transaction ────────────────────
	t.Run("6_tx_write_set_fields", func(t *testing.T) {
		var blk1 blockDetailBody
		requireJSON(t, base+"/blocks/1", &blk1)
		require.NotEmpty(t, blk1.Transactions)

		txID := blk1.Transactions[0].TxID
		var tx txDetailBody
		requireJSON(t, base+"/transactions/"+txID, &tx)

		// All write-set arrays must be present (may be empty for _meta tx).
		require.NotNil(t, tx.BlindWrites, "tx.blind_writes missing")
		require.NotNil(t, tx.ReadWrites, "tx.read_writes missing")
		require.NotNil(t, tx.ReadsOnly, "tx.reads_only missing")
		require.NotNil(t, tx.Endorsements, "tx.endorsements missing")
		require.NotNil(t, tx.Namespaces, "tx.namespaces missing")

		// If read_writes is non-empty, first entry must have ns_id.
		if len(tx.ReadWrites) > 0 {
			require.NotEmpty(t, tx.ReadWrites[0].NsID, "read_writes[0].ns_id missing")
		}
	})

	// ── Test 7: pagination ───────────────────────────────────────────────────
	t.Run("7_GET_blocks_pagination", func(t *testing.T) {
		// Need height >= 2 for page1 (blocks 0,1) and page2 (block 2) to both exist.
		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			var h blockHeightBody
			assertJSON(ct, base+"/blocks/height", &h)
			assert.GreaterOrEqual(ct, h.Height, int64(2))
		}, 2*time.Minute, time.Second, "height never reached 2")

		var page1 listBlocksBody
		requireJSON(t, base+"/blocks?limit=2", &page1)
		require.NotEmpty(t, page1.Blocks)
		lastOnPage1 := page1.Blocks[len(page1.Blocks)-1].BlockNum

		var page2 listBlocksBody
		requireJSON(t, base+"/blocks?limit=2&from=2", &page2)
		require.NotEmpty(t, page2.Blocks, "page2 returned no blocks")
		firstOnPage2 := page2.Blocks[0].BlockNum

		require.NotEqual(t, lastOnPage1, firstOnPage2, "pagination: pages overlap")
		t.Logf("pagination: page1 last=%d, page2 first=%d", lastOnPage1, firstOnPage2)
	})

	// ── Test 8: unknown transaction → 404 ───────────────────────────────────
	t.Run("8_GET_transaction_unknown_404", func(t *testing.T) {
		unknownTxID := strings.Repeat("0", 64)
		resp, err := http.Get(base + "/transactions/" + unknownTxID) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// ── Test 9: write-set variety across multiple blocks ─────────────────────
	t.Run("9_write_set_variety", func(t *testing.T) {
		// Wait for height >= 5.
		var scanHeight int64
		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			var h blockHeightBody
			assertJSON(ct, base+"/blocks/height", &h)
			assert.GreaterOrEqual(ct, h.Height, int64(5))
			scanHeight = h.Height
		}, 2*time.Minute, 2*time.Second, "height never reached 5")

		scanMax := scanHeight
		if scanMax > 20 {
			scanMax = 20
		}
		t.Logf("scanning blocks 1..%d for write-set variety", scanMax)

		sawRW := false
		for blkN := int64(1); blkN <= scanMax; blkN++ {
			var blk blockDetailBody
			requireJSON(t, fmt.Sprintf("%s/blocks/%d", base, blkN), &blk)
			for _, tx := range blk.Transactions {
				if len(tx.ReadWrites) > 0 {
					sawRW = true
				}
			}
			if sawRW {
				break
			}
		}
		require.True(t, sawRW, "no read_writes seen in blocks 1..%d", scanMax)
	})

	// ── Test 10: GET /namespaces/_meta/policies ──────────────────────────────
	t.Run("10_GET_namespace_policies", func(t *testing.T) {
		var body namespacePoliciesBody
		requireJSON(t, base+"/namespaces/_meta/policies", &body)
		require.NotEmpty(t, body.Policies, "no policies returned for namespace _meta")

		pol := body.Policies[0]
		require.NotEmpty(t, pol.Namespace, "policy.namespace missing")
		require.NotEmpty(t, pol.Policy, "policy.policy missing")
		t.Logf("namespace=%s version=%d", pol.Namespace, pol.Version)
	})

	// ── Test 11: GET /openapi.yaml ───────────────────────────────────────────
	t.Run("11_GET_openapi_yaml", func(t *testing.T) {
		resp, err := http.Get(base + "/openapi.yaml") //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "openapi:", "openapi.yaml body missing 'openapi:' key")
	})

	// ── Test 12: GET /docs (Swagger UI) ─────────────────────────────────────
	t.Run("12_GET_docs_swagger_ui", func(t *testing.T) {
		resp, err := http.Get(base + "/docs") //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		bodyStr := strings.ToLower(string(body))
		require.True(t,
			strings.Contains(bodyStr, "swagger") ||
				strings.Contains(bodyStr, "openapi") ||
				strings.Contains(bodyStr, "redoc"),
			"GET /docs body missing expected Swagger/OpenAPI/ReDoc content",
		)
	})
}

// startTestNodeContainer starts the all-in-one committer test node Docker
// container, waits for the Docker health check to report "healthy", and
// returns the host-mapped sidecar port.
//
// The container name uses the sc_test_ prefix so that `make kill-test-docker`
// can clean it up when needed.
func startTestNodeContainer(t *testing.T) (sidecarHostPort string) {
	t.Helper()

	// Remove any leftover container from a previous run.
	_ = exec.Command("docker", "rm", "-f", testContainerName).Run()

	// Build the docker run argument list.
	// Port 4001 (sidecar) is mapped to a random host port to avoid conflicts.
	baseArgs := []string{
		"run", "-d",
		"--name", testContainerName,
		"-p", "127.0.0.1::" + sidecarContainerPort,
	}
	// TLS env vars — matches test-live.sh exactly (no --insecure flag needed).
	insecureTLSEnv := []string{
		"-e", "SC_COORDINATOR_SERVER_TLS_MODE=none",
		"-e", "SC_COORDINATOR_VERIFIER_TLS_MODE=none",
		"-e", "SC_COORDINATOR_VALIDATOR_COMMITTER_TLS_MODE=none",
		"-e", "SC_COORDINATOR_MONITORING_TLS_MODE=none",
		"-e", "SC_QUERY_SERVER_TLS_MODE=none",
		"-e", "SC_QUERY_MONITORING_TLS_MODE=none",
		"-e", "SC_SIDECAR_SERVER_TLS_MODE=none",
		"-e", "SC_SIDECAR_MONITORING_TLS_MODE=none",
		"-e", "SC_SIDECAR_COMMITTER_TLS_MODE=none",
		"-e", "SC_VC_SERVER_TLS_MODE=none",
		"-e", "SC_VC_MONITORING_TLS_MODE=none",
		"-e", "SC_VERIFIER_SERVER_TLS_MODE=none",
		"-e", "SC_SIDECAR_ORDERER_TLS_MODE=none",
		"-e", "SC_VERIFIER_MONITORING_TLS_MODE=none",
		"-e", "SC_SIDECAR_ORDERER_CONNECTION_TLS_MODE=none",
		"-e", "SC_ORDERER_SERVER_TLS_MODE=none",
		"-e", "SC_LOADGEN_SERVER_TLS_MODE=none",
		"-e", "SC_LOADGEN_MONITORING_TLS_MODE=none",
		"-e", "SC_LOADGEN_ORDERER_CLIENT_SIDECAR_CLIENT_TLS_MODE=none",
		"-e", "SC_LOADGEN_ORDERER_CLIENT_ORDERER_TLS_MODE=none",
	}
	args := make([]string, len(baseArgs), len(baseArgs)+len(insecureTLSEnv)+5)
	copy(args, baseArgs)
	args = append(args, insecureTLSEnv...)
	// CMD: run db committer orderer loadgen (TLS disabled via env vars above).
	args = append(args, testNodeImage(), "run", "db", "committer", "orderer", "loadgen")

	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, "docker run failed: %s", string(out))

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", testContainerName).Run()
	})

	// Retrieve the auto-assigned host port for the sidecar immediately after
	// the container starts; we need it before we can dial.
	portOut, err := exec.Command(
		"docker", "inspect",
		"--format", `{{(index (index .NetworkSettings.Ports "4001/tcp") 0).HostPort}}`,
		testContainerName,
	).Output()
	require.NoError(t, err, "failed to inspect sidecar mapped port")
	sidecarHostPort = strings.TrimSpace(string(portOut))

	// Wait for the sidecar gRPC port to be reachable rather than waiting for
	// the Docker healthcheck: the healthcheck becomes "unhealthy" once loadgen
	// finishes its fixed workload (~50 blocks), so we would miss the window.
	// A successful TCP dial to the sidecar port is sufficient to know the
	// committer stack (sidecar, orderer, loadgen) is ready to serve blocks.
	t.Logf("waiting for committer test node sidecar to be reachable on port %s...", sidecarHostPort)
	sidecarAddr := net.JoinHostPort("127.0.0.1", sidecarHostPort)
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		conn, dialErr := net.DialTimeout("tcp", sidecarAddr, time.Second)
		if assert.NoError(ct, dialErr) {
			_ = conn.Close()
		}
	}, 3*time.Minute, 2*time.Second, "sidecar port never became reachable")
	t.Log("committer test node sidecar is reachable")
	return sidecarHostPort
}

// freePort returns a random available TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port //nolint:forcetypeassert,errcheck
	err = ln.Close()
	require.NoError(t, err)
	return port
}

// mustParsePort parses a port string (e.g. "49152") into an int.
func mustParsePort(t *testing.T, portStr string) int {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", portStr))
	require.NoError(t, err)
	return addr.Port
}
