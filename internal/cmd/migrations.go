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
	dsn       string
	noHeaders bool
)

var migrationsCmd = &cobra.Command{
	Use:   "migrations",
	Short: "Run database migrations",
	Long:  "Run database migrations",
}

func init() {
	migrationsCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN connection string")
	_ = migrationsCmd.MarkPersistentFlagRequired("dsn")

	migrationsRollbackCmd.Flags().Int64("version", -1, "Target version to migrate down to (default: roll back one)")
	migrationsShowCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "Suppress column headers in tabular text output")

	migrationsCmd.AddCommand(migrationsApplyCmd)
	migrationsCmd.AddCommand(migrationsRollbackCmd)
	migrationsCmd.AddCommand(migrationsShowCmd)
	migrationsCmd.AddCommand(migrationsCheckCmd)

	rootCmd.AddCommand(migrationsCmd)
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
		return nil, fmt.Errorf("cannot open database handle: %w", err)
	}

	if err := db.PingContext(cmd.Context()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	return db, nil
}

// --- migrations apply ---

var migrationsApplyCmd = &cobra.Command{
	Use:          "apply",
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
				return MigrateResultsResult{}, fmt.Errorf("cannot create goose provider: %w", err)
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

// --- migrations rollback ---

var migrationsRollbackCmd = &cobra.Command{
	Use:          "rollback",
	Short:        "Roll back migration(s)",
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
				return MigrateResultsResult{}, fmt.Errorf("cannot create goose provider: %w", err)
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

// --- migrations show ---

var migrationsShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show migration status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunHandler(cmd, formatMigrationShow, func(ctx context.Context) (MigrationShowResult, error) {
			db, err := openMigrateDB(cmd)
			if err != nil {
				return MigrationShowResult{}, err
			}
			defer func() { _ = db.Close() }()

			provider, err := newGooseProvider(db, GetFormat(cmd) == "json")
			if err != nil {
				return MigrationShowResult{}, fmt.Errorf("cannot create goose provider: %w", err)
			}

			statuses, err := provider.Status(ctx)
			if err != nil {
				return MigrationShowResult{}, err
			}

			return MigrationShowResult{
				Statuses:  statuses,
				NoHeaders: noHeaders,
			}, nil
		})
	},
}

// --- migrations check ---

var migrationsCheckCmd = &cobra.Command{
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
				return CheckResult{}, fmt.Errorf("cannot create goose provider: %w", err)
			}

			hasPending, err := provider.HasPending(ctx)
			if err != nil {
				return CheckResult{}, fmt.Errorf("cannot check pending migrations: %w", err)
			}

			current, versionErr := provider.GetDBVersion(ctx)

			result := CheckResult{Version: current}
			switch {
			case hasPending && versionErr != nil:
				return CheckResult{}, fmt.Errorf("migrations are pending (cannot get current version: %v)", versionErr)
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
