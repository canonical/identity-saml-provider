// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// JanitorOutputFormatter formats output for `janitor` subcommands.
type JanitorOutputFormatter interface {
	// FormatPendingRequests formats the pending requests cleanup result.
	FormatPendingRequests(w io.Writer, deletedCount int64) error
}

// newJanitorFormatter returns a JanitorOutputFormatter for the given format name.
func newJanitorFormatter(format string) (JanitorOutputFormatter, error) {
	switch format {
	case "text":
		return &janitorTextFormatter{}, nil
	case "json":
		return &janitorJSONFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

// --- text formatter ---

type janitorTextFormatter struct{}

func (f *janitorTextFormatter) FormatPendingRequests(w io.Writer, deletedCount int64) error {
	_, err := fmt.Fprintf(w, "[Pruned] successfully deleted %d expired pending request(s).\n", deletedCount)
	return err
}

// --- json formatter ---

type janitorJSONFormatter struct{}

type janitorJSONResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

func (f *janitorJSONFormatter) FormatPendingRequests(w io.Writer, deletedCount int64) error {
	return json.NewEncoder(w).Encode(janitorJSONResult{
		DeletedCount: deletedCount,
	})
}
