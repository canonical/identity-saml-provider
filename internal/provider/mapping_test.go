package provider

import (
	"testing"

	"github.com/crewjam/saml"
)

func TestNameIDFormatToURN(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"persistent", "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
		{"Persistent", "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
		{"transient", "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"},
		{"emailAddress", "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"},
		{"email", "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"},
		{"unspecified", "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"},
		{"urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
		{"", "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"},
		{"unknown", "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := nameIDFormatToURN(tc.input)
			if result != tc.expected {
				t.Errorf("nameIDFormatToURN(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestApplyAttributeMapping_NilMapping(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}

	result := applyAttributeMapping(session, nil)

	// Should return the same session unchanged
	if result != session {
		t.Error("Expected same session reference when mapping is nil")
	}
}

func TestApplyAttributeMapping_NameIDFormat(t *testing.T) {
	testCases := []struct {
		name           string
		format         string
		expectedFormat string
		expectedNameID string
	}{
		{
			name:           "persistent format uses subject",
			format:         "persistent",
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: "user-sub-id",
		},
		{
			name:           "emailAddress format uses email",
			format:         "emailAddress",
			expectedFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			expectedNameID: "user@example.com",
		},
		{
			name:           "transient format uses email as default",
			format:         "transient",
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			expectedNameID: "user@example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := &saml.Session{
				ID:             "test-session",
				NameID:         "user@example.com",
				UserEmail:      "user@example.com",
				UserCommonName: "User Name",
				UserName:       "user-sub-id",
				Groups:         []string{"group1"},
			}

			mapping := &AttributeMapping{
				NameIDFormat: tc.format,
			}

			result := applyAttributeMapping(session, mapping)

			if result.NameIDFormat != tc.expectedFormat {
				t.Errorf("Expected NameIDFormat %q, got %q", tc.expectedFormat, result.NameIDFormat)
			}
			if result.NameID != tc.expectedNameID {
				t.Errorf("Expected NameID %q, got %q", tc.expectedNameID, result.NameID)
			}
		})
	}
}

func TestApplyAttributeMapping_SAMLAttributes(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}

	mapping := &AttributeMapping{
		NameIDFormat: "emailAddress",
		SAMLAttributes: map[string]string{
			"subject": "uid",
			"email":   "mail",
			"name":    "cn",
			"groups":  "memberOf",
		},
		OIDCClaims: map[string]string{
			"sub":    "subject",
			"email":  "email",
			"name":   "name",
			"groups": "groups",
		},
	}

	result := applyAttributeMapping(session, mapping)

	// Built-in fields should be cleared
	if result.UserEmail != "" {
		t.Errorf("Expected UserEmail to be cleared, got %q", result.UserEmail)
	}
	if result.UserCommonName != "" {
		t.Errorf("Expected UserCommonName to be cleared, got %q", result.UserCommonName)
	}
	if result.UserName != "" {
		t.Errorf("Expected UserName to be cleared, got %q", result.UserName)
	}
	if result.Groups != nil {
		t.Errorf("Expected Groups to be nil, got %v", result.Groups)
	}

	// Should have custom attributes
	if len(result.CustomAttributes) == 0 {
		t.Fatal("Expected custom attributes to be set")
	}

	// Check that custom attributes contain the expected mappings
	attrMap := make(map[string]string)
	for _, attr := range result.CustomAttributes {
		if len(attr.Values) > 0 {
			attrMap[attr.Name] = attr.Values[0].Value
		}
	}

	expectedAttrs := map[string]string{
		"uid":  "user-sub-id",
		"mail": "user@example.com",
		"cn":   "User Name",
	}

	for name, expectedValue := range expectedAttrs {
		if actual, ok := attrMap[name]; !ok {
			t.Errorf("Expected attribute %q not found in custom attributes", name)
		} else if actual != expectedValue {
			t.Errorf("Attribute %q: expected value %q, got %q", name, expectedValue, actual)
		}
	}

	// Check groups (multi-valued)
	found := false
	for _, attr := range result.CustomAttributes {
		if attr.Name == "memberOf" {
			found = true
			if len(attr.Values) != 2 {
				t.Errorf("Expected 2 group values, got %d", len(attr.Values))
			}
			break
		}
	}
	if !found {
		t.Error("Expected 'memberOf' attribute in custom attributes")
	}
}

func TestApplyAttributeMapping_LowercaseEmail(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "User@Example.COM",
		UserEmail:      "User@Example.COM",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	mapping := &AttributeMapping{
		NameIDFormat: "emailAddress",
		SAMLAttributes: map[string]string{
			"email": "mail",
		},
		OIDCClaims: map[string]string{
			"email": "email",
		},
		Options: MappingOptions{
			LowercaseEmail: true,
		},
	}

	result := applyAttributeMapping(session, mapping)

	// NameID should be lowercased
	if result.NameID != "user@example.com" {
		t.Errorf("Expected lowercased NameID %q, got %q", "user@example.com", result.NameID)
	}

	// Check that the mail attribute is lowercased
	for _, attr := range result.CustomAttributes {
		if attr.Name == "mail" {
			if len(attr.Values) > 0 && attr.Values[0].Value != "user@example.com" {
				t.Errorf("Expected lowercased email attribute %q, got %q", "user@example.com", attr.Values[0].Value)
			}
		}
	}
}

func TestApplyAttributeMapping_DoesNotModifyOriginalSession(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1"},
	}

	mapping := &AttributeMapping{
		NameIDFormat: "persistent",
		SAMLAttributes: map[string]string{
			"email": "mail",
		},
		OIDCClaims: map[string]string{
			"email": "email",
		},
	}

	_ = applyAttributeMapping(session, mapping)

	// Original session should not be modified
	if session.UserEmail != "user@example.com" {
		t.Errorf("Original session UserEmail was modified to %q", session.UserEmail)
	}
	if session.UserCommonName != "User Name" {
		t.Errorf("Original session UserCommonName was modified to %q", session.UserCommonName)
	}
	if session.NameID != "user@example.com" {
		t.Errorf("Original session NameID was modified to %q", session.NameID)
	}
}

func TestApplyAttributeMapping_OnlyNameIDFormat(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1"},
	}

	mapping := &AttributeMapping{
		NameIDFormat: "persistent",
	}

	result := applyAttributeMapping(session, mapping)

	// NameID format and value should be set
	if result.NameIDFormat != "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent" {
		t.Errorf("Expected persistent NameIDFormat, got %q", result.NameIDFormat)
	}
	if result.NameID != "user-sub-id" {
		t.Errorf("Expected NameID to be sub (user-sub-id), got %q", result.NameID)
	}

	// Built-in fields should NOT be cleared (no SAMLAttributes configured)
	if result.UserEmail != "user@example.com" {
		t.Errorf("Expected UserEmail to remain, got %q", result.UserEmail)
	}
	if result.UserCommonName != "User Name" {
		t.Errorf("Expected UserCommonName to remain, got %q", result.UserCommonName)
	}
}

func TestApplyAttributeMapping_DefaultOIDCMapping(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	// Mapping with saml_attributes but no oidc_claims → uses default OIDC mapping
	mapping := &AttributeMapping{
		SAMLAttributes: map[string]string{
			"subject": "uid",
			"email":   "mail",
		},
	}

	result := applyAttributeMapping(session, mapping)

	// Check that default OIDC mapping (sub→subject, email→email) was used
	attrMap := make(map[string]string)
	for _, attr := range result.CustomAttributes {
		if len(attr.Values) > 0 {
			attrMap[attr.Name] = attr.Values[0].Value
		}
	}

	if attrMap["uid"] != "user-sub-id" {
		t.Errorf("Expected uid=%q, got %q", "user-sub-id", attrMap["uid"])
	}
	if attrMap["mail"] != "user@example.com" {
		t.Errorf("Expected mail=%q, got %q", "user@example.com", attrMap["mail"])
	}
}

func TestApplyAttributeMapping_EmptySAMLAttributes(t *testing.T) {
	session := &saml.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	// Empty mapping (all zero values) should still return a valid session
	mapping := &AttributeMapping{}

	result := applyAttributeMapping(session, mapping)

	// Session should be essentially unchanged (no SAMLAttributes, no NameIDFormat)
	if result.UserEmail != "user@example.com" {
		t.Errorf("Expected UserEmail to remain unchanged, got %q", result.UserEmail)
	}
}

func TestBuildInternalModel(t *testing.T) {
	session := &saml.Session{
		UserName:       "user-sub-id",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		Groups:         []string{"group1", "group2"},
	}

	// Test with default OIDC mapping
	model := buildInternalModel(session, nil)

	if model["subject"] != "user-sub-id" {
		t.Errorf("Expected subject=%q, got %q", "user-sub-id", model["subject"])
	}
	if model["email"] != "user@example.com" {
		t.Errorf("Expected email=%q, got %q", "user@example.com", model["email"])
	}
	if model["name"] != "User Name" {
		t.Errorf("Expected name=%q, got %q", "User Name", model["name"])
	}
	if model["groups"] == "" {
		t.Error("Expected groups to be populated")
	}
}

func TestBuildInternalModel_CustomOIDCMapping(t *testing.T) {
	session := &saml.Session{
		UserName:       "user-sub-id",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
	}

	// Custom mapping: use email as the "identifier" internal field
	customClaims := map[string]string{
		"email": "identifier",
		"name":  "display_name",
	}

	model := buildInternalModel(session, customClaims)

	if model["identifier"] != "user@example.com" {
		t.Errorf("Expected identifier=%q, got %q", "user@example.com", model["identifier"])
	}
	if model["display_name"] != "User Name" {
		t.Errorf("Expected display_name=%q, got %q", "User Name", model["display_name"])
	}
	// "sub" is not in the custom mapping, so "subject" should not be present
	if _, ok := model["subject"]; ok {
		t.Error("Expected 'subject' to not be in model when 'sub' is not in custom OIDC mapping")
	}
}

func TestGetNameIDValue(t *testing.T) {
	model := map[string]string{
		"subject": "user-sub-id",
		"email":   "user@example.com",
	}

	testCases := []struct {
		format   string
		expected string
	}{
		{"persistent", "user-sub-id"},
		{"emailAddress", "user@example.com"},
		{"email", "user@example.com"},
		{"transient", "user@example.com"}, // defaults to email
		{"", "user@example.com"},          // defaults to email
	}

	for _, tc := range testCases {
		t.Run(tc.format, func(t *testing.T) {
			result := getNameIDValue(model, tc.format)
			if result != tc.expected {
				t.Errorf("getNameIDValue(%q) = %q, want %q", tc.format, result, tc.expected)
			}
		})
	}
}

func TestGetNameIDValue_MissingFields(t *testing.T) {
	// Model with only subject, no email
	model := map[string]string{
		"subject": "user-sub-id",
	}

	result := getNameIDValue(model, "emailAddress")
	if result != "" {
		t.Errorf("Expected empty string for emailAddress with no email, got %q", result)
	}

	// Default should fall back to subject when email is missing
	result = getNameIDValue(model, "")
	if result != "user-sub-id" {
		t.Errorf("Expected %q for default format, got %q", "user-sub-id", result)
	}
}
