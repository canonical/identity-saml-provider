package provider

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/crewjam/saml"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Server represents the SAML-OIDC bridge server
type Server struct {
	config          Config
	logger          *zap.SugaredLogger
	oauth2Config    *oauth2.Config
	oidcVerifier    *oidc.IDTokenVerifier
	samlIdp         *saml.IdentityProvider
	db              *Database
	pendingRequests map[string]pendingAuthnRequest
}

type pendingAuthnRequest struct {
	samlRequest string
	relayState  string
}

// NewServer creates a new SAML-OIDC bridge server
func NewServer(cfg Config, logger *zap.SugaredLogger, sqlDB *sql.DB) (*Server, error) {
	s := &Server{
		config:          cfg,
		logger:          logger,
		db:              NewDatabase(sqlDB, logger),
		pendingRequests: make(map[string]pendingAuthnRequest),
	}
	return s, nil
}

// Initialize sets up the OIDC and SAML providers
func (s *Server) Initialize(ctx context.Context, zapLogger *zap.Logger) error {
	// Initialize OIDC Provider (Hydra)
	s.logger.Infow("Connecting to Ory Hydra", "url", s.config.HydraPublicURL)
	// InsecureIssuerURLContext is used here for local testing where the URL
	// used by the provider does not match the public facing URL.
	ctx = oidc.InsecureIssuerURLContext(ctx, s.config.HydraPublicURL)
	provider, err := oidc.NewProvider(ctx, s.config.HydraPublicURL)
	if err != nil {
		return fmt.Errorf("failed to query Hydra provider: %w", err)
	}

	s.oidcVerifier = provider.Verifier(&oidc.Config{ClientID: s.config.ClientID})

	s.oauth2Config = &oauth2.Config{
		ClientID:     s.config.ClientID,
		ClientSecret: s.config.ClientSecret,
		RedirectURL:  s.config.BridgeBaseURL + "/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	// Initialize SAML Identity Provider
	s.logger.Info("Loading SAML keys")
	certPath := s.config.SAMLCertPath
	keyPath := s.config.SAMLKeyPath
	if certPath == "" {
		certPath = ".local/certs/bridge.crt"
	}
	if keyPath == "" {
		keyPath = ".local/certs/bridge.key"
	}
	keyPair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load key pair: %w", err)
	}

	x509Cert, _ := x509.ParseCertificate(keyPair.Certificate[0])

	// Create the IdP instance
	s.samlIdp = &saml.IdentityProvider{
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: x509Cert,
		Logger:      NewZapStdLogger(zapLogger),
		SSOURL:      s.parseURL(s.config.BridgeBaseURL + "/saml/sso"),
		MetadataURL: s.parseURL(s.config.BridgeBaseURL + "/saml/metadata"),
		// This provider handles looking up the SP (Service) details
		ServiceProviderProvider: &serviceProviderAdapter{db: s.db},
		// Session provider handles authentication state
		SessionProvider: &sessionProviderAdapter{server: s},
	}

	return nil
}

// SetupRoutes configures the HTTP routes for the server
func (s *Server) SetupRoutes() {
	// A. Metadata Endpoint (Service providers need this to configure the connection)
	http.HandleFunc("/saml/metadata", s.samlIdp.ServeMetadata)

	// B. SSO Entry Point (Service providers redirect users here)
	http.HandleFunc("/saml/sso", s.samlIdp.ServeSSO)

	// C. OIDC Callback (Hydra redirects users back here)
	http.HandleFunc("/callback", s.handleOIDCCallback)

	// D. Service Provider Registration Endpoint
	http.HandleFunc("/admin/service-providers", s.handleServiceProviderRegistration)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Infow("SAML-OIDC Bridge listening", "url", s.config.BridgeBaseURL)
	return http.ListenAndServe(":"+s.config.BridgeBasePort, nil)
}

// -------------------------------------------------------------------------
// Session Provider Adapter
// -------------------------------------------------------------------------
type sessionProviderAdapter struct {
	server *Server
}

func (sp *sessionProviderAdapter) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	sp.server.logger.Info("Checking for existing SAML session")
	// Check if we have a session cookie from the OIDC callback
	sessionCookie, err := r.Cookie("saml_session")

	// Retrieve the session if cookie exists
	var session *saml.Session
	if err == nil && sessionCookie.Value != "" {
		sp.server.logger.Infow("Found session cookie", "sessionID", sessionCookie.Value)
		session = sp.server.db.GetSession(sessionCookie.Value)
	} else {
		sp.server.logger.Infow("No session cookie found", "error", err)
	}

	// If no valid session, redirect to Hydra for authentication
	if session == nil {
		// Capture the original SAMLRequest so we can replay it after OIDC login
		samlRequest := r.URL.Query().Get("SAMLRequest")
		if samlRequest == "" {
			// Check POST form if not in query string
			if err := r.ParseForm(); err == nil {
				samlRequest = r.PostForm.Get("SAMLRequest")
			}
		}
		if samlRequest != "" {
			sp.server.pendingRequests[req.Request.ID] = pendingAuthnRequest{
				samlRequest: samlRequest,
				relayState:  req.RelayState,
			}
		}

		// Build state with request ID and optional relay state
		state := req.Request.ID
		if req.RelayState != "" {
			state += ":" + req.RelayState
		}

		sp.server.logger.Info("No valid session found, redirecting to Hydra for authentication")
		http.Redirect(w, r, sp.server.oauth2Config.AuthCodeURL(state), http.StatusFound)
		return nil
	}

	return session
}

// -------------------------------------------------------------------------
// OIDC Callback Handler
// -------------------------------------------------------------------------
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Handling OIDC callback from Hydra")
	ctx := context.Background()

	// 1. Exchange the Authorization Code for tokens
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in callback", http.StatusBadRequest)
		return
	}

	token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Extract and Verify the ID Token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token field in oauth2 token", http.StatusInternalServerError)
		return
	}
	idToken, err := s.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Extract User Claims (Email is critical for service)
	var claims struct {
		Email string `json:"email"`
		Sub   string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	if claims.Email == "" {
		http.Error(w, "User has no email in ID Token. Cannot authenticate with Service.", http.StatusForbidden)
		return
	}

	s.logger.Debugw("User authenticated, creating SAML session", "email", claims.Email)

	// 4. Create a SAML Session
	sessionID := fmt.Sprintf("_%d", time.Now().UnixNano())
	samlSession := &saml.Session{
		ID:             sessionID,
		CreateTime:     time.Now(),
		ExpireTime:     time.Now().Add(10 * time.Minute),
		Index:          sessionID,
		NameID:         claims.Email, // Service matches users by NameID (Email)
		UserEmail:      claims.Email,
		UserCommonName: claims.Email, // Use email as display name
		Groups:         []string{},
	}
	// Store the session in database
	if err := s.db.SaveSession(samlSession); err != nil {
		s.logger.Errorw("Failed to save session to database", "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "saml_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// 5. Parse the state to get SAML request ID and RelayState
	state := r.URL.Query().Get("state")
	requestID := ""
	relayState := ""
	if state != "" {
		parts := strings.SplitN(state, ":", 2)
		requestID = parts[0]
		if len(parts) > 1 {
			relayState = parts[1]
		}
	}

	if requestID != "" {
		s.logger.Infow("OIDC callback for SAML request", "requestID", requestID)
	}

	redirectURL := fmt.Sprintf("%s/saml/sso", s.config.BridgeBaseURL)

	// Retrieve and replay the original SAMLRequest if available
	if requestID != "" {
		if pending, ok := s.pendingRequests[requestID]; ok {
			delete(s.pendingRequests, requestID)
			query := url.Values{}
			query.Set("SAMLRequest", pending.samlRequest)
			if pending.relayState != "" {
				query.Set("RelayState", pending.relayState)
			}
			redirectURL += "?" + query.Encode()
		} else if relayState != "" {
			redirectURL += "?RelayState=" + url.QueryEscape(relayState)
		}
	} else if relayState != "" {
		redirectURL += "?RelayState=" + url.QueryEscape(relayState)
	}

	// 6. Redirect back to the SAML SSO handler to continue the flow
	s.logger.Info("Session created, redirecting back to SAML SSO handler")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// -------------------------------------------------------------------------
// Service Provider Adapter
// -------------------------------------------------------------------------
type serviceProviderAdapter struct {
	db *Database
}

func (sp *serviceProviderAdapter) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	// Look up the service provider in the database
	descriptor, err := sp.db.GetServiceProvider(serviceProviderID)
	if err != nil {
		return nil, os.ErrNotExist
	}

	return descriptor, nil
}

// -------------------------------------------------------------------------
// Service Provider Registration Handler
// -------------------------------------------------------------------------
func (s *Server) handleServiceProviderRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST to register a new service provider.", http.StatusMethodNotAllowed)
		return
	}

	// Parse the JSON request body
	var req struct {
		EntityID   string `json:"entity_id"`
		ACSURL     string `json:"acs_url"`
		ACSBinding string `json:"acs_binding"`
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to parse JSON request", http.StatusBadRequest)
			return
		}
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") || contentType == "" {
		// Support form-encoded requests (default if no Content-Type)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form request", http.StatusBadRequest)
			return
		}
		req.EntityID = r.FormValue("entity_id")
		req.ACSURL = r.FormValue("acs_url")
		req.ACSBinding = r.FormValue("acs_binding")
	} else {
		http.Error(w, "Unsupported Content-Type", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.EntityID == "" || req.ACSURL == "" {
		http.Error(w, "Missing required fields: entity_id and acs_url are required", http.StatusBadRequest)
		return
	}

	// Validate that ACSURL is a valid URL
	acsURL, err := url.Parse(req.ACSURL)
	if err != nil || acsURL.Scheme == "" || acsURL.Host == "" {
		http.Error(w, "Invalid acs_url: must be a valid URL with scheme and host", http.StatusBadRequest)
		return
	}
	if acsURL.Scheme != "http" && acsURL.Scheme != "https" {
		http.Error(w, "Invalid acs_url: scheme must be http or https", http.StatusBadRequest)
		return
	}

	// Validate that EntityID is a valid URL
	entityURL, err := url.Parse(req.EntityID)
	if err != nil || entityURL.Scheme == "" || entityURL.Host == "" {
		http.Error(w, "Invalid entity_id: must be a valid URL with scheme and host", http.StatusBadRequest)
		return
	}
	if entityURL.Scheme != "http" && entityURL.Scheme != "https" {
		http.Error(w, "Invalid entity_id: scheme must be http or https", http.StatusBadRequest)
		return
	}

	validBindings := map[string]bool{
		saml.HTTPPostBinding:     true,
		saml.HTTPRedirectBinding: true,
	}
	if req.ACSBinding == "" {
		// Apply default binding when not provided
		req.ACSBinding = saml.HTTPPostBinding
	} else if !validBindings[req.ACSBinding] {
		http.Error(w, "Invalid acs_binding value", http.StatusBadRequest)
		return
	}

	// Save to database
	if err := s.db.SaveServiceProvider(req.EntityID, req.ACSURL, req.ACSBinding); err != nil {
		s.logger.Errorw("Failed to save service provider", "error", err)
		http.Error(w, "Failed to save service provider", http.StatusInternalServerError)
		return
	}

	s.logger.Infow("Service provider registered successfully", "entityID", req.EntityID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]string{
		"status":    "success",
		"message":   "Service provider registered",
		"entity_id": req.EntityID,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Errorw("Failed to encode JSON response", "error", err)
	}
}

func (s *Server) parseURL(u string) url.URL {
	parsed, _ := url.Parse(u)
	return *parsed
}
