package cmd

import (
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the application version",
	Long:  "Print the version of the identity-saml-provider application.",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), version.Version)
		return err
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
