// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/identity-saml-provider/migrations"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

var (
	dsn string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  "Run database migrations",
}

func init() {
	migrateCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN connection string")
	_ = migrateCmd.MarkPersistentFlagRequired("dsn")

	migrateDownCmd.Flags().Int64("version", -1, "Target version to migrate down to (default: roll back one)")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateCheckCmd)

	rootCmd.AddCommand(migrateCmd)
}

func newGooseProvider(db *sql.DB, isJSON bool) (*goose.Provider, error) {
	var opts []goose.ProviderOption
	if isJSON {
		opts = append(opts, goose.WithLogger(goose.NopLogger()))
	}

	return goose.NewProvider(goose.DialectPostgres, db, migrations.EmbedMigrations, opts...)
}

func openMigrateDB(cmd *cobra.Command) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database handle: %w", err)
	}

	if err := db.PingContext(cmd.Context()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// --- migrate up ---

var migrateUpCmd = &cobra.Command{
	Use:          "up",
	Short:        "Apply all pending migrations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, nil, func(ctx context.Context) (MigrateResultsResult, error) {
			db, err := openMigrateDB(cmd)
			if err != nil {
				return MigrateResultsResult{}, err
			}
			defer func() { _ = db.Close() }()

			provider, err := newGooseProvider(db, GetFormat(cmd) == "json")
			if err != nil {
				return MigrateResultsResult{}, fmt.Errorf("failed to create goose provider: %w", err)
			}

			results, err := provider.Up(ctx)
			if err != nil {
				return MigrateResultsResult{}, err
			}
			if results == nil {
				results = []*goose.MigrationResult{}
			}

			return MigrateResultsResult{Applied: results}, nil
		})
	},
}

// --- migrate down ---

var migrateDownCmd = &cobra.Command{
	Use:          "down",
	Short:        "Roll back the last migration",
	Long:         "Roll back the last migration, or down to a specific version with --version",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, nil, func(ctx context.Context) (MigrateResultsResult, error) {
			db, err := openMigrateDB(cmd)
			if err != nil {
				return MigrateResultsResult{}, err
			}
			defer func() { _ = db.Close() }()

			provider, err := newGooseProvider(db, GetFormat(cmd) == "json")
			if err != nil {
				return MigrateResultsResult{}, fmt.Errorf("failed to create goose provider: %w", err)
			}

			version, _ := cmd.Flags().GetInt64("version")
			var results []*goose.MigrationResult

			if version < 0 {
				result, err := provider.Down(ctx)
				if err != nil {
					return MigrateResultsResult{}, err
				}
				results = append(results, result)
			} else {
				var err error
				results, err = provider.DownTo(ctx, version)
				if err != nil {
					return MigrateResultsResult{}, err
				}
			}
			if results == nil {
				results = []*goose.MigrationResult{}
			}

			return MigrateResultsResult{Applied: results}, nil
		})
	},
}

// --- migrate status ---

var migrateStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show migration status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatMigrationStatuses, func(ctx context.Context) ([]*goose.MigrationStatus, error) {
			db, err := openMigrateDB(cmd)
			if err != nil {
				return nil, err
			}
			defer func() { _ = db.Close() }()

			provider, err := newGooseProvider(db, GetFormat(cmd) == "json")
			if err != nil {
				return nil, fmt.Errorf("failed to create goose provider: %w", err)
			}

			statuses, err := provider.Status(ctx)
			if err != nil {
				return nil, err
			}

			return statuses, nil
		})
	},
}

// --- migrate check ---

var migrateCheckCmd = &cobra.Command{
	Use:          "check",
	Short:        "Check if there are pending migrations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatMigrationCheck, func(ctx context.Context) (CheckResult, error) {
			db, err := openMigrateDB(cmd)
			if err != nil {
				return CheckResult{}, err
			}
			defer func() { _ = db.Close() }()

			provider, err := newGooseProvider(db, GetFormat(cmd) == "json")
			if err != nil {
				return CheckResult{}, fmt.Errorf("failed to create goose provider: %w", err)
			}

			hasPending, err := provider.HasPending(ctx)
			if err != nil {
				return CheckResult{}, fmt.Errorf("failed to check pending migrations: %w", err)
			}

			current, versionErr := provider.GetDBVersion(ctx)

			result := CheckResult{Version: current}
			switch {
			case hasPending && versionErr != nil:
				return CheckResult{}, fmt.Errorf("migrations are pending (failed to get current version: %v)", versionErr)
			case hasPending:
				result.Status = CheckStatusPending
			case versionErr != nil:
				result.Status = CheckStatusUnknown
			default:
				result.Status = CheckStatusOK
			}

			return result, nil
		})
	},
}
