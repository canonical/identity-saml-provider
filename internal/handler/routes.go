// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers all endpoints on the provided chi.Router.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	// SAML endpoints (delegated to crewjam/saml IdentityProvider)
	r.HandleFunc("/saml/metadata", h.samlIDP.ServeMetadata)
	r.HandleFunc("/saml/sso", h.HandleSSO)

	// OIDC callback (Hydra redirects users back here)
	r.HandleFunc("/saml/callback", h.HandleOIDCCallback)

	// Admin API
	r.Post("/admin/service-providers", h.HandleRegisterServiceProvider)
	r.Get("/admin/service-providers", h.HandleGetServiceProvider)
	r.Put("/admin/service-providers/attribute-mapping", h.HandleUpdateAttributeMapping)
	r.Delete("/admin/service-providers/attribute-mapping", h.HandleDeleteAttributeMapping)
}
