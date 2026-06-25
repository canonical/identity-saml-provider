// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/spf13/cobra"
)

func TestSPSubcommands(t *testing.T) {
	expected := map[string]bool{"add": false}
	for _, sub := range spCmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected subcommand %q not found on sp command", name)
		}
	}
}

func TestSPAddRequiresEntityID(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"sp", "add", "--acs-url", "http://example.com/acs"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --entity-id is not provided")
	}
}

func TestSPAddRequiresACSURL(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"sp", "add", "--entity-id", "http://example.com/metadata"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --acs-url is not provided")
	}
}

func TestBuildServiceProvider(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		setupFile       func(t *testing.T) string // returns temp file path
		wantErr         bool
		wantErrContains string
		validate        func(t *testing.T, sp *domain.ServiceProvider)
	}{
		{
			name:    "basic entity ID and ACS URL",
			args:    []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			wantErr: false,
			validate: func(t *testing.T, sp *domain.ServiceProvider) {
				if sp.AttributeMapping != nil {
					t.Errorf("expected AttributeMapping to be nil, got %+v", sp.AttributeMapping)
				}
			},
		},
		{
			name: "with attribute mapping file containing only nameid_format",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "nameid.json")
				if err := os.WriteFile(path, []byte(`{"nameid_format": "persistent"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: false,
			validate: func(t *testing.T, sp *domain.ServiceProvider) {
				if sp.AttributeMapping == nil {
					t.Fatal("expected AttributeMapping, got nil")
				}
				if sp.AttributeMapping.NameIDFormat != "persistent" {
					t.Errorf("expected NameIDFormat \"persistent\", got %q", sp.AttributeMapping.NameIDFormat)
				}
				if len(sp.AttributeMapping.SAMLAttributeMappings) != 0 {
					t.Errorf("expected empty SAMLAttributeMappings, got %+v", sp.AttributeMapping.SAMLAttributeMappings)
				}
			},
		},
		{
			name: "with full attribute mapping file",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "mapping.json")
				data := `{
					"nameid_format": "persistent",
					"saml_attribute_mappings": {
						"email": {"name": "mail", "friendly_name": "mail"}
					},
					"oidc_claim_mappings": {"sub": "subject", "email": "email"}
				}`
				if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: false,
			validate: func(t *testing.T, sp *domain.ServiceProvider) {
				if sp.AttributeMapping == nil {
					t.Fatal("expected AttributeMapping, got nil")
				}
				if sp.AttributeMapping.NameIDFormat != "persistent" {
					t.Errorf("expected NameIDFormat \"persistent\", got %q", sp.AttributeMapping.NameIDFormat)
				}
				def, ok := sp.AttributeMapping.SAMLAttributeMappings["email"]
				if !ok {
					t.Fatal("expected SAMLAttributeMappings[\"email\"]")
				}
				if def.Name != "mail" || def.FriendlyName != "mail" {
					t.Errorf("unexpected SAMLAttributeDef: %+v", def)
				}
			},
		},
		{
			name:            "non-existent attribute mapping file path",
			args:            []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs", "--attribute-mapping-file", "/nonexistent/mapping.json"},
			wantErr:         true,
			wantErrContains: "/nonexistent/mapping.json",
		},
		{
			name: "invalid JSON in attribute mapping file",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "bad.json")
				if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr:         true,
			wantErrContains: "bad.json",
		},
		{
			name: "attribute mapping file fails domain validation",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "invalid.json")
				// SAML attribute definition with empty Name fails
				// AttributeMapping.Validate() per FR-7 structural check.
				data := `{"saml_attribute_mappings": {"email": {"name": ""}}}`
				if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset package-level flag state to avoid leakage between
			// table cases.
			spEntityID = ""
			spACSURL = ""
			spACSBinding = ""
			spAttributeMappingFile = ""

			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.setupFile != nil {
				path := tt.setupFile(t)
				args = append(args, "--attribute-mapping-file", path)
			}

			// Create a standalone command with the same flags to test
			// buildServiceProvider without triggering DB connections.
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().StringVarP(&spEntityID, "entity-id", "e", "", "")
			cmd.Flags().StringVarP(&spACSURL, "acs-url", "a", "", "")
			cmd.Flags().StringVarP(&spACSBinding, "acs-binding", "b",
				"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST", "")
			cmd.Flags().StringVar(&spAttributeMappingFile, "attribute-mapping-file", "", "")

			if err := cmd.ParseFlags(args); err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected error parsing flags: %v", err)
				}
				return
			}

			sp, err := buildServiceProvider()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("expected error to contain %q, got %q", tt.wantErrContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sp == nil {
				t.Fatal("expected service provider but got nil")
			}

			if tt.validate != nil {
				tt.validate(t, sp)
			}
		})
	}
}

func TestSPAddHasExpectedFlags(t *testing.T) {
	expectedFlags := []string{
		"entity-id",
		"acs-url",
		"acs-binding",
		"attribute-mapping-file",
	}

	for _, name := range expectedFlags {
		if spAddCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q not found on sp add command", name)
		}
	}
}

func TestSPAddDefaultACSBinding(t *testing.T) {
	flag := spAddCmd.Flags().Lookup("acs-binding")
	if flag == nil {
		t.Fatal("expected --acs-binding flag")
	}
	if flag.DefValue != "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" {
		t.Errorf("expected default binding to be HTTP-POST, got %q", flag.DefValue)
	}
}

// Verify JSON format output structure matches expected schema.
func TestSPJSONOutputSchema(t *testing.T) {
	f := &spJSONFormatter{}
	sp := &domain.ServiceProvider{
		EntityID:   "http://example.com/metadata",
		ACSURL:     "http://example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}

	var buf bytes.Buffer
	if err := f.SPRegistered(&buf, sp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result spJSONResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.EntityID != sp.EntityID {
		t.Errorf("expected entity_id %q, got %q", sp.EntityID, result.EntityID)
	}
}
