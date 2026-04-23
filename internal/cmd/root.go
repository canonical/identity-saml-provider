package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "identity-saml-provider",
	Short: "SAML-to-OIDC bridge provider",
	Long:  "Identity SAML Provider - a SAML-to-OIDC bridge that translates SAML authentication requests to OIDC flows via Ory Hydra.",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
