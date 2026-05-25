// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package logging

import (
	"testing"
)

func TestBuildLogger(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		devMode bool
		wantErr bool
	}{
		{
			name:    "production mode info level",
			level:   "info",
			devMode: false,
		},
		{
			name:    "dev mode debug level",
			level:   "debug",
			devMode: true,
		},
		{
			name:    "production mode debug level",
			level:   "debug",
			devMode: false,
		},
		{
			name:    "dev mode info level",
			level:   "info",
			devMode: true,
		},
		{
			name:    "invalid level returns error",
			level:   "invalid",
			devMode: false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := BuildLogger(tt.level, tt.devMode)
			if tt.wantErr {
				if err == nil {
					t.Error("BuildLogger() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildLogger() unexpected error: %v", err)
			}
			if logger == nil {
				t.Error("BuildLogger() returned nil logger")
			}
		})
	}
}
