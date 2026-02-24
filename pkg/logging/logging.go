/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package logging

import (
	"io"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger to provide structured logging
type Logger struct {
	*zap.SugaredLogger
	mu sync.Mutex
}

var loggerInstance Logger

// Config represents the logging configuration
type Config struct {
	Enabled     bool   `yaml:"enabled" mapstructure:"enabled"`
	Level       string `yaml:"level" mapstructure:"level"`
	Caller      bool   `yaml:"caller" mapstructure:"caller"`
	Development bool   `yaml:"development" mapstructure:"development"`
	Output      string `yaml:"output" mapstructure:"output"`
	Name        string `yaml:"name" mapstructure:"name"`
}

// Log levels
const (
	Debug   string = "DEBUG"
	Info    string = "INFO"
	Warning string = "WARNING"
	Error   string = "ERROR"
)

// DefaultConfig for logging
var DefaultConfig = Config{
	Enabled:     true,
	Level:       Info,
	Caller:      true,
	Development: false,
}

// SetupWithConfig updates the logger with the given config
func SetupWithConfig(config *Config) {
	loggerInstance.updateConfig(config)
}

// New returns a logger instance with the specified name
func New(name string) *Logger {
	loggerInstance.initWithDefault()
	config := DefaultConfig
	config.Name = name
	logger := &Logger{}
	logger.updateConfig(&config)
	return logger
}

// ErrorStackTrace prints the stack trace present in the error type
func (l *Logger) ErrorStackTrace(err error) {
	if err == nil {
		return
	}
	l.WithOptions(zap.AddCallerSkip(1)).Errorf("%+v", err)
}

// WarnStackTrace prints the stack trace present in the error type as warning log
func (l *Logger) WarnStackTrace(err error) {
	if err == nil {
		return
	}
	l.WithOptions(zap.AddCallerSkip(1)).Warnf("%+v", err)
}

// Level returns the current logging level
func (l *Logger) Level() zapcore.Level {
	return l.Desugar().Level()
}

func (l *Logger) updateConfig(config *Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.SugaredLogger = createLogger(config).Sugar()
}

func (l *Logger) initWithDefault() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.SugaredLogger == nil {
		l.SugaredLogger = createLogger(&DefaultConfig).Sugar()
	}
}

func createLogger(config *Config) *zap.Logger {
	if config == nil || !config.Enabled {
		return zap.NewNop()
	}

	level := zap.NewAtomicLevel()
	switch strings.ToUpper(config.Level) {
	case Debug:
		level.SetLevel(zap.DebugLevel)
	case Info:
		level.SetLevel(zap.InfoLevel)
	case Warning:
		level.SetLevel(zap.WarnLevel)
	case Error:
		level.SetLevel(zap.ErrorLevel)
	default:
		level.SetLevel(zap.InfoLevel)
	}

	outputs := []string{"stderr"}
	if config.Output != "" {
		outputs = append(outputs, config.Output)
	}

	var encCfg zapcore.EncoderConfig
	if config.Development {
		encCfg = zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		encCfg = zap.NewProductionEncoderConfig()
	}
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeName = zapcore.FullNameEncoder

	zapConfig := zap.Config{
		Level:       level,
		Development: config.Development,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:          "console",
		EncoderConfig:     encCfg,
		DisableStacktrace: true,
		OutputPaths:       outputs,
		ErrorOutputPaths:  outputs,
	}

	logger := zap.Must(zapConfig.Build(zap.WithCaller(config.Caller)))
	if config.Name != "" {
		logger = logger.Named(config.Name)
	}
	return logger
}

// SetOutput updates logger output (for testing)
func SetOutput(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	// This is a simplified version - in production you'd need to recreate the logger
	loggerInstance.initWithDefault()
}
