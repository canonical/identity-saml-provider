// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/canonical/identity-saml-provider/mocks"
	"go.uber.org/mock/gomock"
)

// attrMap collects single-valued attributes into a name->value map.
func attrMap(attrs []domain.Attribute) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, a := range attrs {
		if len(a.Values) > 0 {
			out[a.Name] = a.Values[0].Value
		}
	}
	return out
}

// findAttr returns the first attribute with the given Name.
func findAttr(attrs []domain.Attribute, name string) (domain.Attribute, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a, true
		}
	}
	return domain.Attribute{}, false
}

func TestMappingService_ApplyMapping_NoMapping(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		ID:             "test-session",
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}

	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "https://sp.example.com").Return(nil, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "https://sp.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != session {
		t.Error("expected same session reference when no mapping configured")
	}
}

func TestMappingService_ApplyMapping_ErrorRetrieving(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{UserEmail: "user@example.com"}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").
		Return(nil, errors.New("db error"))

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != session {
		t.Error("expected same session reference on retrieval error")
	}
}

func TestMappingService_ApplyMapping_NameIDFormats(t *testing.T) {
	t.Parallel()

	// Sentinel for the persistent format's expected NameID.
	const persistentSentinel = "fixed-persistent-uuid-abc-123"

	tests := []struct {
		name           string
		format         string
		persistentType string
		expectedFormat string
		// expectedNameID is the exact value expected. Empty string means
		// "don't check exact value — the assertion below applies extra
		// constraints" (used for transient which returns a fresh UUID).
		expectedNameID string
		mockPersistent bool
	}{
		{
			name:           "persistent format with pairwise type calls persistent repo",
			format:         "persistent",
			persistentType: domain.PersistentTypePairwise,
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: persistentSentinel,
			mockPersistent: true,
		},
		{
			name:           "persistent format default (public) returns OIDC sub directly",
			format:         "persistent",
			persistentType: "",
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: "alice-oidc-sub",
			mockPersistent: false,
		},
		{
			name:           "persistent format explicit public type returns OIDC sub directly",
			format:         "persistent",
			persistentType: domain.PersistentTypePublic,
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: "alice-oidc-sub",
			mockPersistent: false,
		},
		{
			name:           "emailAddress format uses email",
			format:         "emailAddress",
			expectedFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			expectedNameID: "user@example.com",
		},
		{
			name:           "transient format generates fresh UUID",
			format:         "transient",
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			// expectedNameID intentionally empty — see assertion below.
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
			mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
			logger := logging.NewNopLogger()

			session := &domain.Session{
				NameID:         "user@example.com",
				UserEmail:      "user@example.com",
				UserCommonName: "User Name",
				UserName:       "user-sub-id",
				Groups:         []string{"group1"},
				RawOIDCClaims: map[string]interface{}{
					"sub": "alice-oidc-sub",
				},
			}
			mapping := &domain.AttributeMapping{
				NameIDFormat:   tt.format,
				PersistentType: tt.persistentType,
			}
			mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)
			if tt.mockPersistent {
				mockPID.EXPECT().
					GetOrCreate(gomock.Any(), "sp1", "alice-oidc-sub").
					Return(persistentSentinel, nil)
			}

			svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
			result, err := svc.ApplyMapping(context.Background(), session, "sp1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.NameIDFormat != tt.expectedFormat {
				t.Errorf("NameIDFormat = %q, want %q", result.NameIDFormat, tt.expectedFormat)
			}
			if tt.expectedNameID != "" {
				if result.NameID != tt.expectedNameID {
					t.Errorf("NameID = %q, want %q", result.NameID, tt.expectedNameID)
				}
				return
			}
			// transient: assert it parses as a UUID and is not the email.
			if _, err := uuid.Parse(result.NameID); err != nil {
				t.Errorf("NameID = %q, want valid UUID: %v", result.NameID, err)
			}
			if result.NameID == "user@example.com" {
				t.Error("transient NameID must not equal user email")
			}
		})
	}
}

func TestMappingService_ApplyMapping_SAMLAttributes_AllFields(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1", "group2"},
	}
	mapping := &domain.AttributeMapping{
		NameIDFormat: "emailAddress",
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"subject": {Name: "uid"},
			"email":   {Name: "mail"},
			"name":    {Name: "cn"},
			"groups":  {Name: "memberOf"},
		},
		OIDCClaimMappings: map[string]string{
			"sub":    "subject",
			"email":  "email",
			"name":   "name",
			"groups": "groups",
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UserEmail != session.UserEmail {
		t.Errorf("expected built-in fields preserved, got %+v", result)
	}

	got := attrMap(result.CustomAttributes)
	wantSingle := map[string]string{
		"uid":  "user-sub-id",
		"mail": "user@example.com",
		"cn":   "User Name",
	}
	for name, val := range wantSingle {
		if got[name] != val {
			t.Errorf("attribute %q = %q, want %q", name, got[name], val)
		}
	}

	groupsAttr, ok := findAttr(result.CustomAttributes, "memberOf")
	if !ok {
		t.Fatal("expected memberOf attribute")
	}
	if len(groupsAttr.Values) != 2 {
		t.Errorf("memberOf values count = %d, want 2", len(groupsAttr.Values))
	}
}

func TestMappingService_ApplyMapping_SAMLAttributeDef_Honoured(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		UserEmail: "alice@example.com",
		UserName:  "alice-sub",
	}
	mapping := &domain.AttributeMapping{
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email": {
				Name:         "urn:oid:0.9.2342.19200300.100.1.3",
				FriendlyName: "mail",
				NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			},
			"subject": {Name: "uid"}, // no FriendlyName or NameFormat — default applies
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emailAttr, ok := findAttr(result.CustomAttributes, "urn:oid:0.9.2342.19200300.100.1.3")
	if !ok {
		t.Fatal("expected email attribute by OID name")
	}
	if emailAttr.NameFormat != "urn:oasis:names:tc:SAML:2.0:attrname-format:basic" {
		t.Errorf("email NameFormat = %q, want explicit basic", emailAttr.NameFormat)
	}
	if emailAttr.FriendlyName != "mail" {
		t.Errorf("email FriendlyName = %q, want %q", emailAttr.FriendlyName, "mail")
	}

	subjAttr, ok := findAttr(result.CustomAttributes, "uid")
	if !ok {
		t.Fatal("expected subject attribute by name 'uid'")
	}
	if subjAttr.NameFormat != domain.DefaultNameFormat {
		t.Errorf("subject NameFormat = %q, want DefaultNameFormat %q",
			subjAttr.NameFormat, domain.DefaultNameFormat)
	}
	if subjAttr.FriendlyName != "" {
		t.Errorf("subject FriendlyName = %q, want empty", subjAttr.FriendlyName)
	}
}

func TestMappingService_ApplyMapping_GroupsMultiValued(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		RawOIDCClaims: map[string]interface{}{
			"groups": []interface{}{"admin", "users", "devops"},
		},
	}
	mapping := &domain.AttributeMapping{
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"groups": {Name: "memberOf"},
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matching := 0
	for _, a := range result.CustomAttributes {
		if a.Name == "memberOf" {
			matching++
			if len(a.Values) != 3 {
				t.Errorf("memberOf values count = %d, want 3", len(a.Values))
			}
		}
	}
	if matching != 1 {
		t.Errorf("expected exactly one memberOf <saml:Attribute>, got %d", matching)
	}
}

func TestMappingService_ApplyMapping_EmptySourceOmits(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{UserEmail: "alice@example.com"}
	mapping := &domain.AttributeMapping{
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email": {Name: "mail"},
			"name":  {Name: "cn"},
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := findAttr(result.CustomAttributes, "cn"); ok {
		t.Error("expected cn attribute omitted when source empty")
	}
	if _, ok := findAttr(result.CustomAttributes, "mail"); !ok {
		t.Error("expected mail attribute present")
	}
}

func TestMappingService_ApplyMapping_LowercaseEmail_OnlyEmail(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		UserEmail:      "User@Example.COM",
		UserCommonName: "User Name",
		UserName:       "User-SUB-ID",
	}
	mapping := &domain.AttributeMapping{
		NameIDFormat: "emailAddress",
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email":   {Name: "mail"},
			"name":    {Name: "cn"},
			"subject": {Name: "uid"},
		},
		Options: domain.MappingOptions{LowercaseEmail: true},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := attrMap(result.CustomAttributes)
	if got["mail"] != "user@example.com" {
		t.Errorf("mail = %q, want lowercased", got["mail"])
	}
	if got["cn"] != "User Name" {
		t.Errorf("cn = %q, want untouched", got["cn"])
	}
	if got["uid"] != "User-SUB-ID" {
		t.Errorf("uid = %q, want case preserved", got["uid"])
	}
	if result.NameID != "user@example.com" {
		t.Errorf("NameID = %q, want lowercased", result.NameID)

	}
}

func TestMappingService_ApplyMapping_FieldClearing_PreservedWhenOnlyNameID(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		UserEmail:      "u@e.com",
		UserCommonName: "cn",
		UserName:       "sub",
		Groups:         []string{"g1"},
	}
	mapping := &domain.AttributeMapping{NameIDFormat: "transient"}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UserEmail == "" || result.UserCommonName == "" || result.UserName == "" {
		t.Errorf("expected built-in fields preserved, got %+v", result)
	}
	if len(result.Groups) != 1 {
		t.Errorf("expected Groups preserved, got %v", result.Groups)
	}
}

func TestMappingService_ApplyMapping_DoesNotModifyOriginal(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		NameID:         "user@example.com",
		UserEmail:      "user@example.com",
		UserCommonName: "User Name",
		UserName:       "user-sub-id",
		Groups:         []string{"group1"},
		RawOIDCClaims:  map[string]interface{}{"sub": "user-sub-id"},
	}
	mapping := &domain.AttributeMapping{
		NameIDFormat:   "persistent",
		PersistentType: "pairwise",
		SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
			"email": {Name: "mail"},
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)
	mockPID.EXPECT().
		GetOrCreate(gomock.Any(), "sp1", "user-sub-id").
		Return("opaque-persistent-id", nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	if _, err := svc.ApplyMapping(context.Background(), session, "sp1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.UserEmail != "user@example.com" || session.UserCommonName != "User Name" ||
		session.NameID != "user@example.com" || session.UserName != "user-sub-id" ||
		len(session.Groups) != 1 || session.Groups[0] != "group1" {
		t.Errorf("original session was mutated: %+v", session)
	}
}

func TestMappingService_ApplyMapping_EmptyMapping(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{UserEmail: "user@example.com"}
	mapping := &domain.AttributeMapping{} // all zero
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want unchanged", result.UserEmail)
	}
}

func TestBuildUserAttributes_DefaultMapping(t *testing.T) {
	t.Parallel()
	session := &domain.Session{
		RawOIDCClaims: map[string]interface{}{
			"sub":   "alice-sub",
			"email": "alice@example.com",
			"name":  "Alice",
		},
	}

	attrs := service.BuildUserAttributes(session, nil, session.RawOIDCClaims)

	if attrs.Subject != "alice-sub" {
		t.Errorf("Subject = %q, want %q", attrs.Subject, "alice-sub")
	}
	if attrs.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", attrs.Email, "alice@example.com")
	}
	if attrs.Name != "Alice" {
		t.Errorf("Name = %q, want %q", attrs.Name, "Alice")
	}
}

func TestBuildUserAttributes_CustomClaimRoutesToCustom(t *testing.T) {
	t.Parallel()
	session := &domain.Session{
		RawOIDCClaims: map[string]interface{}{
			"department": "eng",
		},
	}
	mappings := map[string]string{"department": "dept"}

	attrs := service.BuildUserAttributes(session, mappings, session.RawOIDCClaims)

	if got := attrs.Custom["dept"]; got != "eng" {
		t.Errorf("Custom[dept] = %q, want %q", got, "eng")
	}
}

func TestBuildUserAttributes_GroupsAsNativeSlice(t *testing.T) {
	t.Parallel()
	session := &domain.Session{
		RawOIDCClaims: map[string]interface{}{
			"groups": []interface{}{"admin", "users"},
		},
	}

	attrs := service.BuildUserAttributes(session, nil, session.RawOIDCClaims)

	want := []string{"admin", "users"}
	if !reflect.DeepEqual(attrs.Groups, want) {
		t.Errorf("Groups = %v, want %v", attrs.Groups, want)
	}
	for _, g := range attrs.Groups {
		if strings.Contains(g, "\x00") {
			t.Errorf("group %q contains null byte", g)
		}
	}
}

func TestBuildUserAttributes_MissingClaimLeavesEmpty(t *testing.T) {
	t.Parallel()
	session := &domain.Session{}

	attrs := service.BuildUserAttributes(session, nil, nil)

	if attrs.Email != "" || attrs.Subject != "" || attrs.Name != "" {
		t.Errorf("expected all fields empty, got %+v", attrs)
	}
	if len(attrs.Groups) != 0 {
		t.Errorf("expected Groups empty, got %v", attrs.Groups)
	}
}

func TestBuildUserAttributes_SessionFallback(t *testing.T) {
	t.Parallel()
	session := &domain.Session{
		UserEmail:      "fallback@example.com",
		UserCommonName: "Fallback Name",
		UserName:       "fallback-sub",
	}

	attrs := service.BuildUserAttributes(session, nil, nil)

	if attrs.Email != "fallback@example.com" {
		t.Errorf("Email = %q, want fallback", attrs.Email)
	}
	if attrs.Subject != "fallback-sub" {
		t.Errorf("Subject = %q, want fallback", attrs.Subject)
	}
	if attrs.Name != "Fallback Name" {
		t.Errorf("Name = %q, want fallback", attrs.Name)
	}
}

func TestBuildUserAttributes_CustomClaimFallsBackByInternalField(t *testing.T) {
	t.Parallel()
	session := &domain.Session{
		UserName:       "alice-sub",
		UserEmail:      "alice@example.com",
		UserCommonName: "Alice",
	}
	mappings := map[string]string{
		"preferred_username": "subject",
		"emails[0]":          "email",
		"display_name":       "name",
	}

	attrs := service.BuildUserAttributes(session, mappings, nil)

	if attrs.Subject != "alice-sub" {
		t.Errorf("Subject = %q, want fallback to UserName", attrs.Subject)
	}
	if attrs.Email != "alice@example.com" {
		t.Errorf("Email = %q, want fallback to UserEmail", attrs.Email)
	}
	if attrs.Name != "Alice" {
		t.Errorf("Name = %q, want fallback to UserCommonName", attrs.Name)
	}
}

func TestMappingService_ApplyMapping_Persistent_FailsClosed_MissingSub(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	// Session has RawOIDCClaims but no "sub" key — must fail closed and
	// must NOT call the persistent repo.
	session := &domain.Session{
		UserEmail:     "alice@example.com",
		UserName:      "alice-sub",
		RawOIDCClaims: map[string]interface{}{"email": "alice@example.com"},
	}
	mapping := &domain.AttributeMapping{NameIDFormat: "persistent"}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)
	// Explicit: mockPID receives no calls.
	mockPID.EXPECT().GetOrCreate(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil session on fail-closed, got %+v", result)
	}
	var resErr *domain.ErrNameIDResolution
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *domain.ErrNameIDResolution, got %T (%v)", err, err)
	}
	if resErr.EntityID != "sp1" {
		t.Errorf("EntityID = %q, want %q", resErr.EntityID, "sp1")
	}
	if resErr.Format != "persistent" {
		t.Errorf("Format = %q, want %q", resErr.Format, "persistent")
	}
	if !strings.Contains(resErr.Reason, "sub") {
		t.Errorf("Reason = %q, want it to mention the missing sub claim", resErr.Reason)
	}
}

func TestMappingService_ApplyMapping_Persistent_FailsClosed_RepoError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		UserEmail:     "alice@example.com",
		UserName:      "alice-sub",
		RawOIDCClaims: map[string]interface{}{"sub": "alice-oidc-sub"},
	}
	mapping := &domain.AttributeMapping{
		NameIDFormat:   "persistent",
		PersistentType: "pairwise",
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	repoErr := errors.New("database unreachable")
	mockPID.EXPECT().
		GetOrCreate(gomock.Any(), "sp1", "alice-oidc-sub").
		Return("", repoErr)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil session on fail-closed, got %+v", result)
	}
	// Wrapped repo error must be reachable via errors.Is.
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repo error to be reachable via errors.Is, got %v", err)
	}
	// Must be a typed nameid resolution error.
	var resErr *domain.ErrNameIDResolution
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *domain.ErrNameIDResolution, got %T", err)
	}
	if resErr.Err == nil {
		t.Error("expected wrapped Err field to be set")
	}
}

func TestMappingService_ApplyMapping_Persistent_UsesCanonicalNotMappedSubject(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	// Admin has remapped: email → subject. The mapped attrs.Subject will
	// therefore equal the email value. The persistent NameID lookup MUST
	// still key on the raw OIDC "sub" claim, not the mapped subject.
	session := &domain.Session{
		UserEmail: "alice@example.com",
		UserName:  "ignored",
		RawOIDCClaims: map[string]interface{}{
			"sub":   "stable-oidc-sub-key",
			"email": "alice@example.com",
		},
	}
	mapping := &domain.AttributeMapping{
		NameIDFormat:   "persistent",
		PersistentType: "pairwise",
		OIDCClaimMappings: map[string]string{
			"email": "subject", // re-route: subject is now sourced from email
		},
	}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	// The assertion: GetOrCreate must be called with the raw OIDC sub,
	// NOT "alice@example.com" (which would be attrs.Subject).
	mockPID.EXPECT().
		GetOrCreate(gomock.Any(), "sp1", "stable-oidc-sub-key").
		Return("persistent-uuid", nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NameID != "persistent-uuid" {
		t.Errorf("NameID = %q, want %q", result.NameID, "persistent-uuid")
	}
}

func TestMappingService_ApplyMapping_Transient_GeneratesUniqueUUIDs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		UserEmail: "alice@example.com",
		UserName:  "alice-sub",
	}
	mapping := &domain.AttributeMapping{NameIDFormat: "transient"}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil).Times(2)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())

	r1, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := uuid.Parse(r1.NameID); err != nil {
		t.Errorf("transient NameID #1 not a UUID: %v", err)
	}
	if _, err := uuid.Parse(r2.NameID); err != nil {
		t.Errorf("transient NameID #2 not a UUID: %v", err)
	}
	if r1.NameID == r2.NameID {
		t.Errorf("transient NameIDs must differ across authentications, both were %q", r1.NameID)
	}
}

func TestMappingService_ApplyMapping_EmailAddress_FailsClosed_EmptyEmail(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
	mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
	logger := logging.NewNopLogger()

	session := &domain.Session{
		// No UserEmail, no email in raw claims — attrs.Email will be empty.
		UserName: "alice-sub",
	}
	mapping := &domain.AttributeMapping{NameIDFormat: "emailAddress"}
	mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)

	svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
	result, err := svc.ApplyMapping(context.Background(), session, "sp1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil session, got %+v", result)
	}
	var resErr *domain.ErrNameIDResolution
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *domain.ErrNameIDResolution, got %T", err)
	}
	if resErr.EntityID != "sp1" {
		t.Errorf("EntityID = %q, want %q", resErr.EntityID, "sp1")
	}
}

func TestMappingService_ApplyMapping_FullURNFormats(t *testing.T) {
	t.Parallel()

	const persistentSentinel = "fixed-persistent-uuid-urn"

	tests := []struct {
		name           string
		format         string
		persistentType string
		expectedFormat string
		expectedNameID string
		mockPersistent bool
	}{
		{
			name:           "persistent full URN dispatches to persistent pairwise branch",
			format:         "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			persistentType: domain.PersistentTypePairwise,
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: persistentSentinel,
			mockPersistent: true,
		},
		{
			name:           "persistent full URN dispatches to persistent public branch",
			format:         "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			persistentType: domain.PersistentTypePublic,
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			expectedNameID: "alice-oidc-sub",
			mockPersistent: false,
		},
		{
			name:           "emailAddress full URN dispatches to email branch",
			format:         "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			expectedFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			expectedNameID: "user@example.com",
		},
		{
			name:           "transient full URN dispatches to transient branch",
			format:         "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			expectedFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			// expectedNameID empty — UUID asserted below.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockRepo := mocks.NewMockServiceProviderRepository(ctrl)
			mockPID := mocks.NewMockPersistentNameIDRepository(ctrl)
			logger := logging.NewNopLogger()

			session := &domain.Session{
				UserEmail: "user@example.com",
				UserName:  "user-sub-id",
				RawOIDCClaims: map[string]interface{}{
					"sub": "alice-oidc-sub",
				},
			}
			mapping := &domain.AttributeMapping{
				NameIDFormat:   tt.format,
				PersistentType: tt.persistentType,
			}
			mockRepo.EXPECT().GetAttributeMapping(gomock.Any(), "sp1").Return(mapping, nil)
			if tt.mockPersistent {
				mockPID.EXPECT().
					GetOrCreate(gomock.Any(), "sp1", "alice-oidc-sub").
					Return(persistentSentinel, nil)
			}

			svc := service.NewMappingService(mockRepo, mockPID, logger, tracing.NewNoopTracer())
			result, err := svc.ApplyMapping(context.Background(), session, "sp1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.NameIDFormat != tt.expectedFormat {
				t.Errorf("NameIDFormat = %q, want %q", result.NameIDFormat, tt.expectedFormat)
			}
			if tt.expectedNameID != "" {
				if result.NameID != tt.expectedNameID {
					t.Errorf("NameID = %q, want %q", result.NameID, tt.expectedNameID)
				}
				return
			}
			if _, err := uuid.Parse(result.NameID); err != nil {
				t.Errorf("NameID = %q, want UUID: %v", result.NameID, err)
			}
		})
	}
}
