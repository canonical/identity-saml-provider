// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
)

// RequestsPruneResult represents the data payload for pending requests prune operations.
type RequestsPruneResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

// formatRequestsPrune formats the pending requests cleanup result in text mode.
func formatRequestsPrune(w io.Writer, res RequestsPruneResult) error {
	_, err := fmt.Fprintf(w, "[Pruned] successfully deleted %d expired pending request(s).\n", res.DeletedCount)
	return err
}
