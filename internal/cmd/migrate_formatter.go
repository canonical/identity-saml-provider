// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/pressly/goose/v3"
)

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

// MigrateResultsResult holds the results of an up or down migration execution.
type MigrateResultsResult struct {
	Applied []*goose.MigrationResult `json:"applied"`
}

// formatMigrationStatuses formats the output of the migrate status command in text mode.
func formatMigrationStatuses(w io.Writer, statuses []*goose.MigrationStatus) error {
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

// formatMigrationCheck formats the output of the migrate check command in text mode.
func formatMigrationCheck(w io.Writer, result CheckResult) error {
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
