/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hyperledger/fabric-x-committer/utils/connection"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/api"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/config"
)

const (
	explorerName    = "Block Explorer"
	explorerVersion = "0.1.0"
)

// VersionCmd prints the explorer version.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        fmt.Sprintf("Print %s version.", explorerName),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("%s version %s %s/%s\n", explorerName, explorerVersion, runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

// StartExplorerCMD starts the block explorer API service (gRPC + REST).
func StartExplorerCMD(use string) *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Starts the %s.", explorerName),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromFile(configPath)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			cmd.Printf("Starting %s\n", explorerName)
			defer cmd.Printf("%s ended\n", explorerName)

			svc := api.New(cfg)
			return connection.StartService(cmd.Context(), svc, cfg.Server.GRPC)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}
