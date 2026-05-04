package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestMappingService_ApplyMapping_NoMapping(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}

	// No mapping configured for this SP
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(nil, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return the same session unchanged
	if result != session {
		t.Error("expected same session reference when no mapping configured")
	}
}

func TestMappingService_ApplyMapping_ErrorRetrieving(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:        "test-session",
		UserEmail: "user@example.com",
	}

	// Error retrieving mapping — graceful degradation
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").
		Return(nil, errors.New("db error"))

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return original session on error (graceful degradation)
	if result != session {
		t.Error("expected same session reference on retrieval error")
	}
}

func TestMappingService_ApplyMapping_NameIDFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
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
		{
			name:           "email shorthand uses email",
			format:         "email",
			expectedFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			expectedNameID: "user@example.com",
		},
		{
			name:           "unspecified format uses email",
			format:         "unspecified",
			expectedFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
			expectedNameID: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			logger := zap.NewNop().Sugar()

			session := &domain.Session{
				ID:             "test-session",
				NameID:         "user@example.com",
				UserEmail:      "user@example.com",
				UserCommonName: "User Name",
				UserName:       "user-sub-id",
				Groups:         []string{"group1"},
			}

			mapping := &domain.AttributeMapping{
				NameIDFormat: tt.format,
			}

			mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

			svc := service.NewMappingService(mockRepo, logger)
			result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.NameIDFormat != tt.expectedFormat {
				t.Errorf("NameIDFormat = %q, want %q", result.NameIDFormat, tt.expectedFormat)
			}
			if result.NameID != tt.expectedNameID {
				t.Errorf("NameID = %q, want %q", result.NameID, tt.expectedNameID)
			}
		})
	}
}

func TestMappingService_ApplyMapping_SAMLAttributes(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}

	mapping := &domain.AttributeMapping{
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

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Built-in fields should be cleared
	if result.UserEmail != "" {
		t.Errorf("UserEmail should be cleared, got %q", result.UserEmail)
	}
	if result.UserCommonName != "" {
		t.Errorf("UserCommonName should be cleared, got %q", result.UserCommonName)
	}
	if result.UserName != "" {
		t.Errorf("UserName should be cleared, got %q", result.UserName)
	}
	if result.Groups != nil {
		t.Errorf("Groups should be nil, got %v", result.Groups)
	}

	// Should have custom attributes
	if len(result.CustomAttributes) == 0 {
		t.Fatal("expected custom attributes to be set")
	}

	// Check single-valued custom attributes
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
			t.Errorf("expected attribute %q not found", name)
		} else if actual != expectedValue {
			t.Errorf("attribute %q = %q, want %q", name, actual, expectedValue)
		}
	}

	// Check groups (multi-valued)
	found := false
	for _, attr := range result.CustomAttributes {
		if attr.Name == "memberOf" {
			found = true
			if len(attr.Values) != 2 {
				t.Errorf("memberOf values count = %d, want 2", len(attr.Values))
			}
			break
		}
	}
	if !found {
		t.Error("expected 'memberOf' attribute in custom attributes")
	}
}

func TestMappingService_ApplyMapping_LowercaseEmail(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "User@Example.COM",
		UserEmail:      "User@Example.COM",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	mapping := &domain.AttributeMapping{
		NameIDFormat: "emailAddress",
		SAMLAttributes: map[string]string{
			"email": "mail",
		},
		OIDCClaims: map[string]string{
			"email": "email",
		},
		Options: domain.MappingOptions{
			LowercaseEmail: true,
		},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NameID should be lowercased
	if result.NameID != "user@example.com" {
		t.Errorf("NameID = %q, want %q", result.NameID, "user@example.com")
	}

	// Check that the mail attribute is lowercased
	for _, attr := range result.CustomAttributes {
		if attr.Name == "mail" {
			if len(attr.Values) > 0 && attr.Values[0].Value != "user@example.com" {
				t.Errorf("mail attribute = %q, want %q", attr.Values[0].Value, "user@example.com")
			}
		}
	}
}

func TestMappingService_ApplyMapping_DefaultOIDCMapping(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	// Mapping with saml_attributes but no oidc_claims — uses default OIDC mapping
	mapping := &domain.AttributeMapping{
		SAMLAttributes: map[string]string{
			"subject": "uid",
			"email":   "mail",
		},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrMap := make(map[string]string)
	for _, attr := range result.CustomAttributes {
		if len(attr.Values) > 0 {
			attrMap[attr.Name] = attr.Values[0].Value
		}
	}

	if attrMap["uid"] != "user-sub-id" {
		t.Errorf("uid = %q, want %q", attrMap["uid"], "user-sub-id")
	}
	if attrMap["mail"] != "user@example.com" {
		t.Errorf("mail = %q, want %q", attrMap["mail"], "user@example.com")
	}
}

func TestMappingService_ApplyMapping_WithRawClaims(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		RawOIDCClaims: map[string]interface{}{
			"sub":                "user-sub-id",
			"email":              "user@example.com",
			"name":               "User Name",
			"preferred_username": "jdoe",
		},
	}

	mapping := &domain.AttributeMapping{
		SAMLAttributes: map[string]string{
			"subject":  "uid",
			"email":    "mail",
			"username": "preferredUsername",
		},
		OIDCClaims: map[string]string{
			"sub":                "subject",
			"email":              "email",
			"preferred_username": "username",
		},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrMap := make(map[string]string)
	for _, attr := range result.CustomAttributes {
		if len(attr.Values) > 0 {
			attrMap[attr.Name] = attr.Values[0].Value
		}
	}

	if attrMap["uid"] != "user-sub-id" {
		t.Errorf("uid = %q, want %q", attrMap["uid"], "user-sub-id")
	}
	if attrMap["mail"] != "user@example.com" {
		t.Errorf("mail = %q, want %q", attrMap["mail"], "user@example.com")
	}
	if attrMap["preferredUsername"] != "jdoe" {
		t.Errorf("preferredUsername = %q, want %q", attrMap["preferredUsername"], "jdoe")
	}
}

func TestMappingService_ApplyMapping_RawClaimsWithGroups(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"session-group"},
		RawOIDCClaims: map[string]interface{}{
			"sub":    "user-sub-id",
			"email":  "user@example.com",
			"groups": []interface{}{"admin", "users", "devops"},
		},
	}

	mapping := &domain.AttributeMapping{
		SAMLAttributes: map[string]string{
			"groups": "memberOf",
		},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check groups multi-valued attribute
	found := false
	for _, attr := range result.CustomAttributes {
		if attr.Name == "memberOf" {
			found = true
			if len(attr.Values) != 3 {
				t.Errorf("memberOf values count = %d, want 3", len(attr.Values))
			} else {
				expected := []string{"admin", "users", "devops"}
				for i, v := range attr.Values {
					if v.Value != expected[i] {
						t.Errorf("memberOf[%d] = %q, want %q", i, v.Value, expected[i])
					}
				}
			}
			break
		}
	}
	if !found {
		t.Error("expected 'memberOf' attribute")
	}
}

func TestMappingService_ApplyMapping_DoesNotModifyOriginal(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1"},
	}

	mapping := &domain.AttributeMapping{
		NameIDFormat: "persistent",
		SAMLAttributes: map[string]string{
			"email": "mail",
		},
		OIDCClaims: map[string]string{
			"email": "email",
		},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	_, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original session should not be modified
	if session.UserEmail != "user@example.com" {
		t.Errorf("original UserEmail was modified to %q", session.UserEmail)
	}
	if session.UserCommonName != "User Name" {
		t.Errorf("original UserCommonName was modified to %q", session.UserCommonName)
	}
	if session.NameID != "user@example.com" {
		t.Errorf("original NameID was modified to %q", session.NameID)
	}
	if session.UserName != "user-sub-id" {
		t.Errorf("original UserName was modified to %q", session.UserName)
	}
	if len(session.Groups) != 1 || session.Groups[0] != "group1" {
		t.Errorf("original Groups was modified to %v", session.Groups)
	}
}

func TestMappingService_ApplyMapping_OnlyNameIDFormat(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1"},
	}

	mapping := &domain.AttributeMapping{
		NameIDFormat: "persistent",
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NameIDFormat != "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent" {
		t.Errorf("NameIDFormat = %q, want persistent URN", result.NameIDFormat)
	}
	if result.NameID != "user-sub-id" {
		t.Errorf("NameID = %q, want %q", result.NameID, "user-sub-id")
	}

	// Built-in fields should NOT be cleared (no SAMLAttributes configured)
	if result.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want unchanged", result.UserEmail)
	}
	if result.UserCommonName != "User Name" {
		t.Errorf("UserCommonName = %q, want unchanged", result.UserCommonName)
	}
}

func TestMappingService_ApplyMapping_EmptyMapping(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	logger := zap.NewNop().Sugar()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
	}

	// Empty mapping (all zero values)
	mapping := &domain.AttributeMapping{}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, logger)
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want unchanged", result.UserEmail)
	}
}
