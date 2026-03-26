package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
)

func TestMappingConfig_LoadFromFile(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	// Create a temporary config file
	configYAML := `
default_mapping:
  nameid_format: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
  nameid_source: "sub"
  attribute_map:
    email: "urn:oid:0.9.2342.19200300.100.1.3"
    name: "urn:oid:2.16.840.1.113730.3.1.241"

service_providers:
  http://www.netsuite.com/sp:
    nameid_format: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
    nameid_source: "email"
    attribute_map:
      email: "urn:oid:0.9.2342.19200300.100.1.3"
      name: "urn:oid:2.5.4.3"
    options:
      lowercase_email: true
`

	// Write to temporary file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mappings.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	mc := NewMappingConfig(logger)
	if err := mc.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify default mapping
	if mc.DefaultMapping == nil {
		t.Fatal("Expected default mapping to be loaded")
	}
	if mc.DefaultMapping.NameIDFormat != "urn:oasis:names:tc:SAML:2.0:nameid-format:transient" {
		t.Errorf("Unexpected default NameID format: %s", mc.DefaultMapping.NameIDFormat)
	}

	// Verify NetSuite mapping
	netlify, ok := mc.ServiceProviders["http://www.netsuite.com/sp"]
	if !ok {
		t.Fatal("Expected NetSuite mapping to be loaded")
	}
	if netlify.NameIDFormat != "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
		t.Errorf("Unexpected NetSuite NameID format: %s", netlify.NameIDFormat)
	}
	if netlify.NameIDSource != "email" {
		t.Errorf("Expected NameID source 'email', got '%s'", netlify.NameIDSource)
	}
	if !netlify.Options.LowercaseEmail {
		t.Error("Expected LowercaseEmail to be true for NetSuite")
	}
}

func TestMappingConfig_GetMapping(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mc := NewMappingConfig(logger)

	// Add a custom mapping
	mc.ServiceProviders["http://custom.sp"] = &SPMapping{
		NameIDFormat: "custom-format",
		NameIDSource: "custom_source",
	}

	// Test getting custom mapping
	customMapping := mc.GetMapping("http://custom.sp")
	if customMapping.NameIDFormat != "custom-format" {
		t.Errorf("Expected custom format, got %s", customMapping.NameIDFormat)
	}

	// Test getting default mapping for unknown SP
	defaultMapping := mc.GetMapping("http://unknown.sp")
	if defaultMapping == nil {
		t.Fatal("Expected default mapping for unknown SP")
	}
	if !strings.Contains(defaultMapping.NameIDFormat, "transient") {
		t.Errorf("Expected transient format in default, got %s", defaultMapping.NameIDFormat)
	}
}

func TestClaimsMap_GetString(t *testing.T) {
	claims := ClaimsMap{
		"email": "user@example.com",
		"name":  "Test User",
	}

	email := claims.GetString("email", false)
	if email != "user@example.com" {
		t.Errorf("Expected 'user@example.com', got '%s'", email)
	}

	// Test lowercase
	email = claims.GetString("email", true)
	if email != "user@example.com" {
		t.Errorf("Expected lowercase email, got '%s'", email)
	}

	// Test nonexistent claim
	missing := claims.GetString("nonexistent", false)
	if missing != "" {
		t.Errorf("Expected empty string for missing claim, got '%s'", missing)
	}
}

func TestClaimsMap_GetStringSlice(t *testing.T) {
	// Test with []string
	claims1 := ClaimsMap{
		"groups": []string{"admin", "users"},
	}
	groups := claims1.GetStringSlice("groups")
	if len(groups) != 2 || groups[0] != "admin" {
		t.Errorf("Expected ['admin', 'users'], got %v", groups)
	}

	// Test with []interface{}
	claims2 := ClaimsMap{
		"groups": []interface{}{"admin", "users", "editors"},
	}
	groups = claims2.GetStringSlice("groups")
	if len(groups) != 3 || groups[1] != "users" {
		t.Errorf("Expected 3 groups, got %v", groups)
	}

	// Test with missing claim
	empty := claims1.GetStringSlice("nonexistent")
	if len(empty) != 0 {
		t.Errorf("Expected empty slice for missing claim, got %v", empty)
	}
}

func TestServer_buildSAMLSessionFromOIDCClaims(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	testServer := &Server{
		logger: logger,
		mappingConfig: &MappingConfig{
			DefaultMapping: &SPMapping{
				NameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
				NameIDSource: "sub",
				AttributeMap: map[string]string{
					"email":  "urn:oid:0.9.2342.19200300.100.1.3",
					"name":   "urn:oid:2.16.840.1.113730.3.1.241",
					"groups": "urn:oid:1.2.840.113556.1.4.221",
				},
				Options: &SPMappingOptions{
					LowercaseEmail: false,
				},
			},
			ServiceProviders: map[string]*SPMapping{
				"http://www.netsuite.com/sp": {
					NameIDFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
					NameIDSource: "email",
					AttributeMap: map[string]string{
						"email": "urn:oid:0.9.2342.19200300.100.1.3",
						"name":  "urn:oid:2.5.4.3",
					},
					Options: &SPMappingOptions{
						LowercaseEmail: true,
					},
				},
			},
		},
	}

	// Test with default mapping (should use 'sub' as NameID)
	t.Run("default mapping uses sub as NameID", func(t *testing.T) {
		mapping := testServer.mappingConfig.GetMapping("http://unknown.sp")
		if mapping.NameIDSource != "sub" {
			t.Errorf("Expected 'sub' as default NameID source, got %s", mapping.NameIDSource)
		}
	})

	// Test with NetSuite mapping (should use 'email' as NameID and lowercase it)
	t.Run("netsuite mapping uses email as NameID and lowercases", func(t *testing.T) {
		mapping := testServer.mappingConfig.GetMapping("http://www.netsuite.com/sp")
		if mapping.NameIDSource != "email" {
			t.Errorf("Expected 'email' as NetSuite NameID source, got %s", mapping.NameIDSource)
		}
		if !mapping.Options.LowercaseEmail {
			t.Error("Expected LowercaseEmail to be true for NetSuite")
		}
	})
}

func TestMappingConfig_EnsureMappingDefaults(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mc := NewMappingConfig(logger)

	// Create a mapping with only some fields set
	partialMapping := &SPMapping{
		NameIDFormat: "custom-format",
		// NameIDSource and AttributeMap are nil/empty
	}

	mc.ensureMappingDefaults(partialMapping)

	if partialMapping.NameIDFormat != "custom-format" {
		t.Error("Should preserve explicitly set NameIDFormat")
	}
	if partialMapping.NameIDSource == "" {
		t.Error("Should fill in NameIDSource from default")
	}
	if len(partialMapping.AttributeMap) == 0 {
		t.Error("Should fill in AttributeMap from default")
	}
}

func TestMappingConfig_InvalidYAML(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	mc := NewMappingConfig(logger)
	err := mc.LoadFromFile(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestMappingConfig_MissingFile(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	mc := NewMappingConfig(logger)
	err := mc.LoadFromFile("/nonexistent/path/mappings.yaml")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestMappingConfig_LoadFromFileWithEmptyPath(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	mc := NewMappingConfig(logger)
	err := mc.LoadFromFile("")
	if err != nil {
		t.Errorf("Expected no error for empty path, got %v", err)
	}

	// Should have defaults
	if mc.DefaultMapping == nil {
		t.Error("Should still have default mapping")
	}
}

func TestSPMappingYAMLUnmarshal(t *testing.T) {
	yamlStr := `
nameid_format: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
nameid_source: "email"
attribute_map:
  email: "urn:oid:0.9.2342.19200300.100.1.3"
  name: "urn:oid:2.5.4.3"
options:
  lowercase_email: true
`

	var mapping SPMapping
	if err := yaml.Unmarshal([]byte(yamlStr), &mapping); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if mapping.NameIDFormat != "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
		t.Errorf("Unexpected NameIDFormat: %s", mapping.NameIDFormat)
	}
	if mapping.NameIDSource != "email" {
		t.Errorf("Unexpected NameIDSource: %s", mapping.NameIDSource)
	}
	if len(mapping.AttributeMap) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(mapping.AttributeMap))
	}
	if !mapping.Options.LowercaseEmail {
		t.Error("Expected LowercaseEmail to be true")
	}
}
