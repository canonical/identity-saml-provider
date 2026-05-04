package domain_test

import (
	"errors"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

func TestAttributeMapping_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mapping *domain.AttributeMapping
		wantErr bool
		field   string // expected ErrValidation.Field when wantErr is true
	}{
		{
			name:    "nil mapping is valid",
			mapping: nil,
			wantErr: false,
		},
		{
			name:    "empty NameIDFormat is valid",
			mapping: &domain.AttributeMapping{},
			wantErr: false,
		},
		{
			name:    "persistent format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "persistent"},
			wantErr: false,
		},
		{
			name:    "transient format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "transient"},
			wantErr: false,
		},
		{
			name:    "emailAddress format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "emailAddress"},
			wantErr: false,
		},
		{
			name:    "email format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "email"},
			wantErr: false,
		},
		{
			name:    "unspecified format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "unspecified"},
			wantErr: false,
		},
		{
			name:    "full URN format is valid",
			mapping: &domain.AttributeMapping{NameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
			wantErr: false,
		},
		{
			name:    "invalid format is rejected",
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
