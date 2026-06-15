// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

func TestAttributeMapping_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mapping *domain.AttributeMapping
		wantErr bool
		field   string
	}{
		{
			name:    "nil mapping is valid",
			mapping: nil,
		},
		{
			name:    "empty NameIDFormat is valid",
			mapping: &domain.AttributeMapping{},
		},
		{
			name:    "persistent format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "persistent"},
		},
		{
			name:    "transient format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "transient"},
		},
		{
			name:    "emailAddress format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "emailAddress"},
		},
		{
			name:    "email format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "email"},
		},
		{
			name:    "unspecified format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "unspecified"},
		},
		{
			name:    "full URN NameID format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
		},
		{
			name:    "invalid NameID format is rejected",
			mapping: &domain.AttributeMapping{NameIDFormat: "bogus"},
			wantErr: true,
			field:   "nameid_format",
		},
		{
			name:    "partial URN is rejected",
			mapping: &domain.AttributeMapping{NameIDFormat: "ur:bad"},
			wantErr: true,
			field:   "nameid_format",
		},
		{
			name: "valid SAMLAttributeDef is accepted",
			mapping: &domain.AttributeMapping{
				SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
					"email": {Name: "mail", FriendlyName: "mail"},
				},
			},
		},
		{
			name: "empty SAMLAttributeDef name is rejected",
			mapping: &domain.AttributeMapping{
				SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
					"email": {Name: ""},
				},
			},
			wantErr: true,
			field:   "saml_attribute_mappings.email.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.mapping.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var valErr *domain.ErrValidation
				if !errors.As(err, &valErr) {
					t.Fatalf("expected *ErrValidation, got %T", err)
				}
				if valErr.Field != tt.field {
					t.Errorf("ErrValidation.Field = %q, want %q", valErr.Field, tt.field)
				}
			}
		})
	}
}

func TestSAMLAttributeDef_EffectiveNameFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  domain.SAMLAttributeDef
		want string
	}{
		{
			name: "explicit name format wins",
			def:  domain.SAMLAttributeDef{Name: "mail", NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"},
			want: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
		},
		{
			name: "empty name format defaults to URI",
			def:  domain.SAMLAttributeDef{Name: "mail"},
			want: domain.DefaultNameFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.def.EffectiveNameFormat(); got != tt.want {
				t.Errorf("EffectiveNameFormat() = %q, want %q", got, tt.want)
			}
		})
	}

	if domain.DefaultNameFormat != "urn:oasis:names:tc:SAML:2.0:attrname-format:uri" {
		t.Errorf("DefaultNameFormat = %q, want URI format", domain.DefaultNameFormat)
	}
}

func TestUserAttributes_GetField(t *testing.T) {
	t.Parallel()

	attrs := &domain.UserAttributes{
		Subject: "sub-1",
		Email:   "alice@example.com",
		Name:    "Alice",
		Custom:  map[string]string{"dept": "eng"},
	}

	tests := []struct {
		field string
		want  string
	}{
		{"subject", "sub-1"},
		{"email", "alice@example.com"},
		{"name", "Alice"},
		{"dept", "eng"},
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()
			if got := attrs.GetField(tt.field); got != tt.want {
				t.Errorf("GetField(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}

	t.Run("nil receiver returns empty", func(t *testing.T) {
		t.Parallel()
		var u *domain.UserAttributes
		if got := u.GetField("email"); got != "" {
			t.Errorf("nil GetField = %q, want empty", got)
		}
	})
}

// TestAttributeMapping_JSONRoundTrip verifies that AttributeMapping
// serialises to the expected JSONB field names and deserialises back
// without field loss.
func TestAttributeMapping_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := &domain.AttributeMapping{
		NameIDFormat: "persistent",
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email": {
				Name:         "urn:oid:0.9.2342.19200300.100.1.3",
				FriendlyName: "mail",
				NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			},
			"subject": {Name: "uid"},
		},
		OIDCClaimMappings: map[string]string{
			"sub":   "subject",
			"email": "email",
		},
		Options: domain.MappingOptions{LowercaseEmail: true},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTripped domain.AttributeMapping
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(*original, roundTripped) {
		t.Errorf("round-trip mismatch:\n  original    = %+v\n  roundTripped = %+v",
			*original, roundTripped)
	}

	// Spot-check that the JSON field names match the expected tag names.
	jsonStr := string(data)
	for _, want := range []string{`"saml_attribute_mappings"`, `"oidc_claim_mappings"`, `"name_format"`, `"friendly_name"`} {
		if !strings.Contains(jsonStr, want) {
			t.Errorf("expected JSON to contain %s, got: %s", want, jsonStr)
		}
	}
}
