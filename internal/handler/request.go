// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"github.com/canonical/identity-saml-provider/internal/domain"
)

// RegisterSPRequest is the JSON DTO for service provider registration.
type RegisterSPRequest struct {
	EntityID         string                   `json:"entity_id"`
	ACSURL           string                   `json:"acs_url"`
	ACSBinding       string                   `json:"acs_binding"`
	AttributeMapping *domain.AttributeMapping `json:"attribute_mapping,omitempty"`
}

// ToDomain converts the DTO to a domain.ServiceProvider.
func (r *RegisterSPRequest) ToDomain() *domain.ServiceProvider {
	return &domain.ServiceProvider{
		EntityID:         r.EntityID,
		ACSURL:           r.ACSURL,
		ACSBinding:       r.ACSBinding,
		AttributeMapping: r.AttributeMapping,
	}
}
