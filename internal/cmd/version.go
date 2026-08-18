// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"

	"github.com/canonical/identity-saml-provider/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the application version",
	Long:  "Print the version of the identity-saml-provider application.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatVersion, func(ctx context.Context) (VersionResult, error) {
			return VersionResult{Version: version.Version}, nil
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
