// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestFormatMigrationStatuses(t *testing.T) {
	sampleStatuses := []*goose.MigrationStatus{
		{
			State:     goose.StateApplied,
			Source:    &goose.Source{Path: "001_init.sql"},
			AppliedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			State:  goose.StatePending,
			Source: &goose.Source{Path: "002_add_table.sql"},
		},
	}

	var buf bytes.Buffer
	if err := formatMigrationStatuses(&buf, sampleStatuses); err != nil {
		t.Fatalf("formatMigrationStatuses() unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"Applied At", "001_init.sql", "Pending", "002_add_table.sql"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got %q", want, output)
		}
	}
}

func TestFormatMigrationCheck(t *testing.T) {
	tests := []struct {
		name     string
		result   CheckResult
		validate func(t *testing.T, output string)
	}{
		{
			name:   "text pending",
			result: CheckResult{Status: CheckStatusPending, Version: 3},
			validate: func(t *testing.T, output string) {
				for _, want := range []string{"pending", "3"} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:   "text up to date",
			result: CheckResult{Status: CheckStatusOK, Version: 5},
			validate: func(t *testing.T, output string) {
				for _, want := range []string{"up to date", "5"} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:   "text unknown version",
			result: CheckResult{Status: CheckStatusUnknown},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "unknown") {
					t.Errorf("expected 'unknown' in output, got %q", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := formatMigrationCheck(&buf, tt.result); err != nil {
				t.Fatalf("formatMigrationCheck() unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}

func TestMigrateJSONEnvelopes(t *testing.T) {
	t.Run("migration results envelope", func(t *testing.T) {
		payload := MigrateResultsResult{
			Applied: []*goose.MigrationResult{
				{Source: &goose.Source{Path: "001_init.sql"}},
			},
		}
		var buf bytes.Buffer
		if err := WriteJSONSuccess(&buf, payload); err != nil {
			t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
		}

		var env ResponseEnvelope[MigrateResultsResult]
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if env.Status != "success" {
			t.Errorf("expected status 'success', got %q", env.Status)
		}
		if len(env.Data.Applied) != 1 || env.Data.Applied[0].Source.Path != "001_init.sql" {
			t.Errorf("unexpected applied results: %+v", env.Data.Applied)
		}
	})

	t.Run("migration statuses envelope", func(t *testing.T) {
		payload := []*goose.MigrationStatus{
			{
				State:  goose.StatePending,
				Source: &goose.Source{Path: "002_add_table.sql"},
			},
		}
		var buf bytes.Buffer
		if err := WriteJSONSuccess(&buf, payload); err != nil {
			t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
		}

		var env ResponseEnvelope[[]*goose.MigrationStatus]
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if env.Status != "success" {
			t.Errorf("expected status 'success', got %q", env.Status)
		}
		if len(env.Data) != 1 || env.Data[0].Source.Path != "002_add_table.sql" {
			t.Errorf("unexpected statuses: %+v", env.Data)
		}
	})

	t.Run("migration check envelope", func(t *testing.T) {
		payload := CheckResult{
			Status:  CheckStatusOK,
			Version: 4,
		}
		var buf bytes.Buffer
		if err := WriteJSONSuccess(&buf, payload); err != nil {
			t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
		}

		var env ResponseEnvelope[CheckResult]
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if env.Status != "success" {
			t.Errorf("expected status 'success', got %q", env.Status)
		}
		if env.Data.Status != CheckStatusOK || env.Data.Version != 4 {
			t.Errorf("unexpected check data: %+v", env.Data)
		}
	})
}
