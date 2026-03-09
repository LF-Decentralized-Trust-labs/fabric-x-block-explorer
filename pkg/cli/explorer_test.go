/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartExplorerCMD_ValidatesConfigBeforeStartup(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
database:
  endpoints:
    - host: localhost
      port: 5432
  user: postgres
sidecar:
  connection:
    endpoint:
      host: localhost
      port: 7052
  channel_id: mychannel
workers:
  processor_count: 4
  writer_count: 4
`), 0o600))

	cmd := StartExplorerCMD("start")
	cmd.SetArgs([]string{"--config", configPath})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "database name is required")
}