package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
		name      string
		args      []string
		setupFile func(t *testing.T) string // returns temp file path
		wantErr   bool
		validate  func(t *testing.T, sp interface{})
	}{
		{
			name:    "basic entity ID and ACS URL",
			args:    []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			wantErr: false,
		},
		{
			name: "with nameid-format",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs", "--nameid-format", "persistent"},
			validate: func(t *testing.T, sp interface{}) {
				// Validation handled by domain model
			},
			wantErr: false,
		},
		{
			name: "with attribute mapping file",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "mapping.json")
				data := `{"nameid_format": "persistent", "saml_attributes": {"subject": "uid"}}`
				if err := os.WriteFile(path, []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: false,
		},
		{
			name:    "invalid attribute mapping file path",
			args:    []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs", "--attribute-mapping-file", "/nonexistent/mapping.json"},
			wantErr: true,
		},
		{
			name: "invalid JSON in attribute mapping file",
			args: []string{"--entity-id", "http://example.com/metadata", "--acs-url", "http://example.com/acs"},
			setupFile: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "mapping.json")
				if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			cmd.Flags().StringVar(&spNameIDFormat, "nameid-format", "", "")

			if err := cmd.ParseFlags(args); err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected error parsing flags: %v", err)
				}
				return
			}

			sp, err := buildServiceProvider()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
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
		"nameid-format",
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
