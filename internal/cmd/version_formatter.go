// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
)

// VersionResult represents the data payload for the version command.
type VersionResult struct {
	Version string `json:"version"`
}

// formatVersion formats the version output in text mode.
func formatVersion(w io.Writer, res VersionResult) error {
	_, err := fmt.Fprintln(w, res.Version)
	return err
}
