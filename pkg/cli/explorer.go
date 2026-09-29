/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hyperledger/fabric-lib-go/common/flogging"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/api"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

var cliLogger = flogging.MustGetLogger("cli")

// knownWeakPasswords is a set of well-known default passwords operators
// frequently forget to change before going to production.
var knownWeakPasswords = map[string]struct{}{
	"":         {},
	"postgres": {},
	"password": {},
	"changeme": {},
	"secret":   {},
	"admin":    {},
	"1234":     {},
	"12345678": {},
}

// warnWeakSecrets logs a warning when the database password is a known weak
// default. It is not an error — the service still starts — but the message is
// hard to miss in logs so operators notice before going live.
func warnWeakSecrets(cfg *config.Config) {
	if _, weak := knownWeakPasswords[cfg.DB.Password]; weak {
		cliLogger.Warn("⚠️  database.password is a well-known default value — " +
			"change it before deploying to production")
	}
}

const explorerName = "Block Explorer"

// Version is set at build time via -ldflags "-X ...cli.Version=x.y.z".
// Falls back to "dev" when running without build flags.
var Version = "dev"

// VersionCmd prints the explorer version.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        fmt.Sprintf("Print %s version.", explorerName),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("%s version %s %s/%s\n", explorerName, Version, runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

// StartExplorerCMD starts the block explorer REST API service.
func StartExplorerCMD(use string) *cobra.Command {
	var configPath string
	var envOnly bool
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Starts the %s.", explorerName),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var (
				cfg *config.Config
				err error
			)
			if envOnly {
				cfg, err = config.LoadFromEnv()
			} else {
				cfg, err = config.LoadFromFile(configPath)
			}
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			warnWeakSecrets(cfg)
			cmd.SilenceUsage = true
			cmd.Printf("Starting %s\n", explorerName)
			defer cmd.Printf("%s ended\n", explorerName)

			svc := api.New(cfg)
			return svc.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	cmd.Flags().BoolVar(&envOnly, "env-only", false,
		"load configuration entirely from EXPLORER_* environment variables (no config file)")
	return cmd
}
