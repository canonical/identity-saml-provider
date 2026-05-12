package domain

import "time"

// IDToken represents a verified OIDC ID token with structured
// metadata and raw claims.
type IDToken struct {
	// Issuer is the URL of the OIDC provider that issued the token.
	Issuer string

	// Subject is the unique identifier for the authenticated user.
	Subject string

	// Expiry is the time at which the token expires.
	Expiry time.Time

	// IssuedAt is the time at which the token was issued.
	IssuedAt time.Time

	// Claims contains all claims from the ID token, including
	// standard and custom claims. Used for per-SP attribute mapping.
	Claims map[string]interface{}
}
