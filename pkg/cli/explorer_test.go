/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
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

func TestWarnWeakSecrets(t *testing.T) {
	t.Parallel()

	weakPasswords := []string{"postgres", "password", "changeme", "secret", "admin", "", "1234", "12345678"}
	for _, pw := range weakPasswords {
		pw := pw
		t.Run("weak_"+pw, func(t *testing.T) {
			t.Parallel()
			// warnWeakSecrets must not panic and must recognise the password as weak.
			// We verify indirectly: the password appears in knownWeakPasswords.
			_, isWeak := knownWeakPasswords[pw]
			assert.True(t, isWeak, "expected %q to be in knownWeakPasswords", pw)

			// Calling the function must not panic even with a minimal config.
			cfg := &config.Config{}
			cfg.DB.Password = pw
			assert.NotPanics(t, func() { warnWeakSecrets(cfg) })
		})
	}

	t.Run("strong_password_not_in_map", func(t *testing.T) {
		t.Parallel()
		_, isWeak := knownWeakPasswords["s3cureP@ssw0rd!"]
		assert.False(t, isWeak)

		cfg := &config.Config{}
		cfg.DB.Password = "s3cureP@ssw0rd!"
		assert.NotPanics(t, func() { warnWeakSecrets(cfg) })
	})
}
