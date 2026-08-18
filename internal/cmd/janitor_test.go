// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"testing"
)

func TestJanitorCommandRegistered(t *testing.T) {
	var janitorCmdFound bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "janitor" {
			janitorCmdFound = true

			// Verify format flag is inherited
			formatFlag := cmd.Flag("format")
			if formatFlag == nil {
				t.Error("expected 'format' flag to be available on 'janitor'")
			} else {
				if formatFlag.DefValue != "text" {
					t.Errorf("expected default format to be text, got %s", formatFlag.DefValue)
				}
			}

			// Verify pending-requests subcommand is registered under janitor
			var pendingReqSubCmdFound bool
			for _, sub := range cmd.Commands() {
				if sub.Name() == "pending-requests" {
					pendingReqSubCmdFound = true

					// Verify default flags are registered correctly
					batchSizeFlag := sub.Flags().Lookup("batch-size")
					if batchSizeFlag == nil {
						t.Error("expected 'batch-size' flag to be registered")
					} else {
						if batchSizeFlag.DefValue != "1000" {
							t.Errorf("expected default batch-size to be 1000, got %s", batchSizeFlag.DefValue)
						}
					}
				}
			}

			if !pendingReqSubCmdFound {
				t.Error("expected 'pending-requests' subcommand to be registered under 'janitor'")
			}
		}
	}

	if !janitorCmdFound {
		t.Error("expected 'janitor' command to be registered under rootCmd")
	}
}

func TestValidateBatchSize(t *testing.T) {
	// Test case 1: valid batch-size
	if err := validateBatchSize(500); err != nil {
		t.Errorf("unexpected error for valid batch-size 500: %v", err)
	}

	// Test case 2: invalid batch-size <= 0
	if err := validateBatchSize(0); err == nil {
		t.Error("expected error for batch-size 0, got nil")
	} else if err.Error() != "batch-size must be greater than 0" {
		t.Errorf("expected 'batch-size must be greater than 0' error, got %q", err.Error())
	}
}
