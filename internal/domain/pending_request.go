// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "time"

// PendingAuthnRequest captures a SAML AuthnRequest that is waiting
// for the user to complete OIDC authentication.
type PendingAuthnRequest struct {
	RequestID      string
	SAMLRequest    string
	RelayState     string
	ClientMetadata map[string]string
	CreatedAt      time.Time
	ExpireAt       time.Time
}
