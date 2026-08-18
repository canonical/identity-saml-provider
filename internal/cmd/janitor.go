// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"github.com/spf13/cobra"
)

var janitorCmd = &cobra.Command{
	Use:   "janitor",
	Short: "Database janitor utility",
	Long:  "Database janitor utility containing subcommands to prune expired resources.",
}

func init() {
	rootCmd.AddCommand(janitorCmd)
}
