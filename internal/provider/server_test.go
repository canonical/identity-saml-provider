package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/internal/tracing"
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

func newHydraStubServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// setupTestServer creates a test server with mock dependencies
func setupTestServer(t *testing.T) *Server {
	logger := zaptest.NewLogger(t).Sugar()
	hydraStub := newHydraStubServer(t, nil)

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
			HydraPublicURL: hydraStub.URL,
			ClientID:       "test-client",
			ClientSecret:   "test-secret",
		},
		logger:          logger,
		db:              NewDatabase(testDB, logger),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
		hydraHTTPClient: hydraStub.Client(),
		monitor:         monitoring.NewNoopMonitor("identity-saml-provider", logger),
		tracer:          tracing.NewNoopTracer(),
	}

	return server
}

func TestNewServer(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	db := &sql.DB{}

	cfg := Config{
		BridgeBaseURL: "http://localhost:8082",
	}

	server, err := NewServer(
		cfg,
		logger,
		db,
		monitoring.NewNoopMonitor("identity-saml-provider", logger),
		tracing.NewNoopTracer(),
	)
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
		RedirectURL:  "http://localhost:8082/saml/callback",
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

func TestWithHydraHTTPClient_WithClient(t *testing.T) {
	server := setupTestServer(t)

	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	server.hydraHTTPClient = customClient

	ctx := context.Background()
	result := server.withHydraHTTPClient(ctx)

	// Should return a new context with the client embedded
	retrievedClient := result.Value(oauth2.HTTPClient)
	if retrievedClient == nil {
		t.Fatal("Expected HTTP client to be in context")
	}

	if retrievedClient.(*http.Client) != customClient {
		t.Error("Expected retrieved client to match the hydraHTTPClient")
	}
}

func TestWithHydraHTTPClient_PreservesExistingValues(t *testing.T) {
	server := setupTestServer(t)

	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	server.hydraHTTPClient = customClient

	// Create a context with an existing value
	ctx := context.WithValue(context.Background(), "testKey", "testValue")
	result := server.withHydraHTTPClient(ctx)

	// Should preserve the existing value
	if result.Value("testKey") != "testValue" {
		t.Error("Expected existing context value to be preserved")
	}

	// Should also have the HTTP client
	if result.Value(oauth2.HTTPClient) == nil {
		t.Error("Expected HTTP client to be in context")
	}
}

func TestHandleOIDCCallback_NoCode(t *testing.T) {
	server := setupTestServer(t)

	// Request without code parameter
	req := httptest.NewRequest(http.MethodGet, "/saml/callback?state=test", nil)
	rec := httptest.NewRecorder()

	server.handleOIDCCallback(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No code in callback") {
		t.Errorf("Expected error message about missing code, got: %s", string(body))
	}
}

func TestHandleOIDCCallback_InvalidCode(t *testing.T) {
	server := setupTestServer(t)
	hydraStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			http.Error(w, "stubbed token exchange failure", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer hydraStub.Close()

	server.hydraHTTPClient = hydraStub.Client()
	server.oauth2Config = &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8082/saml/callback",
		Scopes:       []string{"openid"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  hydraStub.URL + "/oauth2/auth",
			TokenURL: hydraStub.URL + "/oauth2/token",
		},
	}

	// Request with invalid code
	req := httptest.NewRequest(http.MethodGet, "/saml/callback?code=invalid-code&state=test", nil)
	rec := httptest.NewRecorder()

	server.handleOIDCCallback(rec, req)

	resp := rec.Result()
	// Should get an error because the token exchange fails against the stub server.
	if resp.StatusCode < 400 {
		t.Errorf("Expected error status, got %d", resp.StatusCode)
	}
}

func TestSessionProviderAdapter_GetSession_WithExpiredSession(t *testing.T) {
	server := setupTestServer(t)

	// Skip if database is not available
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}

	// Initialize database schema
	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}

	server.oauth2Config = &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8082/saml/callback",
		Scopes:       []string{"openid"},
	}

	// Create an expired session
	sessionID := "expired-session-123"
	expiredSession := &saml.Session{
		ID:             sessionID,
		CreateTime:     time.Now().Add(-20 * time.Minute),
		ExpireTime:     time.Now().Add(-10 * time.Minute), // Already expired
		Index:          sessionID,
		NameID:         "test@example.com",
		UserEmail:      "test@example.com",
		UserCommonName: "Test User",
		Groups:         []string{},
	}

	if err := server.db.SaveSession(expiredSession); err != nil {
		t.Skipf("Cannot save test session: %v", err)
	}

	adapter := &sessionProviderAdapter{server: server}

	// Create a request with expired session cookie
	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "saml_session",
		Value: sessionID,
	})

	rec := httptest.NewRecorder()

	authnRequest := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "test-auth-request",
		},
		RelayState: "test-relay",
	}

	result := adapter.GetSession(rec, req, authnRequest)

	// Should return nil for expired session
	if result != nil {
		t.Error("Expected nil session for expired session")
	}

	// Should have stored pending request
	if _, ok := server.pendingRequests["test-auth-request"]; !ok {
		t.Error("Expected pending request to be stored")
	}
}

func TestSessionProviderAdapter_GetSession_WithRelayState(t *testing.T) {
	server := setupTestServer(t)
	server.oauth2Config = &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8082/saml/callback",
		Scopes:       []string{"openid"},
	}

	adapter := &sessionProviderAdapter{server: server}

	// Create a request with RelayState in the AuthnRequest
	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=encoded-request", nil)
	rec := httptest.NewRecorder()

	authnRequest := &saml.IdpAuthnRequest{
		Request: saml.AuthnRequest{
			ID: "test-request-with-relay",
		},
		RelayState: "my-relay-state",
	}

	result := adapter.GetSession(rec, req, authnRequest)

	// Should return nil and redirect
	if result != nil {
		t.Error("Expected nil session")
	}

	// Should preserve RelayState in pending requests
	if pending, ok := server.pendingRequests["test-request-with-relay"]; !ok {
		t.Error("Expected pending request to be stored")
	} else {
		if pending.relayState != "my-relay-state" {
			t.Errorf("Expected RelayState 'my-relay-state', got '%s'", pending.relayState)
		}
	}
}

func TestHandleServiceProviderRegistration_EmptyACSBinding(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}

	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}

	server.SetupRoutes()

	// Request without acs_binding should use default HTTP-POST
	reqBody := map[string]string{
		"entity_id": "http://example.com/metadata",
		"acs_url":   "http://example.com/acs",
		// Intentionally omit acs_binding
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

	// Verify the SP was saved with default binding
	descriptor, err := server.db.GetServiceProvider("http://example.com/metadata")
	if err != nil {
		t.Skipf("Cannot verify saved provider: %v", err)
	}

	if len(descriptor.SPSSODescriptors) == 0 || len(descriptor.SPSSODescriptors[0].AssertionConsumerServices) == 0 {
		t.Fatal("Expected AssertionConsumerServices to be populated")
	}

	actualBinding := descriptor.SPSSODescriptors[0].AssertionConsumerServices[0].Binding
	if actualBinding != saml.HTTPPostBinding {
		t.Errorf("Expected default binding %s, got %s", saml.HTTPPostBinding, actualBinding)
	}
}

func TestHandleServiceProviderRegistration_PostBinding(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}

	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}

	server.SetupRoutes()

	reqBody := map[string]string{
		"entity_id":   "http://example.com/metadata",
		"acs_url":     "http://example.com/acs",
		"acs_binding": saml.HTTPPostBinding,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.handleServiceProviderRegistration(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestHandleServiceProviderRegistration_RedirectBinding(t *testing.T) {
	server := setupTestServer(t)
	if server.db.db == nil {
		t.Skip("Skipping test: database not available")
	}

	if err := server.db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}

	server.SetupRoutes()

	reqBody := map[string]string{
		"entity_id":   "http://example.com/metadata",
		"acs_url":     "http://example.com/acs",
		"acs_binding": saml.HTTPRedirectBinding,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/admin/service-providers", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.handleServiceProviderRegistration(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestServiceProviderAdapter_GetServiceProvider_NotFound(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	testDB, err := sql.Open("postgres", "postgres://saml_provider:saml_provider@localhost:5432/saml_provider_tests?sslmode=disable")
	if err != nil || testDB.Ping() != nil {
		t.Skip("Skipping test: database not available")
	}
	defer testDB.Close()

	db := NewDatabase(testDB, logger)
	if err := db.InitSchema(); err != nil {
		t.Skipf("Cannot initialize schema: %v", err)
	}

	adapter := &serviceProviderAdapter{db: db}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	descriptor, err := adapter.GetServiceProvider(req, "http://nonexistent.com/metadata")

	if err == nil {
		t.Error("Expected error for non-existent service provider")
	}

	if descriptor != nil {
		t.Error("Expected nil descriptor for non-existent service provider")
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected error to match os.ErrNotExist, got %v", err)
	}
}

func TestParseURL_ValidURL(t *testing.T) {
	server := setupTestServer(t)

	testCases := []struct {
		input  string
		scheme string
		host   string
		path   string
	}{
		{
			input:  "http://example.com/path",
			scheme: "http",
			host:   "example.com",
			path:   "/path",
		},
		{
			input:  "https://example.com:8080/path?query=value",
			scheme: "https",
			host:   "example.com:8080",
			path:   "/path",
		},
		{
			input:  "http://localhost:8082/saml/metadata",
			scheme: "http",
			host:   "localhost:8082",
			path:   "/saml/metadata",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			parsed := server.parseURL(tc.input)

			if parsed.Scheme != tc.scheme {
				t.Errorf("Expected scheme %s, got %s", tc.scheme, parsed.Scheme)
			}
			if parsed.Host != tc.host {
				t.Errorf("Expected host %s, got %s", tc.host, parsed.Host)
			}
			if parsed.Path != tc.path {
				t.Errorf("Expected path %s, got %s", tc.path, parsed.Path)
			}
		})
	}
}

func TestNewHydraHTTPClient_WithCustomCACert(t *testing.T) {
	server := setupTestServer(t)
	server.config.HydraInsecureSkipTLSVerify = false

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "hydra-ca.pem")
	if err := os.WriteFile(certPath, generateTestCertificatePEM(t), 0o600); err != nil {
		t.Fatalf("Failed to write test certificate: %v", err)
	}

	server.config.HydraCACertPath = certPath

	client, err := server.newHydraHTTPClient()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected *http.Transport, got %T", client.Transport)
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLSClientConfig to be set")
	}

	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("Expected RootCAs to be set when HydraCACertPath is configured")
	}

	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("Expected InsecureSkipVerify to be false")
	}
}

func TestNewHydraHTTPClient_MissingCACertPath(t *testing.T) {
	server := setupTestServer(t)
	server.config.HydraCACertPath = filepath.Join(t.TempDir(), "does-not-exist.pem")

	_, err := server.newHydraHTTPClient()
	if err == nil {
		t.Fatal("Expected error for missing Hydra CA certificate path")
	}

	if !strings.Contains(err.Error(), "failed to read Hydra CA certificate") {
		t.Fatalf("Expected read error message, got %v", err)
	}
}

func TestNewHydraHTTPClient_InvalidCACertPEM(t *testing.T) {
	server := setupTestServer(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "invalid-ca.pem")
	if err := os.WriteFile(certPath, []byte("not-a-valid-pem"), 0o600); err != nil {
		t.Fatalf("Failed to write invalid PEM file: %v", err)
	}

	server.config.HydraCACertPath = certPath

	_, err := server.newHydraHTTPClient()
	if err == nil {
		t.Fatal("Expected error for invalid Hydra CA certificate PEM")
	}

	if !strings.Contains(err.Error(), "failed to parse Hydra CA certificate PEM") {
		t.Fatalf("Expected parse error message, got %v", err)
	}
}

func TestNewHydraHTTPClient_InsecureSkipVerify(t *testing.T) {
	server := setupTestServer(t)
	server.config.HydraInsecureSkipTLSVerify = true

	client, err := server.newHydraHTTPClient()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected *http.Transport, got %T", client.Transport)
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLSClientConfig to be set")
	}

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("Expected InsecureSkipVerify to be true")
	}
}

func generateTestCertificatePEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-hydra-ca",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
}

// -----------------------------------------------
// Observability Tests (Metrics & Tracing)
// -----------------------------------------------

func TestMetricsEndpointExists(t *testing.T) {
	server := setupTestServer(t)
	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}

	server.SetupRoutes()

	// Verify /metrics route is registered
	routes := []string{}
	err := chi.Walk(server.router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk routes: %v", err)
	}

	found := false
	for _, route := range routes {
		if strings.Contains(route, "GET /metrics") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected GET /metrics route, got routes: %v", routes)
	}
}

func TestMetricsEndpointReturnsPrometheusData(t *testing.T) {
	server := setupTestServer(t)
	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}

	server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	bodyStr := string(body)

	// Prometheus metrics endpoint should return content-type text/plain
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected text/plain content-type, got %s", contentType)
	}

	// Body should be non-empty and contain Prometheus text format markers
	if len(bodyStr) == 0 {
		t.Fatalf("Expected non-empty metrics response body")
	}

	if !strings.Contains(bodyStr, "# HELP") && !strings.Contains(bodyStr, "# TYPE") {
		snippet := bodyStr
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("Metrics response does not contain Prometheus HELP/TYPE markers. Snippet: %s", snippet)
	}
}

func TestResponseTimeMiddlewareRecordsMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	// Create a mock monitor that tracks calls
	mockMonitor := &testMockMonitor{
		metrics: make(map[string]map[string]float64),
	}

	server := &Server{
		config: Config{
			BridgeBaseURL:  "http://localhost:8082",
			BridgeBasePort: "8082",
		},
		logger:          logger,
		monitor:         mockMonitor,
		tracer:          tracing.NewNoopTracer(),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
	}

	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}

	server.SetupRoutes()

	// Make a request to /metrics (simple endpoint that doesn't require DB)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if len(mockMonitor.metrics) == 0 {
		t.Fatal("Expected response-time metric to be recorded")
	}

	if len(mockMonitor.responseTimeCalls) == 0 {
		t.Fatal("Expected SetResponseTimeMetric to be called at least once")
	}

	call := mockMonitor.responseTimeCalls[0]
	if call.Tags["route"] != "GET/metrics" {
		t.Fatalf("Expected route tag GET/metrics, got %q", call.Tags["route"])
	}

	if call.Tags["status"] != "200" {
		t.Fatalf("Expected status tag 200, got %q", call.Tags["status"])
	}

	if call.Value < 0 {
		t.Fatalf("Expected duration to be >= 0, got %f", call.Value)
	}
}

func TestResponseTimeMiddlewareIncludesRouteAndStatus(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	mockMonitor := &testMockMonitor{
		metrics: make(map[string]map[string]float64),
	}

	server := &Server{
		config: Config{
			BridgeBaseURL:  "http://localhost:8082",
			BridgeBasePort: "8082",
		},
		logger:          logger,
		monitor:         mockMonitor,
		tracer:          tracing.NewNoopTracer(),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
	}

	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}

	server.SetupRoutes()

	// Make a request to /metrics which should succeed
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	// Status should be 200
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if len(mockMonitor.responseTimeCalls) == 0 {
		t.Fatal("Expected SetResponseTimeMetric to be called at least once")
	}

	found := false
	for _, call := range mockMonitor.responseTimeCalls {
		if call.Tags["route"] == "GET/metrics" && call.Tags["status"] == "200" {
			if call.Value < 0 {
				t.Fatalf("Expected duration to be >= 0, got %f", call.Value)
			}
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected a response-time metric call with route=GET/metrics and status=200, got calls: %+v", mockMonitor.responseTimeCalls)
	}
}

func TestDependencyAvailabilityOnHydraInitialize(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	hydraStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			http.Error(w, "stubbed hydra discovery failure", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer hydraStub.Close()

	mockMonitor := &testMockMonitor{
		metrics: make(map[string]map[string]float64),
	}

	server := &Server{
		config: Config{
			BridgeBaseURL:  "http://localhost:8082",
			ClientID:       "test-client",
			ClientSecret:   "test-secret",
			HydraPublicURL: hydraStub.URL,
		},
		logger:          logger,
		monitor:         mockMonitor,
		tracer:          tracing.NewNoopTracer(),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
		hydraHTTPClient: hydraStub.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call Initialize which should attempt to connect to Hydra
	err := server.Initialize(ctx, zaptest.NewLogger(t))

	// Should get an error since Hydra is unavailable
	if err == nil {
		t.Fatal("Expected error during Initialize due to Hydra unavailability")
	}

	if len(mockMonitor.dependencyCalls) == 0 {
		t.Fatal("Expected SetDependencyAvailability to be called")
	}

	foundHydraUnavailable := false
	for _, call := range mockMonitor.dependencyCalls {
		if call.Tags["component"] == "hydra" && call.Value == 0 {
			foundHydraUnavailable = true
			break
		}
	}

	if !foundHydraUnavailable {
		t.Fatalf("Expected a dependency availability call with component=hydra and value=0, got: %+v", mockMonitor.dependencyCalls)
	}
}

func TestDependencyAvailabilityOnHydraTokenExchange(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	hydraStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			http.Error(w, "stubbed token exchange failure", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer hydraStub.Close()

	mockMonitor := &testMockMonitor{
		metrics: make(map[string]map[string]float64),
	}

	server := &Server{
		config: Config{
			BridgeBaseURL: "http://localhost:8082",
			ClientID:      "test-client",
			ClientSecret:  "test-secret",
		},
		logger:          logger,
		monitor:         mockMonitor,
		tracer:          tracing.NewNoopTracer(),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
		oauth2Config: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost:8082/saml/callback",
			Scopes:       []string{"openid"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  hydraStub.URL + "/oauth2/auth",
				TokenURL: hydraStub.URL + "/oauth2/token",
			},
		},
		hydraHTTPClient: hydraStub.Client(),
	}

	server.SetupRoutes()

	// Make request to OIDC callback with invalid code
	req := httptest.NewRequest(http.MethodGet, "/saml/callback?code=invalid&state=test", nil)
	rec := httptest.NewRecorder()

	server.handleOIDCCallback(rec, req)

	// Should fail due to invalid Hydra connection, which should trigger dependency metric
	if rec.Code < 400 {
		t.Logf("Unexpected success status: %d", rec.Code)
	}

	if len(mockMonitor.dependencyCalls) == 0 {
		t.Fatal("Expected SetDependencyAvailability to be called")
	}

	foundHydraUnavailable := false
	for _, call := range mockMonitor.dependencyCalls {
		if call.Tags["component"] == "hydra" && call.Value == 0 {
			foundHydraUnavailable = true
			break
		}
	}

	if !foundHydraUnavailable {
		t.Fatalf("Expected a dependency availability call with component=hydra and value=0, got: %+v", mockMonitor.dependencyCalls)
	}
}

func TestMetricsEndpointWithCustomServiceName(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	// Create server with custom monitor that has a service name
	mockMonitor := &testMockMonitor{
		metrics:     make(map[string]map[string]float64),
		serviceName: "test-saml-provider",
	}

	server := &Server{
		config: Config{
			BridgeBaseURL:  "http://localhost:8082",
			BridgeBasePort: "8082",
		},
		logger:          logger,
		monitor:         mockMonitor,
		tracer:          tracing.NewNoopTracer(),
		pendingRequests: make(map[string]pendingAuthnRequest),
		router:          chi.NewRouter(),
	}

	server.samlIdp = &saml.IdentityProvider{
		MetadataURL: url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/metadata"},
		SSOURL:      url.URL{Scheme: "http", Host: "localhost:8082", Path: "/saml/sso"},
	}

	server.SetupRoutes()

	if mockMonitor.GetService() != "test-saml-provider" {
		t.Errorf("Expected service name 'test-saml-provider', got %s", mockMonitor.GetService())
	}
}

// -----------------------------------------------
// Test Helper Structs for Mocking Monitoring
// -----------------------------------------------

type testMockMonitor struct {
	metrics           map[string]map[string]float64
	serviceName       string
	responseTimeCalls []responseTimeMetricCall
	dependencyCalls   []dependencyAvailabilityCall
}

type responseTimeMetricCall struct {
	Tags  map[string]string
	Value float64
}

type dependencyAvailabilityCall struct {
	Tags  map[string]string
	Value float64
}

func (m *testMockMonitor) GetService() string {
	if m.serviceName != "" {
		return m.serviceName
	}
	return "test-service"
}

func (m *testMockMonitor) SetResponseTimeMetric(tags map[string]string, value float64) error {
	if m.metrics == nil {
		m.metrics = make(map[string]map[string]float64)
	}

	tagsCopy := map[string]string{}
	for k, v := range tags {
		tagsCopy[k] = v
	}
	m.responseTimeCalls = append(m.responseTimeCalls, responseTimeMetricCall{Tags: tagsCopy, Value: value})

	key := fmt.Sprintf("%s_%s", tags["route"], tags["status"])
	if m.metrics[key] == nil {
		m.metrics[key] = make(map[string]float64)
	}

	m.metrics[key]["duration"] = value
	return nil
}

func (m *testMockMonitor) SetDependencyAvailability(tags map[string]string, value float64) error {
	if m.metrics == nil {
		m.metrics = make(map[string]map[string]float64)
	}

	tagsCopy := map[string]string{}
	for k, v := range tags {
		tagsCopy[k] = v
	}
	m.dependencyCalls = append(m.dependencyCalls, dependencyAvailabilityCall{Tags: tagsCopy, Value: value})

	key := fmt.Sprintf("dep_%s", tags["component"])
	if m.metrics[key] == nil {
		m.metrics[key] = make(map[string]float64)
	}

	m.metrics[key]["availability"] = value
	return nil
}
