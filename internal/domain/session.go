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
