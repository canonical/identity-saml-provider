package provider

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/migrations"
	"github.com/crewjam/saml"
	_ "github.com/lib/pq"
	"go.uber.org/zap/zaptest"
)

// setupTestDB creates a test database connection and runs migrations
func setupTestDB(t *testing.T) (*Database, *sql.DB, func()) {
	logger := zaptest.NewLogger(t).Sugar()

	db, err := sql.Open("postgres", "postgres://saml_provider:saml_provider@localhost:5432/saml_provider_tests?sslmode=disable")
	if err != nil {
		t.Skip("Skipping database tests: PostgreSQL not available")
		return nil, nil, func() {}
	}

	if err := db.Ping(); err != nil {
		t.Skip("Skipping database tests: Cannot connect to PostgreSQL")
		return nil, nil, func() {}
	}

	// Run goose migrations
	if err := migrations.RunMigrationsUp(context.Background(), db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	database := NewDatabase(db, logger)

	cleanup := func() {
		db.Exec("DROP TABLE IF EXISTS sessions")
		db.Exec("DROP TABLE IF EXISTS service_providers")
		db.Exec("DROP TABLE IF EXISTS goose_db_version")
		db.Close()
	}

	return database, db, cleanup
}

func TestNewDatabase(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	db := &sql.DB{}

	database := NewDatabase(db, logger)

	if database == nil {
		t.Fatal("Expected database instance, got nil")
	}
	if database.db != db {
		t.Error("Database db field not set correctly")
	}
	if database.logger != logger {
		t.Error("Database logger field not set correctly")
	}
}

func TestSaveAndGetSession(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestSaveAndGetSessionWithRawClaims(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestSaveAndGetSessionWithNilClaims(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestGetSession_NotFound(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

	retrieved, _ := database.GetSession("non-existent-id")
	if retrieved != nil {
		t.Error("Expected nil for non-existent session, got a session")
	}
}

func TestGetSession_Expired(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestCleanupExpiredSessions(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestSaveAndGetServiceProvider(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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

func TestGetServiceProvider_NotFound(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

	descriptor, err := database.GetServiceProvider("http://non-existent.com/metadata")
	if err == nil {
		t.Error("Expected error for non-existent service provider")
	}
	if descriptor != nil {
		t.Error("Expected nil descriptor for non-existent service provider")
	}
}

func TestSaveServiceProvider_Update(t *testing.T) {
	database, _, cleanup := setupTestDB(t)
	if database == nil {
		return
	}
	defer cleanup()

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
