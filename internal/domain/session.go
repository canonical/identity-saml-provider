// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "time"

// Attribute represents a SAML attribute with one or more values.
// This is the domain equivalent of saml.Attribute, keeping the domain
// layer free of external SAML library imports.
type Attribute struct {
	FriendlyName string
	Name         string
	NameFormat   string
	Values       []AttributeValue
}

// AttributeValue represents a single value of a SAML attribute.
type AttributeValue struct {
	Type  string
	Value string
}

// Session represents an authenticated user session bridging OIDC and SAML.
type Session struct {
	ID                    string
	CreateTime            time.Time
	ExpireTime            time.Time
	Index                 string
	NameID                string
	NameIDFormat          string
	UserEmail             string
	UserCommonName        string
	UserName              string // OIDC subject (sub claim)
	UserSurname           string
	UserGivenName         string
	UserScopedAffiliation string
	Groups                []string
	CustomAttributes      []Attribute
	RawOIDCClaims         map[string]interface{} // All claims from the OIDC ID token
}

// IsExpired reports whether the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpireTime)
}

// DisplayName returns the user's display name, falling back to email
// if UserCommonName is empty.
func (s *Session) DisplayName() string {
	if s.UserCommonName != "" {
		return s.UserCommonName
	}
	return s.UserEmail
}

// CanonicalSubject returns the OIDC `sub` claim from RawOIDCClaims as
// a string, together with `true` when the claim is present, is a
// string, and is non-empty. It returns `"", false` when the claim is
// missing, nil, non-string, or an empty string. Callers MUST treat
// the bool as the authoritative presence signal and SHALL NOT
// substitute any other session field on `false`.
func (s *Session) CanonicalSubject() (string, bool) {
	if s == nil || s.RawOIDCClaims == nil {
		return "", false
	}
	str, ok := s.RawOIDCClaims["sub"].(string)
	if !ok || str == "" {
		return "", false
	}
	return str, true
}
