// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"github.com/spf13/cobra"
)

var (
	janitorFormat string
)

var janitorCmd = &cobra.Command{
	Use:   "janitor",
	Short: "Database janitor utility",
	Long:  "Database janitor utility containing subcommands to prune expired resources.",
}

func init() {
	janitorCmd.PersistentFlags().StringVarP(
		&janitorFormat, "format", "f", "text",
		"Output format (text or json)",
	)
	rootCmd.AddCommand(janitorCmd)
}
