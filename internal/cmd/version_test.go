// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/version"
)

func TestVersionCommandText(t *testing.T) {
	buf := new(bytes.Buffer)

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

	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != version.Version {
		t.Errorf("expected version %q, got %q", version.Version, got)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	buf := new(bytes.Buffer)

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

	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version", "--format", "json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ResponseEnvelope[VersionResult]
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.Data.Version != version.Version {
		t.Errorf("expected version %q, got %q", version.Version, result.Data.Version)
	}
}
