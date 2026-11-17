// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"

	"github.com/canonical/identity-saml-provider/internal/domain"
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

// SPListItem represents a service provider entry in list output.
// AttributeMapping is omitted when the SP has no mapping configured,
// mirroring the admin API's GetServiceProviderResponse JSON shape.
type SPListItem struct {
	EntityID         string                   `json:"entity_id"`
	ACSURL           string                   `json:"acs_url"`
	ACSBinding       string                   `json:"acs_binding"`
	AttributeMapping *domain.AttributeMapping `json:"attribute_mapping,omitempty"`
}

// formatSPList formats the list of service providers in text mode.
// An empty slice prints a friendly message instead of an error.
func formatSPList(w io.Writer, sps []*SPListItem) error {
	if len(sps) == 0 {
		_, err := fmt.Fprintln(w, "No service providers registered.")
		return err
	}

	for _, sp := range sps {
		if _, err := fmt.Fprintf(w,
			"Entity ID:   %s\n  ACS URL:     %s\n  ACS Binding: %s\n",
			sp.EntityID, sp.ACSURL, sp.ACSBinding,
		); err != nil {
			return err
		}
	}
	return nil
}
