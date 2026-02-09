package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
)

// mockDatabase is a mock implementation of Database for testing
type mockDatabase struct {
	sessions         map[string]*saml.Session
	serviceProviders map[string]*saml.EntityDescriptor
}

func newMockDatabase() *mockDatabase {
	return &mockDatabase{
		sessions:         make(map[string]*saml.Session),
		serviceProviders: make(map[string]*saml.EntityDescriptor),
	}
}

func (m *mockDatabase) SaveSession(session *saml.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockDatabase) GetSession(sessionID string) *saml.Session {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil
	}
	// Check if expired
	if session.ExpireTime.Before(time.Now()) {
		return nil
	}
	return session
}

func (m *mockDatabase) SaveServiceProvider(entityID, acsURL, acsBinding string) error {
	m.serviceProviders[entityID] = &saml.EntityDescriptor{
		EntityID: entityID,
		SPSSODescriptors: []saml.SPSSODescriptor{
			{
				AssertionConsumerServices: []saml.IndexedEndpoint{
					{
						Binding:  acsBinding,
						Location: acsURL,
						Index:    1,
					},
				},
			},
		},
	}
	return nil
}

func (m *mockDatabase) GetServiceProvider(entityID string) (*saml.EntityDescriptor, error) {
	descriptor, ok := m.serviceProviders[entityID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return descriptor, nil
}

func (m *mockDatabase) InitSchema() error {
	return nil
}

func (m *mockDatabase) CleanupExpiredSessions() error {
	for id, session := range m.sessions {
		if session.ExpireTime.Before(time.Now()) {
			delete(m.sessions, id)
		}
	}
	return nil
}

// setupTestServer creates a test server with mock dependencies
func setupTestServer(t *testing.T) *Server {
	logger := zaptest.NewLogger(t).Sugar()
	
	// Create a test database connection matching the setup in database_test.go
	testDB, err := sql.Open("postgres", "postgres://saml_provider:saml_provider@localhost:5432/saml_provider_tests?sslmode=disable")
	if err != nil || testDB.Ping() != nil {
		// Use a minimal stub if no database available
		testDB = nil
	}
	
	server := &Server{
		config: Config{
			BridgeBaseURL:  "http://localhost:8082",
			BridgeBasePort: "8082",
			HydraPublicURL: "http://localhost:4444",
			ClientID:       "test-client",
			ClientSecret:   "test-secret",
		},
		logger:          logger,
		db:              NewDatabase(testDB, logger),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
	}
	
	return server
}

func TestNewServer(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	db := &sql.DB{}
	
	cfg := Config{
		BridgeBaseURL: "http://localhost:8082",
	}
	
	server, err := NewServer(cfg, logger, db)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	
	if server == nil {
		t.Fatal("Expected server instance, got nil")
	}
	
	if server.config.BridgeBaseURL != cfg.BridgeBaseURL {
		t.Errorf("Expected BridgeBaseURL %s, got %s", cfg.BridgeBaseURL, server.config.BridgeBaseURL)
	}
	
	if server.router == nil {
		t.Error("Expected router to be initialized")
	}
	
	if server.pendingRequests == nil {
		t.Error("Expected pendingRequests map to be initialized")
	}
}

func TestSetupRoutes(t *testing.T) {
	server := setupTestServer(t)
	
	// Initialize a minimal SAML IdP for testing
	// We need this to avoid nil pointer when SetupRoutes is called
	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}
	
	server.SetupRoutes()
	
	// Verify that routes are set up by walking the router
	routes := []string{}
	chi.Walk(server.router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	
	// Check that we have at least some routes registered
	if len(routes) == 0 {
		t.Error("Expected routes to be registered, got none")
	}
	
	// Verify POST route for admin endpoint
	found := false
	for _, route := range routes {
		if strings.Contains(route, "POST /admin/service-providers") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected POST /admin/service-providers route, got routes: %v", routes)
	}
}

func TestHandleServiceProviderRegistration_Success(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}
	
	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	server.SetupRoutes()
	
	// Create a test request with JSON body
	reqBody := `{
		"entity_id": "http://example.com/saml/metadata",
		"acs_url": "http://example.com/saml/acs",
		"acs_binding": "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
	}`
	
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	
	server.handleServiceProviderRegistration(rec, req)
	
	resp := rec.Result()
	
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, resp.StatusCode, string(body))
	}
	
	// Verify response body
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got '%s'", response["status"])
	}
	
	if response["entity_id"] != "http://example.com/saml/metadata" {
		t.Errorf("Expected entity_id in response, got '%s'", response["entity_id"])
	}
}

func TestHandleServiceProviderRegistration_FormData(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}
	
	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	server.SetupRoutes()
	
	// Create a test request with form data
	formData := url.Values{}
	formData.Set("entity_id", "http://example.com/saml/metadata")
	formData.Set("acs_url", "http://example.com/saml/acs")
	formData.Set("acs_binding", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST")
	
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	rec := httptest.NewRecorder()
	
	server.handleServiceProviderRegistration(rec, req)
	
	resp := rec.Result()
	
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, resp.StatusCode, string(body))
	}
}

func TestHandleServiceProviderRegistration_MissingFields(t *testing.T) {
	server := setupTestServer(t)
	server.SetupRoutes()
	
	// Create a test request with missing required fields
	reqBody := `{
		"entity_id": "http://example.com/saml/metadata"
	}`
	
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	
	server.handleServiceProviderRegistration(rec, req)
	
	resp := rec.Result()
	
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleServiceProviderRegistration_InvalidEntityID(t *testing.T) {
	server := setupTestServer(t)
	server.SetupRoutes()
	
	testCases := []struct {
		name     string
		entityID string
	}{
		{"not a URL", "not-a-url"},
		{"missing scheme", "example.com/metadata"},
		{"invalid scheme", "ftp://example.com/metadata"},
		{"no host", "http:///metadata"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]string{
				"entity_id":   tc.entityID,
				"acs_url":     "http://example.com/saml/acs",
				"acs_binding": saml.HTTPPostBinding,
			}
			
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			
			rec := httptest.NewRecorder()
			server.handleServiceProviderRegistration(rec, req)
			
			resp := rec.Result()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status %d for invalid entity_id '%s', got %d", http.StatusBadRequest, tc.entityID, resp.StatusCode)
			}
		})
	}
}

func TestHandleServiceProviderRegistration_InvalidACSURL(t *testing.T) {
	server := setupTestServer(t)
	server.SetupRoutes()
	
	testCases := []struct {
		name   string
		acsURL string
	}{
		{"not a URL", "not-a-url"},
		{"missing scheme", "example.com/acs"},
		{"invalid scheme", "ftp://example.com/acs"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]string{
				"entity_id":   "http://example.com/metadata",
				"acs_url":     tc.acsURL,
				"acs_binding": saml.HTTPPostBinding,
			}
			
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			
			rec := httptest.NewRecorder()
			server.handleServiceProviderRegistration(rec, req)
			
			resp := rec.Result()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status %d for invalid acs_url '%s', got %d", http.StatusBadRequest, tc.acsURL, resp.StatusCode)
			}
		})
	}
}

func TestHandleServiceProviderRegistration_InvalidBinding(t *testing.T) {
	server := setupTestServer(t)
	server.SetupRoutes()
	
	reqBody := map[string]string{
		"entity_id":   "http://example.com/metadata",
		"acs_url":     "http://example.com/acs",
		"acs_binding": "invalid-binding",
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	server.handleServiceProviderRegistration(rec, req)
	
	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid binding, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleServiceProviderRegistration_DefaultBinding(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}
	
	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	server.SetupRoutes()
	
	// Request without acs_binding should use default
	reqBody := map[string]string{
		"entity_id": "http://example.com/metadata",
		"acs_url":   "http://example.com/acs",
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	server.handleServiceProviderRegistration(rec, req)
	
	resp := rec.Result()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, resp.StatusCode, string(bodyBytes))
	}
}

func TestHandleOIDCCallback_MissingCode(t *testing.T) {
	server := setupTestServer(t)
	
	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	rec := httptest.NewRecorder()
	
	server.handleOIDCCallback(rec, req)
	
	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestServiceProviderAdapter_GetServiceProvider(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	mockDB := newMockDatabase()
	
	// Create a test database connection matching the setup in database_test.go
	testDB, err := sql.Open("postgres", "postgres://saml_provider:saml_provider@localhost:5432/saml_provider_tests?sslmode=disable")
	if err != nil || testDB.Ping() != nil {
		t.Skip("Skipping test: database not available")
	}
	defer testDB.Close()
	
	// Initialize schema
	db := NewDatabase(testDB, logger)
	if err := db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	adapter := &serviceProviderAdapter{db: db}
	
	entityID := "http://example.com/metadata"
	acsURL := "http://example.com/acs"
	
	// Save a service provider
	err = db.SaveServiceProvider(entityID, acsURL, saml.HTTPPostBinding)
	if err != nil {
		t.Skipf("Skipping test: cannot initialize test data: %v", err)
	}
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	
	descriptor, err := adapter.GetServiceProvider(req, entityID)
	if err != nil {
		t.Fatalf("GetServiceProvider failed: %v", err)
	}
	
	if descriptor == nil {
		t.Fatal("Expected descriptor, got nil")
	}
	
	if descriptor.EntityID != entityID {
		t.Errorf("Expected EntityID %s, got %s", entityID, descriptor.EntityID)
	}
	
	_ = mockDB // Keep mockDB reference to avoid unused variable warning
}

func TestSessionProviderAdapter_GetSession_WithValidCookie(t *testing.T) {
	server := setupTestServer(t)
	
	// Skip if database is not available
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}
	
	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	// Create a mock session in the database
	sessionID := "test-session-123"
	
	// Create and save a real session in the test database
	session := &saml.Session{
		ID:             sessionID,
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          sessionID,
		NameID:         "test@example.com",
		UserEmail:      "test@example.com",
		UserCommonName: "Test User",
		Groups:         []string{},
	}
	
	if err := server.db.SaveSession(session); err != nil {
		t.Skipf("Cannot save test session: %v", err)
	}
	
	server.pendingRequests["test-request-id"] = pendingAuthnRequest{
		samlRequest: "test-saml-request",
		relayState:  "test-relay-state",
	}
	
	adapter := &sessionProviderAdapter{server: server}
	
	// Create a request with session cookie
	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "saml_session",
		Value: sessionID,
	})
	
	rec := httptest.NewRecorder()
	
	authnRequest := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "test-request-id",
		},
		RelayState: "test-relay",
	}
	
	// Call GetSession - should retrieve the session from database
	result := adapter.GetSession(rec, req, authnRequest)
	
	// Verify the session was retrieved
	if result == nil {
		t.Fatal("Expected session to be retrieved, got nil")
	}
	
	if result.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, result.ID)
	}
	
	if result.NameID != "test@example.com" {
		t.Errorf("Expected NameID test@example.com, got %s", result.NameID)
	}
	
	// Should not have redirected
	if rec.Code == http.StatusFound {
		t.Error("Should not have redirected when session exists")
	}
}

func TestSessionProviderAdapter_GetSession_NoValidSession(t *testing.T) {
	server := setupTestServer(t)
	server.oauth2Config = &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8082/callback",
		Scopes:       []string{"openid"},
	}
	
	adapter := &sessionProviderAdapter{server: server}
	
	// Create a request without session cookie
	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=test-request", nil)
	rec := httptest.NewRecorder()
	
	authnRequest := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "test-auth-request",
		},
		RelayState: "test-relay-state",
	}
	
	result := adapter.GetSession(rec, req, authnRequest)
	
	// Should return nil and redirect to Hydra
	if result != nil {
		t.Error("Expected nil session when no valid cookie")
	}
	
	// Verify redirect occurred
	resp := rec.Result()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != 0 {
		t.Errorf("Expected redirect status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	
	// Verify pending request was stored
	if pending, ok := server.pendingRequests["test-auth-request"]; !ok {
		t.Error("Expected pending request to be stored")
	} else {
		if pending.samlRequest != "test-request" {
			t.Errorf("Expected SAMLRequest 'test-request', got '%s'", pending.samlRequest)
		}
		if pending.relayState != "test-relay-state" {
			t.Errorf("Expected RelayState 'test-relay-state', got '%s'", pending.relayState)
		}
	}
}

func TestParseURL(t *testing.T) {
	server := setupTestServer(t)
	
	testURL := "http://example.com:8080/path?query=value"
	parsed := server.parseURL(testURL)
	
	if parsed.Scheme != "http" {
		t.Errorf("Expected scheme 'http', got '%s'", parsed.Scheme)
	}
	if parsed.Host != "example.com:8080" {
		t.Errorf("Expected host 'example.com:8080', got '%s'", parsed.Host)
	}
	if parsed.Path != "/path" {
		t.Errorf("Expected path '/path', got '%s'", parsed.Path)
	}
}

func TestPendingRequestsManagement(t *testing.T) {
	server := setupTestServer(t)
	
	requestID := "test-request-123"
	pending := pendingAuthnRequest{
		samlRequest: "encoded-saml-request",
		relayState:  "test-relay",
	}
	
	// Store pending request
	server.pendingRequests[requestID] = pending
	
	// Retrieve it
	retrieved, ok := server.pendingRequests[requestID]
	if !ok {
		t.Fatal("Expected to find pending request")
	}
	
	if retrieved.samlRequest != pending.samlRequest {
		t.Errorf("Expected SAMLRequest '%s', got '%s'", pending.samlRequest, retrieved.samlRequest)
	}
	
	if retrieved.relayState != pending.relayState {
		t.Errorf("Expected RelayState '%s', got '%s'", pending.relayState, retrieved.relayState)
	}
	
	// Delete it
	delete(server.pendingRequests, requestID)
	
	// Verify deletion
	if _, ok := server.pendingRequests[requestID]; ok {
		t.Error("Expected pending request to be deleted")
	}
}

// Integration-style test for the full router
func TestRouterIntegration(t *testing.T) {
	server := setupTestServer(t)
	
	// Skip if database is not available
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}
	
	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}
	
	// Setup minimal SAML IDP to avoid nil pointer
	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}
	
	server.SetupRoutes()
	
	// Test that POST requests to /admin/service-providers work
	reqBody := `{
		"entity_id": "http://test.com/metadata",
		"acs_url": "http://test.com/acs"
	}`
	
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	
	server.router.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusCreated {
		body, _ := io.ReadAll(rec.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, string(body))
	}
}

func TestInitialize(t *testing.T) {
	// This test would need a running Hydra instance or mock OIDC provider
	// Skipping for now but showing the test structure
	t.Skip("Requires running OIDC provider")
	
	server := setupTestServer(t)
	ctx := context.Background()
	
	err := server.Initialize(ctx, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	if server.oidcVerifier == nil {
		t.Error("Expected OIDC verifier to be initialized")
	}
	
	if server.oauth2Config == nil {
		t.Error("Expected OAuth2 config to be initialized")
	}
	
	if server.samlIdp == nil {
		t.Error("Expected SAML IdP to be initialized")
	}
}
