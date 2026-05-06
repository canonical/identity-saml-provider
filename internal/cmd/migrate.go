package cmd

import (
	"database/sql"
	"fmt"

	"github.com/canonical/identity-saml-provider/migrations"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

var (
	dsn           string
	migrateFormat string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  "Run database migrations",
}

func init() {
	migrateCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN connection string")
	migrateCmd.PersistentFlags().StringVarP(&migrateFormat, "format", "f", "text", "Output format (text or json)")
	_ = migrateCmd.MarkPersistentFlagRequired("dsn")

	migrateDownCmd.Flags().Int64("version", -1, "Target version to migrate down to (default: roll back one)")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateCheckCmd)

	rootCmd.AddCommand(migrateCmd)
}

func newGooseProvider(db *sql.DB, formatter MigrateOutputFormatter) (*goose.Provider, error) {
	var opts []goose.ProviderOption
	if formatter.ShouldSilenceGoose() {
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
		db.Close()
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
		formatter, err := newMigrateFormatter(migrateFormat)
		if err != nil {
			return err
		}

		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db, formatter)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		results, err := provider.Up(cmd.Context())
		if err != nil {
			return err
		}

		return formatter.MigrationResults(cmd.OutOrStdout(), results)
	},
}

// --- migrate down ---

var migrateDownCmd = &cobra.Command{
	Use:          "down",
	Short:        "Roll back the last migration",
	Long:         "Roll back the last migration, or down to a specific version with --version",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		formatter, err := newMigrateFormatter(migrateFormat)
		if err != nil {
			return err
		}

		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db, formatter)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		ctx := cmd.Context()
		version, _ := cmd.Flags().GetInt64("version")

		var results []*goose.MigrationResult

		if version < 0 {
			result, err := provider.Down(ctx)
			if err != nil {
				return err
			}
			results = append(results, result)
		} else {
			var err error
			results, err = provider.DownTo(ctx, version)
			if err != nil {
				return err
			}
		}

		return formatter.MigrationResults(cmd.OutOrStdout(), results)
	},
}

// --- migrate status ---

var migrateStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show migration status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		formatter, err := newMigrateFormatter(migrateFormat)
		if err != nil {
			return err
		}

		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db, formatter)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		statuses, err := provider.Status(cmd.Context())
		if err != nil {
			return err
		}

		return formatter.MigrationStatuses(cmd.OutOrStdout(), statuses)
	},
}

// --- migrate check ---

var migrateCheckCmd = &cobra.Command{
	Use:          "check",
	Short:        "Check if there are pending migrations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		formatter, err := newMigrateFormatter(migrateFormat)
		if err != nil {
			return err
		}

		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db, formatter)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		ctx := cmd.Context()
		out := cmd.OutOrStdout()

		hasPending, err := provider.HasPending(ctx)
		if err != nil {
			return fmt.Errorf("failed to check pending migrations: %w", err)
		}

		current, versionErr := provider.GetDBVersion(ctx)

		result := CheckResult{Version: current}
		switch {
		case hasPending && versionErr != nil:
			return fmt.Errorf("migrations are pending (failed to get current version: %v)", versionErr)
		case hasPending:
			result.Status = CheckStatusPending
		case versionErr != nil:
			result.Status = CheckStatusUnknown
		default:
			result.Status = CheckStatusOK
		}

		if err := formatter.MigrationCheck(out, result); err != nil {
			return err
		}

		return nil
	},
}
