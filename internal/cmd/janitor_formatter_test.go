// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewJanitorFormatter(t *testing.T) {
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
			f, err := newJanitorFormatter(tt.format)
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

func TestFormatPendingRequests(t *testing.T) {
	tests := []struct {
		name         string
		formatter    JanitorOutputFormatter
		deletedCount int64
		validate     func(t *testing.T, output string)
	}{
		{
			name:         "text pruned format",
			formatter:    &janitorTextFormatter{},
			deletedCount: 42,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "[Pruned] successfully deleted 42 expired pending request(s).") {
					t.Errorf("expected pruned text format, got %q", output)
				}
			},
		},
		{
			name:         "json pruned format",
			formatter:    &janitorJSONFormatter{},
			deletedCount: 100,
			validate: func(t *testing.T, output string) {
				var result janitorJSONResult
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result.DeletedCount != 100 {
					t.Errorf("expected deleted_count 100, got %d", result.DeletedCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.FormatPendingRequests(&buf, tt.deletedCount)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}
