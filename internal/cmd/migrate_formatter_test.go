package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantError bool
	}{
		{"text format", "text", false},
		{"json format", "json", false},
		{"unknown format", "yaml", true},
		{"empty format", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := newMigrateFormatter(tt.format)
			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f == nil {
				t.Fatal("expected formatter but got nil")
			}
		})
	}
}

func TestShouldSilenceGoose(t *testing.T) {
	tests := []struct {
		name      string
		formatter MigrateOutputFormatter
		want      bool
	}{
		{"text formatter", &textFormatter{}, false},
		{"json formatter", &jsonFormatter{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.formatter.ShouldSilenceGoose(); got != tt.want {
				t.Errorf("ShouldSilenceGoose() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMigrationResults(t *testing.T) {
	sampleResults := []*goose.MigrationResult{
		{Source: &goose.Source{Path: "001_init.sql"}},
	}

	tests := []struct {
		name      string
		formatter MigrateOutputFormatter
		results   []*goose.MigrationResult
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "text with nil results",
			formatter: &textFormatter{},
			results:   nil,
			validate: func(t *testing.T, output string) {
				if output != "" {
					t.Errorf("expected no output, got %q", output)
				}
			},
		},
		{
			name:      "text with results",
			formatter: &textFormatter{},
			results:   sampleResults,
			validate: func(t *testing.T, output string) {
				if output != "" {
					t.Errorf("expected no output, got %q", output)
				}
			},
		},
		{
			name:      "json with nil results",
			formatter: &jsonFormatter{},
			results:   nil,
			validate: func(t *testing.T, output string) {
				var result map[string]json.RawMessage
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if _, ok := result["applied"]; !ok {
					t.Error("expected 'applied' key in JSON output")
				}
			},
		},
		{
			name:      "json with results",
			formatter: &jsonFormatter{},
			results:   sampleResults,
			validate: func(t *testing.T, output string) {
				var result map[string]json.RawMessage
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if _, ok := result["applied"]; !ok {
					t.Error("expected 'applied' key in JSON output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.MigrationResults(&buf, tt.results)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}

func TestMigrationStatuses(t *testing.T) {
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

	tests := []struct {
		name      string
		formatter MigrateOutputFormatter
		statuses  []*goose.MigrationStatus
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "text",
			formatter: &textFormatter{},
			statuses:  sampleStatuses,
			validate: func(t *testing.T, output string) {
				for _, want := range []string{"Applied At", "001_init.sql", "Pending"} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:      "json",
			formatter: &jsonFormatter{},
			statuses:  sampleStatuses,
			validate: func(t *testing.T, output string) {
				if !json.Valid([]byte(output)) {
					t.Errorf("expected valid JSON, got %q", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.MigrationStatuses(&buf, tt.statuses)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}

func TestMigrationCheck(t *testing.T) {
	tests := []struct {
		name      string
		formatter MigrateOutputFormatter
		result    CheckResult
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "text pending",
			formatter: &textFormatter{},
			result:    CheckResult{Status: CheckStatusPending, Version: 3},
			validate: func(t *testing.T, output string) {
				for _, want := range []string{"pending", "3"} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:      "text up to date",
			formatter: &textFormatter{},
			result:    CheckResult{Status: CheckStatusOK, Version: 5},
			validate: func(t *testing.T, output string) {
				for _, want := range []string{"up to date", "5"} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:      "text unknown version",
			formatter: &textFormatter{},
			result:    CheckResult{Status: CheckStatusUnknown},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "unknown") {
					t.Errorf("expected 'unknown' in output, got %q", output)
				}
			},
		},
		{
			name:      "json pending",
			formatter: &jsonFormatter{},
			result:    CheckResult{Status: CheckStatusPending, Version: 3},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result["status"] != "pending" {
					t.Errorf("expected status 'pending', got %v", result["status"])
				}
				if result["version"] != float64(3) {
					t.Errorf("expected version 3, got %v", result["version"])
				}
			},
		},
		{
			name:      "json up to date",
			formatter: &jsonFormatter{},
			result:    CheckResult{Status: CheckStatusOK, Version: 5},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result["status"] != "ok" {
					t.Errorf("expected status 'ok', got %v", result["status"])
				}
			},
		},
		{
			name:      "json unknown version",
			formatter: &jsonFormatter{},
			result:    CheckResult{Status: CheckStatusUnknown},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result["status"] != "unknown" {
					t.Errorf("expected status 'unknown', got %v", result["status"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.MigrationCheck(&buf, tt.result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}
