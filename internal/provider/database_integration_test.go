//go:build integration

package provider

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/crewjam/saml"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap/zaptest"
)

// setupPostgresContainer starts a PostgreSQL container and returns a
// connected Database, the raw *sql.DB, and a cleanup function.
func setupPostgresContainer(t *testing.T) (*Database, *sql.DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("saml_provider_tests"),
		postgres.WithUsername("saml_provider"),
		postgres.WithPassword("saml_provider"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	logger := zaptest.NewLogger(t).Sugar()
	database := NewDatabase(db, logger)

	cleanup := func() {
		db.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}

	return database, db, cleanup
}

func TestIntegration_InitSchema(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	err := database.InitSchema()
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	var tableName string
	err = database.db.QueryRow("SELECT tablename FROM pg_tables WHERE tablename = 'sessions'").Scan(&tableName)
	if err != nil {
		t.Errorf("Sessions table not created: %v", err)
	}

	err = database.db.QueryRow("SELECT tablename FROM pg_tables WHERE tablename = 'service_providers'").Scan(&tableName)
	if err != nil {
		t.Errorf("Service providers table not created: %v", err)
	}
}

func TestIntegration_SaveAndGetSession(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	session := &saml.Session{
		ID:             "test-session-id",
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          "test-index",
		NameID:         "test@example.com",
		UserEmail:      "test@example.com",
		UserCommonName: "Test User",
		Groups:         []string{"group1", "group2"},
	}

	err := database.SaveSession(session, nil)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	retrieved, _ := database.GetSession("test-session-id")
	if retrieved == nil {
		t.Fatal("GetSession returned nil")
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.NameID != session.NameID {
		t.Errorf("Expected NameID %s, got %s", session.NameID, retrieved.NameID)
	}
	if retrieved.UserEmail != session.UserEmail {
		t.Errorf("Expected UserEmail %s, got %s", session.UserEmail, retrieved.UserEmail)
	}
	if retrieved.UserCommonName != session.UserCommonName {
		t.Errorf("Expected UserCommonName %s, got %s", session.UserCommonName, retrieved.UserCommonName)
	}
	if len(retrieved.Groups) != len(session.Groups) {
		t.Errorf("Expected %d groups, got %d", len(session.Groups), len(retrieved.Groups))
	}
}

func TestIntegration_SaveAndGetSessionWithRawClaims(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	session := &saml.Session{
		ID:             "test-session-claims",
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          "test-index",
		NameID:         "test@example.com",
		UserEmail:      "test@example.com",
		UserCommonName: "Test User",
		Groups:         []string{"group1"},
	}

	rawClaims := map[string]interface{}{
		"sub":                "user-sub-id",
		"email":              "test@example.com",
		"name":               "Test User",
		"preferred_username": "testuser",
		"given_name":         "Test",
		"family_name":        "User",
		"groups":             []interface{}{"group1"},
	}

	err := database.SaveSession(session, rawClaims)
	if err != nil {
		t.Fatalf("SaveSession with claims failed: %v", err)
	}

	retrieved, retrievedClaims := database.GetSession("test-session-claims")
	if retrieved == nil {
		t.Fatal("GetSession returned nil session")
	}
	if retrievedClaims == nil {
		t.Fatal("GetSession returned nil claims")
	}

	// Verify standard session fields
	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.UserEmail != session.UserEmail {
		t.Errorf("Expected UserEmail %s, got %s", session.UserEmail, retrieved.UserEmail)
	}

	// Verify raw claims were persisted and retrieved
	expectedClaims := map[string]string{
		"sub":                "user-sub-id",
		"email":              "test@example.com",
		"name":               "Test User",
		"preferred_username": "testuser",
		"given_name":         "Test",
		"family_name":        "User",
	}
	for key, expected := range expectedClaims {
		actual, ok := retrievedClaims[key]
		if !ok {
			t.Errorf("Expected claim %q not found in retrieved claims", key)
			continue
		}
		if actual != expected {
			t.Errorf("Claim %q: expected %q, got %v", key, expected, actual)
		}
	}

	// Verify groups array claim
	groups, ok := retrievedClaims["groups"]
	if !ok {
		t.Error("Expected 'groups' claim not found in retrieved claims")
	} else {
		groupSlice, ok := groups.([]interface{})
		if !ok {
			t.Errorf("Expected groups to be []interface{}, got %T", groups)
		} else if len(groupSlice) != 1 || groupSlice[0] != "group1" {
			t.Errorf("Expected groups [group1], got %v", groupSlice)
		}
	}
}

func TestIntegration_SaveAndGetSessionWithNilClaims(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	session := &saml.Session{
		ID:             "test-session-nil-claims",
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          "test-index",
		NameID:         "test@example.com",
		UserEmail:      "test@example.com",
		UserCommonName: "Test User",
		Groups:         []string{},
	}

	err := database.SaveSession(session, nil)
	if err != nil {
		t.Fatalf("SaveSession with nil claims failed: %v", err)
	}

	retrieved, retrievedClaims := database.GetSession("test-session-nil-claims")
	if retrieved == nil {
		t.Fatal("GetSession returned nil session")
	}
	if retrievedClaims != nil {
		t.Errorf("Expected nil claims for session saved without claims, got %v", retrievedClaims)
	}
}

func TestIntegration_GetSession_NotFound(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	retrieved, _ := database.GetSession("non-existent-id")
	if retrieved != nil {
		t.Error("Expected nil for non-existent session, got a session")
	}
}

func TestIntegration_GetSession_Expired(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	session := &saml.Session{
		ID:             "expired-session-id",
		CreateTime:     time.Now().Add(-20 * time.Minute),
		ExpireTime:     time.Now().Add(-10 * time.Minute),
		Index:          "expired-index",
		NameID:         "expired@example.com",
		UserEmail:      "expired@example.com",
		UserCommonName: "Expired User",
		Groups:         []string{},
	}

	if err := database.SaveSession(session, nil); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	retrieved, _ := database.GetSession("expired-session-id")
	if retrieved != nil {
		t.Error("Expected nil for expired session, got a session")
	}
}

func TestIntegration_CleanupExpiredSessions(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	expiredSession := &saml.Session{
		ID:             "expired-cleanup-id",
		CreateTime:     time.Now().Add(-20 * time.Minute),
		ExpireTime:     time.Now().Add(-10 * time.Minute),
		Index:          "expired-index",
		NameID:         "expired@example.com",
		UserEmail:      "expired@example.com",
		UserCommonName: "Expired User",
		Groups:         []string{},
	}

	validSession := &saml.Session{
		ID:             "valid-cleanup-id",
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          "valid-index",
		NameID:         "valid@example.com",
		UserEmail:      "valid@example.com",
		UserCommonName: "Valid User",
		Groups:         []string{},
	}

	if err := database.SaveSession(expiredSession, nil); err != nil {
		t.Fatalf("Failed to save expired session: %v", err)
	}
	if err := database.SaveSession(validSession, nil); err != nil {
		t.Fatalf("Failed to save valid session: %v", err)
	}

	if err := database.CleanupExpiredSessions(); err != nil {
		t.Fatalf("CleanupExpiredSessions failed: %v", err)
	}

	if session, _ := database.GetSession("expired-cleanup-id"); session != nil {
		t.Error("Expired session should have been cleaned up")
	}

	if session, _ := database.GetSession("valid-cleanup-id"); session == nil {
		t.Error("Valid session should still exist")
	}
}

func TestIntegration_SaveAndGetServiceProvider(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	entityID := "http://example.com/saml/metadata"
	acsURL := "http://example.com/saml/acs"
	acsBinding := saml.HTTPPostBinding

	err := database.SaveServiceProvider(entityID, acsURL, acsBinding, nil)
	if err != nil {
		t.Fatalf("SaveServiceProvider failed: %v", err)
	}

	descriptor, err := database.GetServiceProvider(entityID)
	if err != nil {
		t.Fatalf("GetServiceProvider failed: %v", err)
	}

	if descriptor == nil {
		t.Fatal("Expected service provider descriptor, got nil")
	}

	if descriptor.EntityID != entityID {
		t.Errorf("Expected EntityID %s, got %s", entityID, descriptor.EntityID)
	}

	if len(descriptor.SPSSODescriptors) == 0 {
		t.Fatal("Expected SPSSODescriptors, got empty slice")
	}

	acs := descriptor.SPSSODescriptors[0].AssertionConsumerServices
	if len(acs) == 0 {
		t.Fatal("Expected AssertionConsumerServices, got empty slice")
	}

	if acs[0].Location != acsURL {
		t.Errorf("Expected ACS URL %s, got %s", acsURL, acs[0].Location)
	}

	if acs[0].Binding != acsBinding {
		t.Errorf("Expected ACS Binding %s, got %s", acsBinding, acs[0].Binding)
	}
}

func TestIntegration_GetServiceProvider_NotFound(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	descriptor, err := database.GetServiceProvider("http://non-existent.com/metadata")
	if err == nil {
		t.Error("Expected error for non-existent service provider")
	}
	if descriptor != nil {
		t.Error("Expected nil descriptor for non-existent service provider")
	}
}

func TestIntegration_SaveServiceProvider_Update(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	entityID := "http://example.com/saml/metadata"
	acsURL1 := "http://example.com/saml/acs1"
	acsURL2 := "http://example.com/saml/acs2"
	acsBinding := saml.HTTPPostBinding

	if err := database.SaveServiceProvider(entityID, acsURL1, acsBinding, nil); err != nil {
		t.Fatalf("SaveServiceProvider failed: %v", err)
	}

	if err := database.SaveServiceProvider(entityID, acsURL2, acsBinding, nil); err != nil {
		t.Fatalf("SaveServiceProvider update failed: %v", err)
	}

	descriptor, err := database.GetServiceProvider(entityID)
	if err != nil {
		t.Fatalf("GetServiceProvider failed: %v", err)
	}

	acs := descriptor.SPSSODescriptors[0].AssertionConsumerServices[0]
	if acs.Location != acsURL2 {
		t.Errorf("Expected updated ACS URL %s, got %s", acsURL2, acs.Location)
	}
}

func TestIntegration_SaveAndGetAttributeMapping(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	entityID := "http://example.com/saml/metadata"
	acsURL := "http://example.com/saml/acs"
	acsBinding := saml.HTTPPostBinding

	mapping := &AttributeMapping{
		NameIDFormat: "persistent",
		SAMLAttributes: map[string]string{
			"subject": "uid",
			"email":   "mail",
		},
		OIDCClaims: map[string]string{
			"sub":   "subject",
			"email": "email",
		},
		Options: MappingOptions{
			LowercaseEmail: true,
		},
	}

	if err := database.SaveServiceProvider(entityID, acsURL, acsBinding, mapping); err != nil {
		t.Fatalf("SaveServiceProvider with mapping failed: %v", err)
	}

	retrieved, err := database.GetAttributeMapping(entityID)
	if err != nil {
		t.Fatalf("GetAttributeMapping failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected attribute mapping, got nil")
	}

	if retrieved.NameIDFormat != "persistent" {
		t.Errorf("Expected NameIDFormat 'persistent', got %q", retrieved.NameIDFormat)
	}

	if retrieved.SAMLAttributes["subject"] != "uid" {
		t.Errorf("Expected SAMLAttributes[subject]='uid', got %q", retrieved.SAMLAttributes["subject"])
	}

	if retrieved.SAMLAttributes["email"] != "mail" {
		t.Errorf("Expected SAMLAttributes[email]='mail', got %q", retrieved.SAMLAttributes["email"])
	}

	if !retrieved.Options.LowercaseEmail {
		t.Error("Expected LowercaseEmail to be true")
	}
}

func TestIntegration_GetAttributeMapping_NoMapping(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	entityID := "http://example.com/saml/metadata"
	acsURL := "http://example.com/saml/acs"

	if err := database.SaveServiceProvider(entityID, acsURL, saml.HTTPPostBinding, nil); err != nil {
		t.Fatalf("SaveServiceProvider failed: %v", err)
	}

	mapping, err := database.GetAttributeMapping(entityID)
	if err != nil {
		t.Fatalf("GetAttributeMapping failed: %v", err)
	}

	if mapping != nil {
		t.Error("Expected nil mapping when none configured, got non-nil")
	}
}

func TestIntegration_GetAttributeMapping_NotFound(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	mapping, err := database.GetAttributeMapping("http://non-existent.com/metadata")
	if err != nil {
		t.Fatalf("GetAttributeMapping should not error for non-existent SP: %v", err)
	}

	if mapping != nil {
		t.Error("Expected nil mapping for non-existent SP")
	}
}

func TestIntegration_InitSchema_Idempotent(t *testing.T) {
	database, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	// Call InitSchema twice — it should be idempotent
	if err := database.InitSchema(); err != nil {
		t.Fatalf("First InitSchema failed: %v", err)
	}

	if err := database.InitSchema(); err != nil {
		t.Fatalf("Second InitSchema failed (not idempotent): %v", err)
	}
}
