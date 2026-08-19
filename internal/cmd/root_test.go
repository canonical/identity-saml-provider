// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootFormatFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("format")
	if flag == nil {
		t.Fatal("expected persistent flag 'format' on rootCmd")
	}

	if flag.Shorthand != "" {
		t.Errorf("expected no shorthand alias for --format flag per DE013, got %q", flag.Shorthand)
	}

	if flag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", flag.DefValue)
	}
}

func TestRootFormatValidation(t *testing.T) {
	tests := []struct {
		name        string
		formatValue string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid text format",
			formatValue: "text",
			wantErr:     false,
		},
		{
			name:        "valid json format",
			formatValue: "json",
			wantErr:     false,
		},
		{
			name:        "invalid xml format",
			formatValue: "xml",
			wantErr:     true,
			errContains: "unsupported output format: \"xml\"",
		},
		{
			name:        "invalid yaml format",
			formatValue: "yaml",
			wantErr:     true,
			errContains: "unsupported output format: \"yaml\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFormat = tt.formatValue
			defer func() { outputFormat = "text" }()

			err := rootCmd.PersistentPreRunE(rootCmd, []string{})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSubcommandsInheritFormatFlag(t *testing.T) {
	subcommands := []string{"service-provider", "requests", "migrations", "version", "serve"}

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			var targetCmdFound bool
			for _, cmd := range rootCmd.Commands() {
				if cmd.Name() == name {
					targetCmdFound = true
					flag := cmd.Flag("format")
					if flag == nil {
						t.Errorf("command %q did not inherit persistent flag 'format'", name)
					}
					break
				}
			}
			if !targetCmdFound {
				t.Errorf("subcommand %q not found under rootCmd", name)
			}
		})
	}
}

func TestExecuteRoot_FormatError(t *testing.T) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	// Avoid cross contamination of global state by saving and restoring the
	// original values.
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	oldArgs := rootCmd.Flags().Args()
	oldFormat := outputFormat
	defer func() {
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(oldArgs)
		outputFormat = oldFormat
	}()

	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"version", "--format", "invalid"})

	err := ExecuteRoot(rootCmd)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}

	// In text mode or invalid format, error should go to stderr
	if !strings.Contains(errBuf.String(), "unsupported output format") {
		t.Errorf("expected error on stderr, got stderr=%q, stdout=%q", errBuf.String(), outBuf.String())
	}
}
