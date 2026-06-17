/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/cli"
)

func main() {
	root := &cobra.Command{
		Use:          "explorer",
		Short:        "Fabric-X Block Explorer",
		SilenceUsage: true,
	}

	root.AddCommand(cli.StartExplorerCMD("start"))
	root.AddCommand(cli.VersionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
