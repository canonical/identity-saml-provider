package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/crewjam/saml"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/oauth2"
)

var config Config

var (
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	samlIdp      *saml.IdentityProvider
	db           *sql.DB
	// Store pending SAML requests until OIDC login completes (kept in-memory as they're short-lived)
	pendingRequests = make(map[string]pendingAuthnRequest)
)

type pendingAuthnRequest struct {
	samlRequest string
	relayState  string
}

func main() {
	ctx := context.Background()

	// Load configuration from environment variables
	if err := envconfig.Process("", &config); err != nil {
		log.Fatalf("Failed to process configuration: %v", err)
	}

	// -------------------------------------------------------------------------
	// 1. Initialize Database Connection
	// -------------------------------------------------------------------------
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	log.Printf("Connecting to PostgreSQL at %s:%s...", config.DBHost, config.DBPort)
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Verify the connection
	if err = db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	// Initialize database schema
	if err = initDatabase(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// -------------------------------------------------------------------------
	// 2. Initialize OIDC Provider (Hydra)
	// -------------------------------------------------------------------------
	log.Printf("Connecting to Ory Hydra at %s...", config.HydraPublicURL)
	// InsecureIssuerURLContext is used here for local testing where the URL
	// used by the provider does not match the public facing URL.
	ctx = oidc.InsecureIssuerURLContext(ctx, config.HydraPublicURL)
	provider, err := oidc.NewProvider(ctx, config.HydraPublicURL)
	if err != nil {
		log.Fatalf("Failed to query Hydra provider: %v", err)
	}

	oidcVerifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	oauth2Config = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.BridgeBaseURL + "/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	// -------------------------------------------------------------------------
	// 3. Initialize SAML Identity Provider
	// -------------------------------------------------------------------------
	log.Println("Loading SAML Keys...")
	certPath := config.SAMLCertPath
	keyPath := config.SAMLKeyPath
	if certPath == "" {
		certPath = "etc/certs/bridge.crt"
	}
	if keyPath == "" {
		keyPath = "etc/certs/bridge.key"
	}
	keyPair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("Failed to load key pair from %s, %s: %v. Did you run the openssl command?", certPath, keyPath, err)
	}

	x509Cert, _ := x509.ParseCertificate(keyPair.Certificate[0])

	// Create the IdP instance
	samlIdp = &saml.IdentityProvider{
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: x509Cert,
		Logger:      log.Default(),
		SSOURL:      parseURL(config.BridgeBaseURL + "/saml/sso"),
		MetadataURL: parseURL(config.BridgeBaseURL + "/saml/metadata"),
		// This provider handles looking up the SP (Service) details
		ServiceProviderProvider: &ServiceProvider{},
		// Session provider handles authentication state
		SessionProvider: &BridgeSessionProvider{},
	}

	// -------------------------------------------------------------------------
	// 4. Define HTTP Routes
	// -------------------------------------------------------------------------

	// A. Metadata Endpoint (Greenhouse will need this to configure the connection)
	http.HandleFunc("/saml/metadata", samlIdp.ServeMetadata)

	// B. SSO Entry Point (Greenhouse redirects users here)
	http.HandleFunc("/saml/sso", samlIdp.ServeSSO)

	// C. OIDC Callback (Hydra redirects users back here)
	http.HandleFunc("/callback", handleOIDCCallback)

	// D. Service Provider Registration Endpoint
	http.HandleFunc("/admin/service-providers", handleServiceProviderRegistration)

	log.Printf("SAML-OIDC Bridge listening on %s", config.BridgeBaseURL)
	log.Fatal(http.ListenAndServe(":"+config.BridgeBasePort, nil))
}

// -------------------------------------------------------------------------
// STEP 1: Session Provider - Handles authentication state
// -------------------------------------------------------------------------
type BridgeSessionProvider struct{}

func (sp *BridgeSessionProvider) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	log.Println("Checking for existing SAML session...")
	// Check if we have a session cookie from the OIDC callback
	sessionCookie, err := r.Cookie("saml_session")

	// Retrieve the session if cookie exists
	var session *saml.Session
	if err == nil && sessionCookie.Value != "" {
		log.Printf("Found session cookie with ID: %s", sessionCookie.Value)
		session = getSessionFromDB(sessionCookie.Value)
	} else {
		log.Printf("No session cookie found: %v", err)
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
			pendingRequests[req.Request.ID] = pendingAuthnRequest{
				samlRequest: samlRequest,
				relayState:  req.RelayState,
			}
		}

		// Build state with request ID and optional relay state
		state := req.Request.ID
		if req.RelayState != "" {
			state += ":" + req.RelayState
		}

		log.Printf("No valid session found. Redirecting to Hydra OIDC...")
		http.Redirect(w, r, oauth2Config.AuthCodeURL(state), http.StatusFound)
		return nil
	}

	return session
}

// -------------------------------------------------------------------------
// STEP 2: Handle Return from Hydra
// -------------------------------------------------------------------------
func handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling OIDC callback from Hydra...")
	ctx := context.Background()

	// 1. Exchange the Authorization Code for tokens
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in callback", http.StatusBadRequest)
		return
	}

	token, err := oauth2Config.Exchange(ctx, code)
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
	idToken, err := oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Extract User Claims (Email is critical for Greenhouse)
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

	log.Printf("Authenticated user: %s. Creating SAML session...", claims.Email)

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
	if err := saveSessionToDB(samlSession); err != nil {
		log.Printf("Failed to save session to database: %v", err)
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
		log.Printf("OIDC callback for SAML request: %s", requestID)
	}

	redirectURL := fmt.Sprintf("%s/saml/sso", config.BridgeBaseURL)

	// Retrieve and replay the original SAMLRequest if available
	if requestID != "" {
		if pending, ok := pendingRequests[requestID]; ok {
			delete(pendingRequests, requestID)
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
	log.Printf("Session created. Redirecting back to SAML SSO handler...")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// -------------------------------------------------------------------------
// Helper: Service Provider Definition
// -------------------------------------------------------------------------
type ServiceProvider struct{}

func (s *ServiceProvider) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	log.Printf("Looking up service provider in database: %s", serviceProviderID)

	// Look up the service provider in the database
	descriptor, err := getServiceProviderFromDB(serviceProviderID)
	if err != nil {
		log.Printf("Failed to retrieve service provider %s: %v. Did you forget to register it?", serviceProviderID, err)
		return nil, os.ErrNotExist
	}

	return descriptor, nil
}

// -------------------------------------------------------------------------
// Service Provider Registration Handler
// -------------------------------------------------------------------------
func handleServiceProviderRegistration(w http.ResponseWriter, r *http.Request) {
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
	if err := saveServiceProviderToDB(req.EntityID, req.ACSURL, req.ACSBinding); err != nil {
		log.Printf("Failed to save service provider: %v", err)
		http.Error(w, "Failed to save service provider", http.StatusInternalServerError)
		return
	}

	log.Printf("Service provider registered successfully: %s", req.EntityID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]string{
		"status":    "success",
		"message":   "Service provider registered",
		"entity_id": req.EntityID,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func parseURL(u string) url.URL {
	parsed, _ := url.Parse(u)
	return *parsed
}
