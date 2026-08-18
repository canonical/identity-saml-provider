// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
)

var rootCmd = &cobra.Command{
	Use:           "identity-saml-provider",
	Short:         "SAML-to-OIDC bridge provider",
	Long:          "Identity SAML Provider - a SAML-to-OIDC bridge that translates SAML authentication requests to OIDC flows via Ory Hydra.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if outputFormat != "text" && outputFormat != "json" {
			return fmt.Errorf("unsupported output format: %q", outputFormat)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&outputFormat, "format", "text",
		"Output format (text or json)",
	)
}

func Execute() {
	if err := ExecuteRoot(rootCmd); err != nil {
		os.Exit(1)
	}
}
