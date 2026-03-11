/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-committer/utils/connection"
)

// validCfg returns a minimal Config that passes Validate.
func validCfg() Config {
	return Config{
		DB: DBConfig{
			Endpoints: []*connection.Endpoint{{Host: "localhost", Port: 5432}},
			User:      "postgres",
			DBName:    "explorer",
		},
		Sidecar: SidecarConfig{
			Connection: connection.ClientConfig{
				Endpoint: &connection.Endpoint{Host: "localhost", Port: 7052},
			},
			ChannelID: "mychannel",
		},
		Workers: WorkerConfig{
			ProcessorCount: DefaultProcessorCount,
			WriterCount:    DefaultWriterCount,
		},
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:    "valid config",
			mutate:  func(*Config) {},
			wantErr: "",
		},
		{
			name:    "no endpoints",
			mutate:  func(c *Config) { c.DB.Endpoints = nil },
			wantErr: "database endpoints must not be empty",
		},
		{
			name:    "endpoint missing host",
			mutate:  func(c *Config) { c.DB.Endpoints[0].Host = "" },
			wantErr: "database endpoint host is required",
		},
		{
			name:    "endpoint port zero",
			mutate:  func(c *Config) { c.DB.Endpoints[0].Port = 0 },
			wantErr: "database endpoint port must be between 1 and 65535",
		},
		{
			name:    "endpoint port too large",
			mutate:  func(c *Config) { c.DB.Endpoints[0].Port = 70000 },
			wantErr: "database endpoint port must be between 1 and 65535",
		},
		{
			name:    "missing db user",
			mutate:  func(c *Config) { c.DB.User = "" },
			wantErr: "database user is required",
		},
		{
			name:    "missing db name",
			mutate:  func(c *Config) { c.DB.DBName = "" },
			wantErr: "database name is required",
		},
		{
			name:    "nil sidecar endpoint",
			mutate:  func(c *Config) { c.Sidecar.Connection.Endpoint = nil },
			wantErr: "sidecar endpoint host is required",
		},
		{
			name:    "empty sidecar host",
			mutate:  func(c *Config) { c.Sidecar.Connection.Endpoint.Host = "" },
			wantErr: "sidecar endpoint host is required",
		},
		{
			name:    "invalid sidecar port",
			mutate:  func(c *Config) { c.Sidecar.Connection.Endpoint.Port = 70000 },
			wantErr: "sidecar endpoint port must be between 1 and 65535",
		},
		{
			name:    "missing channel ID",
			mutate:  func(c *Config) { c.Sidecar.ChannelID = "" },
			wantErr: "sidecar channel ID is required",
		},
		{
			name:    "processor count zero",
			mutate:  func(c *Config) { c.Workers.ProcessorCount = 0 },
			wantErr: "processor count must be greater than 0",
		},
		{
			name:    "writer count negative",
			mutate:  func(c *Config) { c.Workers.WriterCount = -1 },
			wantErr: "writer count must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validCfg()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// writeTempConfig writes yaml content to a temp file and returns the path.
func writeTempConfig(t *testing.T, yaml string) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(yaml), 0o600))
	return p
}

const minimalYAML = `
database:
  endpoints:
    - host: dbhost
      port: 5433
  user: dbuser
  password: secret
  dbname: mydb
sidecar:
  connection:
    endpoint:
      host: sidecarhost
      port: 7053
  channel_id: testchannel
  start_block: 5
  end_block: 100
buffer:
  raw_channel_size: 300
  proc_channel_size: 400
workers:
  processor_count: 8
  writer_count: 6
`

func TestLoadFromFile(t *testing.T) {
	t.Parallel()

	t.Run("parses all fields from YAML", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadFromFile(writeTempConfig(t, minimalYAML))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		require.Len(t, cfg.DB.Endpoints, 1)
		assert.Equal(t, "dbhost", cfg.DB.Endpoints[0].Host)
		assert.Equal(t, 5433, cfg.DB.Endpoints[0].Port)
		assert.Equal(t, "dbuser", cfg.DB.User)
		assert.Equal(t, "secret", cfg.DB.Password)
		assert.Equal(t, "mydb", cfg.DB.DBName)

		require.NotNil(t, cfg.Sidecar.Connection.Endpoint)
		assert.Equal(t, "sidecarhost", cfg.Sidecar.Connection.Endpoint.Host)
		assert.Equal(t, 7053, cfg.Sidecar.Connection.Endpoint.Port)
		assert.Equal(t, "testchannel", cfg.Sidecar.ChannelID)
		assert.Equal(t, uint64(5), cfg.Sidecar.StartBlk)
		assert.Equal(t, uint64(100), cfg.Sidecar.EndBlk)

		assert.Equal(t, 300, cfg.Buffer.RawChannelSize)
		assert.Equal(t, 400, cfg.Buffer.ProcChannelSize)
		assert.Equal(t, 8, cfg.Workers.ProcessorCount)
		assert.Equal(t, 6, cfg.Workers.WriterCount)
	})

	t.Run("applies defaults for unset fields", func(t *testing.T) {
		t.Parallel()
		const sparse = `
database:
  endpoints:
    - host: localhost
      port: 5432
  user: u
  dbname: d
sidecar:
  connection:
    endpoint:
      host: localhost
      port: 7052
  channel_id: ch
`
		cfg, err := LoadFromFile(writeTempConfig(t, sparse))
		require.NoError(t, err)

		assert.Equal(t, DefaultRawChannelSize, cfg.Buffer.RawChannelSize)
		assert.Equal(t, DefaultProcChannelSize, cfg.Buffer.ProcChannelSize)
		assert.Equal(t, DefaultProcessorCount, cfg.Workers.ProcessorCount)
		assert.Equal(t, DefaultWriterCount, cfg.Workers.WriterCount)
		assert.Equal(t, int32(DefaultDBMaxConns), cfg.DB.MaxConns)
		assert.Equal(t, ^uint64(0), cfg.Sidecar.EndBlk)
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromFile("/nonexistent/path/config.yaml")
		assert.Error(t, err)
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromFile(writeTempConfig(t, `invalid: [unclosed`))
		assert.Error(t, err)
	})
}
