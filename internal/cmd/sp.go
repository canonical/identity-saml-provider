// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/app"
	"github.com/canonical/identity-saml-provider/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

var spCmd = &cobra.Command{
	Use:   "sp",
	Short: "Manage SAML service providers",
	Long:  "Register, list, and manage SAML service providers.",
}

func init() {
	rootCmd.AddCommand(spCmd)
}

// openDB opens a pgxpool connection using the application config.
func openDB(ctx context.Context, cfg app.Config) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, cfg.PoolConfig())
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}
