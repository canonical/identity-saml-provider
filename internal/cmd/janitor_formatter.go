// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
)

// JanitorResult represents the data payload for janitor pruning operations.
type JanitorResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

// formatJanitorPendingRequests formats the pending requests cleanup result in text mode.
func formatJanitorPendingRequests(w io.Writer, res JanitorResult) error {
	_, err := fmt.Fprintf(w, "[Pruned] successfully deleted %d expired pending request(s).\n", res.DeletedCount)
	return err
}
