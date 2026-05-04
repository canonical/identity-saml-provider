package domain

import "time"

// Session represents an authenticated user session bridging OIDC and SAML.
type Session struct {
	ID             string
	CreateTime     time.Time
	ExpireTime     time.Time
	Index          string
	NameID         string
	UserEmail      string
	UserCommonName string
	UserName       string // OIDC subject (sub claim)
	Groups         []string
	RawOIDCClaims  map[string]interface{} // All claims from the OIDC ID token
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
