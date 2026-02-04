package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	// Store authenticated sessions temporarily (in production, use Redis or a database)
	sessions = make(map[string]*saml.Session)
	// Store pending SAML requests until OIDC login completes
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
	// 1. Initialize OIDC Provider (Hydra)
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
	// 2. Initialize SAML Identity Provider
	// -------------------------------------------------------------------------
	log.Println("Loading SAML Keys...")
	keyPair, err := tls.LoadX509KeyPair("bridge.crt", "bridge.key")
	if err != nil {
		log.Fatalf("Failed to load key pair: %v. Did you run the openssl command?", err)
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
	// 3. Define HTTP Routes
	// -------------------------------------------------------------------------

	// A. Metadata Endpoint (Greenhouse will need this to configure the connection)
	http.HandleFunc("/saml/metadata", samlIdp.ServeMetadata)

	// B. SSO Entry Point (Greenhouse redirects users here)
	http.HandleFunc("/saml/sso", samlIdp.ServeSSO)

	// C. OIDC Callback (Hydra redirects users back here)
	http.HandleFunc("/callback", handleOIDCCallback)

	log.Printf("SAML-OIDC Bridge listening on %s", config.BridgeBaseURL)
	log.Fatal(http.ListenAndServe(":"+config.BridgeBasePort, nil))
}

// -------------------------------------------------------------------------
// STEP 1: Session Provider - Handles authentication state
// -------------------------------------------------------------------------
type BridgeSessionProvider struct{}

func (sp *BridgeSessionProvider) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	// Check if we have a session cookie from the OIDC callback
	sessionCookie, err := r.Cookie("saml_session")

	// Retrieve the session if cookie exists
	var session *saml.Session
	if err == nil && sessionCookie.Value != "" {
		session, _ = sessions[sessionCookie.Value]
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
	// Store the session
	sessions[sessionID] = samlSession

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
// This struct mocks a database lookup for the Service Provider
type ServiceProvider struct{}

func (s *ServiceProvider) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	// In a real app, you might look up 'serviceProviderID' in a database.
	// Here we return the hardcoded configuration for the Service.

	return &saml.EntityDescriptor{
		EntityID: config.ServiceEntityID,
		SPSSODescriptors: []saml.SPSSODescriptor{
			{
				AssertionConsumerServices: []saml.IndexedEndpoint{
					{
						Binding:  saml.HTTPPostBinding,
						Location: config.ServiceACS,
						Index:    1,
					},
				},
			},
		},
	}, nil
}

func parseURL(u string) url.URL {
	parsed, _ := url.Parse(u)
	return *parsed
}
