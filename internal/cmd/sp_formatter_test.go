package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

func TestNewSPFormatter(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantError bool
	}{
		{"text format", "text", false},
		{"json format", "json", false},
		{"unknown format", "yaml", true},
		{"empty format", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := newSPFormatter(tt.format)
			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f == nil {
				t.Fatal("expected formatter but got nil")
			}
		})
	}
}

func TestSPRegistered(t *testing.T) {
	sp := &domain.ServiceProvider{
		EntityID:   "http://example.com/metadata",
		ACSURL:     "http://example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}

	tests := []struct {
		name      string
		formatter SPOutputFormatter
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "text output",
			formatter: &spTextFormatter{},
			validate: func(t *testing.T, output string) {
				for _, want := range []string{
					"✓",
					"http://example.com/metadata",
					"http://example.com/acs",
					"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
				} {
					if !strings.Contains(output, want) {
						t.Errorf("expected %q in output, got %q", want, output)
					}
				}
			},
		},
		{
			name:      "json output",
			formatter: &spJSONFormatter{},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result["status"] != "success" {
					t.Errorf("expected status 'success', got %v", result["status"])
				}
				if result["entity_id"] != "http://example.com/metadata" {
					t.Errorf("expected entity_id 'http://example.com/metadata', got %v", result["entity_id"])
				}
				if result["acs_url"] != "http://example.com/acs" {
					t.Errorf("expected acs_url 'http://example.com/acs', got %v", result["acs_url"])
				}
				if result["acs_binding"] != "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" {
					t.Errorf("expected acs_binding, got %v", result["acs_binding"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.SPRegistered(&buf, sp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, buf.String())
		})
	}
}

func TestSPError(t *testing.T) {
	sp := &domain.ServiceProvider{
		EntityID:   "http://example.com/metadata",
		ACSURL:     "http://example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}
	testErr := errors.New("database connection failed")

	tests := []struct {
		name      string
		formatter SPOutputFormatter
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "text error",
			formatter: &spTextFormatter{},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "database connection failed") {
					t.Errorf("expected error message in output, got %q", output)
				}
				if !strings.Contains(output, "http://example.com/metadata") {
					t.Errorf("expected entity ID in output, got %q", output)
				}
			},
		},
		{
			name:      "json error",
			formatter: &spJSONFormatter{},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if result["status"] != "error" {
					t.Errorf("expected status 'error', got %v", result["status"])
				}
				if result["error"] != "database connection failed" {
					t.Errorf("expected error message, got %v", result["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.formatter.SPError(&buf, sp, testErr)
			if err == nil {
				t.Fatal("expected error to be returned")
			}
			tt.validate(t, buf.String())
		})
	}
}
