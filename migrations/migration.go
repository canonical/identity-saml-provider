package migrations

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var EmbedMigrations embed.FS

// RunMigrationsUp is a helper for running migrations programmatically (e.g., in tests).
func RunMigrationsUp(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(EmbedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, EmbedMigrations, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
