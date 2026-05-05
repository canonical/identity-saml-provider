package handler_test

import (
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/handler"
	"github.com/crewjam/saml"
)

func TestRegisterSPRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     handler.RegisterSPRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request with all fields",
			req: handler.RegisterSPRequest{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: saml.HTTPPostBinding,
			},
		},
		{
			name: "valid request — default binding applied",
			req: handler.RegisterSPRequest{
				EntityID: "https://sp.example.com",
				ACSURL:   "https://sp.example.com/acs",
			},
		},
		{
			name: "valid with attribute mapping",
			req: handler.RegisterSPRequest{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: saml.HTTPPostBinding,
				AttributeMapping: &domain.AttributeMapping{
					NameIDFormat: "persistent",
				},
			},
		},
		{
			name: "missing entity_id",
			req: handler.RegisterSPRequest{
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: saml.HTTPPostBinding,
			},
			wantErr: true,
			errMsg:  "entity_id",
		},
		{
			name: "missing acs_url",
			req: handler.RegisterSPRequest{
				EntityID:   "https://sp.example.com",
				ACSBinding: saml.HTTPPostBinding,
			},
			wantErr: true,
			errMsg:  "acs_url",
		},
		{
			name: "invalid acs_url — not a URL",
			req: handler.RegisterSPRequest{
				EntityID: "https://sp.example.com",
				ACSURL:   "not-a-url",
			},
			wantErr: true,
			errMsg:  "acs_url",
		},
		{
			name: "invalid acs_url — missing scheme",
			req: handler.RegisterSPRequest{
				EntityID: "https://sp.example.com",
				ACSURL:   "example.com/acs",
			},
			wantErr: true,
			errMsg:  "acs_url",
		},
		{
			name: "invalid acs_url — ftp scheme",
			req: handler.RegisterSPRequest{
				EntityID: "https://sp.example.com",
				ACSURL:   "ftp://example.com/acs",
			},
			wantErr: true,
			errMsg:  "acs_url",
		},
		{
			name: "invalid acs_binding",
			req: handler.RegisterSPRequest{
				EntityID:   "https://sp.example.com",
				ACSURL:     "https://sp.example.com/acs",
				ACSBinding: "invalid-binding",
			},
			wantErr: true,
			errMsg:  "acs_binding",
		},
		{
			name: "invalid attribute mapping",
			req: handler.RegisterSPRequest{
				EntityID: "https://sp.example.com",
				ACSURL:   "https://sp.example.com/acs",
				AttributeMapping: &domain.AttributeMapping{
					NameIDFormat: "BOGUS_FORMAT",
				},
			},
			wantErr: true,
			errMsg:  "nameid_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					var validErr *domain.ErrValidation
					if ok := isValidationErr(err, &validErr); ok {
						if validErr.Field != tt.errMsg {
							t.Errorf("field = %q, want containing %q", validErr.Field, tt.errMsg)
						}
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func isValidationErr(err error, target **domain.ErrValidation) bool {
	for err != nil {
		if e, ok := err.(*domain.ErrValidation); ok {
			*target = e
			return true
		}
		// Try unwrapping
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
