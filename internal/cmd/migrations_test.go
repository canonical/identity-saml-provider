// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"testing"
)

func TestMigrationsSubcommands(t *testing.T) {
	// Verify all expected subcommands are registered on migrationsCmd
	expected := map[string]bool{
		"apply":    false,
		"rollback": false,
		"show":     false,
		"check":    false,
	}

	for _, sub := range migrationsCmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected subcommand %q not found on migrations command", name)
		}
	}
}

func TestMigrationsRequiresDSN(t *testing.T) {
	// Running migrations apply without --dsn should fail
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"migrations", "apply"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --dsn is not provided")
	}
}

func TestMigrationsRollbackVersionFlag(t *testing.T) {
	// Verify rollback command has --version flag
	flag := migrationsRollbackCmd.Flags().Lookup("version")
	if flag == nil {
		t.Error("expected --version flag on migrations rollback command")
	}
}

func TestMigrationsShowNoHeadersFlag(t *testing.T) {
	// Verify show command has --no-headers flag
	flag := migrationsShowCmd.Flags().Lookup("no-headers")
	if flag == nil {
		t.Error("expected --no-headers flag on migrations show command")
	}
}
