// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
)

// SPResult represents the data payload of a registered service provider.
type SPResult struct {
	EntityID   string `json:"entity_id"`
	ACSURL     string `json:"acs_url"`
	ACSBinding string `json:"acs_binding"`
}

// formatSPRegistered formats successful registration output in text mode.
func formatSPRegistered(w io.Writer, sp *SPResult) error {
	_, err := fmt.Fprintf(w,
		"✓ Service provider registered successfully!\n  Entity ID:   %s\n  ACS URL:     %s\n  ACS Binding: %s\n",
		sp.EntityID, sp.ACSURL, sp.ACSBinding,
	)
	return err
}
