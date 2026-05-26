/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-x-committer/utils/connection"
	"github.com/hyperledger/fabric-x-committer/utils/dbconn"
	"github.com/hyperledger/fabric-x-committer/utils/retry"
)

// DBConfig holds PostgreSQL connection configuration.
type DBConfig struct {
	User            string                   `mapstructure:"user"      yaml:"user"`
	Password        string                   `mapstructure:"password"  yaml:"password"` //nolint:gosec
	DBName          string                   `mapstructure:"dbname"    yaml:"dbname"`
	Endpoints       []*connection.Endpoint   `mapstructure:"endpoints" yaml:"endpoints"`
	TLS             dbconn.DatabaseTLSConfig `mapstructure:"tls"       yaml:"tls"`
	MaxConns        int32                    `mapstructure:"max_conns" yaml:"max_conns"`
	MaxConnIdleTime time.Duration            `mapstructure:"max_conn_idle_time" yaml:"max_conn_idle_time"`
	MaxConnLifetime time.Duration            `mapstructure:"max_conn_lifetime"  yaml:"max_conn_lifetime"`
	// Retry controls exponential back-off when the initial DB connection fails.
	Retry retry.Profile `mapstructure:"retry" yaml:"retry"`
}

// SidecarConfig holds fabric-x sidecar connection configuration.
// Reconnection on stream drops is handled automatically by the delivercommitter
// layer (via retry.Sustain). gRPC-level retry is configured via Connection.Retry.
//
// TLS modes (Connection.TLS.Mode):
//
//	"none"  — plaintext gRPC (default, no certificates required).
//	"tls"   — server-side TLS: client verifies the sidecar's certificate.
//	          Requires Connection.TLS.CACertPaths to point to the sidecar CA cert(s).
//	"mtls"  — mutual TLS: both sides verify each other.
//	          Requires CACertPaths (sidecar CA) + CertPath + KeyPath (client cert/key).
type SidecarConfig struct {
	Connection connection.ClientConfig `mapstructure:"connection"  yaml:"connection"`
	StartBlk   uint64                  `mapstructure:"start_block" yaml:"start_block"`
}

// BufferConfig controls channel buffer sizes between pipeline stages.
type BufferConfig struct {
	RawChannelSize  int `mapstructure:"raw_channel_size"  yaml:"raw_channel_size"`
	ProcChannelSize int `mapstructure:"proc_channel_size" yaml:"proc_channel_size"`
}

// WorkerConfig controls the number of goroutines at each pipeline stage.
type WorkerConfig struct {
	ProcessorCount int `mapstructure:"processor_count" yaml:"processor_count"`
	WriterCount    int `mapstructure:"writer_count"    yaml:"writer_count"`
}

// ServerConfig holds the REST API server configuration.
type ServerConfig struct {
	REST RESTConfig `mapstructure:"rest" yaml:"rest"`
}

// RESTConfig holds the REST server endpoint and configuration.
type RESTConfig struct {
	Endpoint          connection.Endpoint `mapstructure:"endpoint"           yaml:"endpoint"`
	ReadHeaderTimeout time.Duration       `mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	// ReadTimeout is the maximum duration for reading the entire HTTP request
	// (including body). A zero value uses the package default.
	ReadTimeout time.Duration `mapstructure:"read_timeout"    yaml:"read_timeout"`
	// WriteTimeout is the maximum duration before timing out a response write.
	// A zero value uses the package default.
	WriteTimeout time.Duration `mapstructure:"write_timeout"   yaml:"write_timeout"`
	// ShutdownTimeout is how long the server waits for in-flight requests to
	// drain before forcing a close. A zero value uses the package default.
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	DefaultTxLimit  int32         `mapstructure:"default_tx_limit" yaml:"default_tx_limit"`
}

// Config is the top-level application configuration.
type Config struct {
	DB      DBConfig      `mapstructure:"database" yaml:"database"`
	Sidecar SidecarConfig `mapstructure:"sidecar"  yaml:"sidecar"`
	Buffer  BufferConfig  `mapstructure:"buffer"   yaml:"buffer"`
	Workers WorkerConfig  `mapstructure:"workers"  yaml:"workers"`
	Server  ServerConfig  `mapstructure:"server"   yaml:"server"`
}

// envPrefix is the environment-variable prefix for all config overrides.
// Example: EXPLORER_DATABASE_PASSWORD overrides database.password.
const envPrefix = "EXPLORER"

// LoadFromFile reads a YAML config file at path into Config and applies defaults.
// Any field may be overridden by an environment variable of the form
// EXPLORER_<KEY> where <KEY> is the YAML path with dots and hyphens replaced by
// underscores (e.g. EXPLORER_DATABASE_PASSWORD, EXPLORER_SIDECAR_START_BLOCK).
func LoadFromFile(path string) (*Config, error) {
	v := newViperWithDefaults()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFromEnv reads configuration entirely from environment variables.
// Useful when no config file is desired (e.g. container deployments).
func LoadFromEnv() (*Config, error) {
	v := newViperWithDefaults()
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// envKeyReplacer converts viper key characters to env-var-safe underscores.
var envKeyReplacer = strings.NewReplacer(".", "_", "-", "_")

// Validate returns an error if any required field is missing or out of range.
func (c *Config) Validate() error {
	if err := c.DB.validate(); err != nil {
		return err
	}
	if err := c.Sidecar.validate(); err != nil {
		return err
	}
	if c.Workers.ProcessorCount <= 0 {
		return errors.New("processor count must be greater than 0")
	}
	if c.Workers.WriterCount <= 0 {
		return errors.New("writer count must be greater than 0")
	}
	if c.DB.MaxConns > 0 && c.Workers.WriterCount > int(c.DB.MaxConns) {
		return errors.Newf("writer_count (%d) must not exceed database max_conns (%d)",
			c.Workers.WriterCount, c.DB.MaxConns)
	}
	return nil
}

func (d *DBConfig) validate() error {
	if len(d.Endpoints) == 0 {
		return errors.New("database endpoints must not be empty")
	}
	for _, ep := range d.Endpoints {
		if ep.Host == "" {
			return errors.New("database endpoint host is required")
		}
		if err := validatePort(ep.Port, "database endpoint"); err != nil {
			return err
		}
	}
	if d.User == "" {
		return errors.New("database user is required")
	}
	if d.DBName == "" {
		return errors.New("database name is required")
	}
	return nil
}

func (s *SidecarConfig) validate() error {
	if s.Connection.Endpoint == nil || s.Connection.Endpoint.Host == "" {
		return errors.New("sidecar endpoint host is required")
	}
	if err := validatePort(s.Connection.Endpoint.Port, "sidecar endpoint"); err != nil {
		return err
	}
	return validateTLSMode(s.Connection.TLS.Mode)
}

// validatePort returns an error if port is outside the valid TCP range.
func validatePort(port int, subject string) error {
	if port <= 0 || port > 65535 {
		return errors.Newf("%s port must be between 1 and 65535", subject)
	}
	return nil
}

// validateTLSMode returns an error if mode is not one of the recognised values.
func validateTLSMode(mode string) error {
	switch mode {
	case connection.NoneTLSMode, connection.OneSideTLSMode, connection.MutualTLSMode, connection.UnmentionedTLSMode:
		return nil
	default:
		return errors.Newf(
			"invalid sidecar TLS mode %q: must be one of %q, %q, %q",
			mode, connection.NoneTLSMode, connection.OneSideTLSMode, connection.MutualTLSMode,
		)
	}
}
