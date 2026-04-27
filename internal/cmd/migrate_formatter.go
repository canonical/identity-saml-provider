package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pressly/goose/v3"
)

// MigrateOutputFormatter defines the strategy interface for formatting
// migration command output. Implement this interface to add new output
// formats (e.g., YAML, table).
type MigrateOutputFormatter interface {
	// ShouldSilenceGoose returns true if the goose provider's built-in
	// logging should be suppressed (e.g., when the formatter handles all output).
	ShouldSilenceGoose() bool

	// MigrationResults formats the output of migrate up/down commands.
	MigrationResults(w io.Writer, results []*goose.MigrationResult) error

	// MigrationStatuses formats the output of the migrate status command.
	MigrationStatuses(w io.Writer, statuses []*goose.MigrationStatus) error

	// MigrationCheck formats the output of the migrate check command.
	MigrationCheck(w io.Writer, result CheckResult) error
}

// CheckStatus represents the state of database migrations.
type CheckStatus string

const (
	CheckStatusOK      CheckStatus = "ok"
	CheckStatusPending CheckStatus = "pending"
	CheckStatusUnknown CheckStatus = "unknown"
)

// CheckResult holds the outcome of a migration check.
type CheckResult struct {
	// Status is the migration state: "ok", "pending", or "unknown".
	Status CheckStatus `json:"status"`
	// Version is the current database schema version.
	Version int64 `json:"version"`
}

// newFormatter returns a MigrateOutputFormatter for the given format name.
func newFormatter(format string) (MigrateOutputFormatter, error) {
	switch format {
	case "text":
		return &textFormatter{}, nil
	case "json":
		return &jsonFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

// --- text formatter ---

type textFormatter struct{}

func (f *textFormatter) ShouldSilenceGoose() bool { return false }

func (f *textFormatter) MigrationResults(_ io.Writer, _ []*goose.MigrationResult) error {
	// goose already logs applied migrations in text mode.
	return nil
}

func (f *textFormatter) MigrationStatuses(w io.Writer, statuses []*goose.MigrationStatus) error {
	if _, err := fmt.Fprintf(w, "    Applied At                  Migration\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "    =======================================\n"); err != nil {
		return err
	}
	for _, s := range statuses {
		var appliedAt string
		if s.State == goose.StateApplied {
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		} else {
			appliedAt = "Pending"
		}
		if _, err := fmt.Fprintf(w, "    %-24s -- %s\n", appliedAt, s.Source.Path); err != nil {
			return err
		}
	}
	return nil
}

func (f *textFormatter) MigrationCheck(w io.Writer, result CheckResult) error {
	var err error
	switch result.Status {
	case CheckStatusPending:
		_, err = fmt.Fprintf(w, "migrations are pending: current version %d\n", result.Version)
	case CheckStatusUnknown:
		_, err = fmt.Fprintf(w, "migration status is unknown\n")
	default:
		_, err = fmt.Fprintf(w, "database is up to date (version %d)\n", result.Version)
	}
	return err
}

// --- json formatter ---

type jsonFormatter struct{}

func (f *jsonFormatter) ShouldSilenceGoose() bool { return true }

func (f *jsonFormatter) MigrationResults(w io.Writer, results []*goose.MigrationResult) error {
	if results == nil {
		results = []*goose.MigrationResult{}
	}
	return json.NewEncoder(w).Encode(map[string]interface{}{
		"applied": results,
	})
}

func (f *jsonFormatter) MigrationStatuses(w io.Writer, statuses []*goose.MigrationStatus) error {
	return json.NewEncoder(w).Encode(statuses)
}

func (f *jsonFormatter) MigrationCheck(w io.Writer, result CheckResult) error {
	return json.NewEncoder(w).Encode(result)
}
