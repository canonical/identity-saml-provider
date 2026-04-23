package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/canonical/identity-saml-provider/migrations"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

var (
	dsn    string
	format string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  "Run database migrations",
}

func init() {
	migrateCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN connection string")
	migrateCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "Output format (text or json)")
	_ = migrateCmd.MarkPersistentFlagRequired("dsn")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateCheckCmd)

	rootCmd.AddCommand(migrateCmd)
}

func newGooseProvider(db *sql.DB) (*goose.Provider, error) {
	goose.SetBaseFS(migrations.EmbedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}

	var opts []goose.ProviderOption
	if format == "json" {
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
		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		results, err := provider.Up(cmd.Context())
		if err != nil {
			return err
		}

		if format == "json" {
			if results == nil {
				results = []*goose.MigrationResult{}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"applied": results,
			})
		}
		return nil
	},
}

// --- migrate down ---

var migrateDownCmd = &cobra.Command{
	Use:          "down",
	Short:        "Roll back the last migration",
	Long:         "Roll back the last migration, or down to a specific version with --version",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		ctx := cmd.Context()
		out := cmd.OutOrStdout()
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

		if format == "json" {
			if results == nil {
				results = []*goose.MigrationResult{}
			}
			return json.NewEncoder(out).Encode(map[string]interface{}{
				"applied": results,
			})
		}
		return nil
	},
}

func init() {
	migrateDownCmd.Flags().Int64("version", -1, "Target version to migrate down to (default: roll back one)")
}

// --- migrate status ---

var migrateStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show migration status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		statuses, err := provider.Status(cmd.Context())
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		if format == "json" {
			return json.NewEncoder(out).Encode(statuses)
		}

		fmt.Fprintln(out, "    Applied At                  Migration")
		fmt.Fprintln(out, "    =======================================")
		for _, s := range statuses {
			var appliedAt string
			if s.State == goose.StateApplied {
				appliedAt = s.AppliedAt.Format(time.RFC3339)
			} else {
				appliedAt = "Pending"
			}
			fmt.Fprintf(out, "    %-24s -- %s\n", appliedAt, s.Source.Path)
		}
		return nil
	},
}

// --- migrate check ---

var migrateCheckCmd = &cobra.Command{
	Use:          "check",
	Short:        "Check if there are pending migrations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openMigrateDB(cmd)
		if err != nil {
			return err
		}
		defer db.Close()

		provider, err := newGooseProvider(db)
		if err != nil {
			return fmt.Errorf("failed to create goose provider: %w", err)
		}

		ctx := cmd.Context()
		out := cmd.OutOrStdout()

		hasPending, err := provider.HasPending(ctx)
		if err != nil {
			return fmt.Errorf("failed to check pending migrations: %w", err)
		}

		if hasPending {
			current, err := provider.GetDBVersion(ctx)
			if err != nil {
				return fmt.Errorf("migrations are pending (failed to get current version: %v)", err)
			}
			if format == "json" {
				return json.NewEncoder(out).Encode(map[string]interface{}{
					"status":  "pending",
					"version": current,
				})
			}
			return fmt.Errorf("migrations are pending: current version %d", current)
		}

		current, err := provider.GetDBVersion(ctx)
		if format == "json" {
			status := "ok"
			if err != nil {
				status = "unknown"
			}
			return json.NewEncoder(out).Encode(map[string]interface{}{
				"status":  status,
				"version": current,
			})
		}

		if err != nil {
			fmt.Fprintln(out, "Database is up to date")
		} else {
			fmt.Fprintf(out, "Database is up to date (version %d)\n", current)
		}
		return nil
	},
}
