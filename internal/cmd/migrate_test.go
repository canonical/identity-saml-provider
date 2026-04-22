package cmd

import (
	"bytes"
	"testing"
)

func TestMigrateSubcommands(t *testing.T) {
	// Verify all expected subcommands are registered
	expected := map[string]bool{
		"up":     false,
		"down":   false,
		"status": false,
		"check":  false,
	}

	for _, sub := range migrateCmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected subcommand %q not found on migrate command", name)
		}
	}
}

func TestMigrateRequiresDSN(t *testing.T) {
	// Running migrate up without --dsn should fail
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"migrate", "up"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --dsn is not provided")
	}
}

func TestMigrateDownVersionFlag(t *testing.T) {
	// Verify down command has --version flag
	flag := migrateDownCmd.Flags().Lookup("version")
	if flag == nil {
		t.Error("expected --version flag on migrate down command")
	}
}
