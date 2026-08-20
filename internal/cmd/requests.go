// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"github.com/spf13/cobra"
)

var requestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Manage SAML pending authentication requests",
	Long:  "Manage SAML pending authentication requests stored in the database.",
}

func init() {
	rootCmd.AddCommand(requestsCmd)
}
