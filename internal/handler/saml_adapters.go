// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beevik/etree"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/canonical/identity-saml-provider/internal/crypto"
	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/crewjam/saml"
)

// samlSession is an intermediate struct used for domain→saml conversion.
// It mirrors relevant domain.Session fields for adapter use.
type samlSession struct {
	ID                    string
	CreateTime            time.Time
	ExpireTime            time.Time
	Index                 string
	NameID                string
	NameIDFormat          string
	UserEmail             string
	UserCommonName        string
	UserName              string
	UserSurname           string
	UserGivenName         string
	UserScopedAffiliation string
	Groups                []string
	CustomAttributes      []domain.Attribute
}

// toSAML converts the intermediate session to a crewjam/saml Session.
func (s *samlSession) toSAML() *saml.Session {
	ss := &saml.Session{
		ID:                    s.ID,
		CreateTime:            s.CreateTime,
		ExpireTime:            s.ExpireTime,
		Index:                 s.Index,
		NameID:                s.NameID,
		NameIDFormat:          s.NameIDFormat,
		UserEmail:             s.UserEmail,
		UserCommonName:        s.UserCommonName,
		UserName:              s.UserName,
		UserSurname:           s.UserSurname,
		UserGivenName:         s.UserGivenName,
		UserScopedAffiliation: s.UserScopedAffiliation,
		Groups:                s.Groups,
	}

	// Convert domain.Attribute → saml.Attribute
	for _, attr := range s.CustomAttributes {
		samlAttr := saml.Attribute{
			FriendlyName: attr.FriendlyName,
			Name:         attr.Name,
			NameFormat:   attr.NameFormat,
		}
		for _, v := range attr.Values {
			samlAttr.Values = append(samlAttr.Values, saml.AttributeValue{
				Type:  v.Type,
				Value: v.Value,
			})
		}
		ss.CustomAttributes = append(ss.CustomAttributes, samlAttr)
	}

	return ss
}

// domainSessionToSAML converts a domain.Session to an intermediate samlSession.
func domainSessionToSAML(s *domain.Session) *samlSession {
	return &samlSession{
		ID:                    s.ID,
		CreateTime:            s.CreateTime,
		ExpireTime:            s.ExpireTime,
		Index:                 s.Index,
		NameID:                s.NameID,
		NameIDFormat:          s.NameIDFormat,
		UserEmail:             s.UserEmail,
		UserCommonName:        s.UserCommonName,
		UserName:              s.UserName,
		UserSurname:           s.UserSurname,
		UserGivenName:         s.UserGivenName,
		UserScopedAffiliation: s.UserScopedAffiliation,
		Groups:                s.Groups,
		CustomAttributes:      s.CustomAttributes,
	}
}

// -------------------------------------------------------------------------
// SAML Session Provider Adapter
// -------------------------------------------------------------------------

// SAMLSessionAdapter implements the crewjam/saml SessionProvider interface.
// It bridges the SAML library's session management with our service layer.
type SAMLSessionAdapter struct {
	Sessions service.SessionService
	Mapping  service.MappingService
	Pending  service.PendingRequestService
	OIDC     service.OIDCService
	Config   HandlerConfig
	Logger   logging.Logger
}

// GetSession implements saml.SessionProvider. It checks for an existing session
// cookie, retrieves the session, applies per-SP attribute mapping, and converts
// to a saml.Session. If no valid session exists, it stores the pending SAML
// request and redirects the user to the OIDC provider.
func (a *SAMLSessionAdapter) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	logger := logging.FromContext(ctx, a.Logger)
	logger.Debugw("Checking for existing SAML session")

	// 1. Check session cookie
	sessionCookie, err := r.Cookie("saml_session")

	var domainSession *domain.Session
	if err == nil && sessionCookie.Value != "" {
		span.AddEvent("session_cookie_found")
		logger.Debugw("Found session cookie", "sessionID", sessionCookie.Value)
		domainSession, err = a.Sessions.GetByID(ctx, sessionCookie.Value)
		if err != nil {
			span.AddEvent("session_lookup_miss")
			logger.Debugw("Session not found or expired", "sessionID", sessionCookie.Value, "error", err)
			domainSession = nil
		} else {
			span.AddEvent("session_lookup_hit")
		}
	} else {
		span.AddEvent("session_cookie_missing")
		logger.Debugw("No session cookie found")
	}

	// 2. If no valid session, redirect to OIDC for authentication
	if domainSession == nil {
		samlRequest := r.FormValue("SAMLRequest")

		if samlRequest != "" {
			clientIP := getClientIP(r)
			userAgent := r.UserAgent()

			now := time.Now()
			pendingReq := &domain.PendingAuthnRequest{
				RequestID:   req.Request.ID,
				SAMLRequest: samlRequest,
				RelayState:  req.RelayState,
				ClientMetadata: map[string]string{
					"client_ip":  clientIP,
					"user_agent": userAgent,
				},
				CreatedAt: now,
				ExpireAt:  now.Add(a.Config.PendingRequestTTL),
			}
			if storeErr := a.Pending.Store(ctx, pendingReq); storeErr != nil {
				span.RecordError(storeErr)
				logger.Errorw("Failed to store pending request", "error", storeErr)
			} else {
				span.AddEvent("pending_request_stored")
			}
		}

		// Generate independent state and nonce values for CSRF
		// protection and ID token replay prevention.
		stateValue, err := crypto.GenerateNonce()
		if err != nil {
			span.RecordError(err)
			logger.Errorw("Failed to generate state value", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return nil
		}
		nonceValue, err := crypto.GenerateNonce()
		if err != nil {
			span.RecordError(err)
			logger.Errorw("Failed to generate nonce value", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return nil
		}

		// Store both values in a single cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_nonce",
			Value:    crypto.EncodeCookieValue(stateValue, nonceValue),
			Path:     "/saml/callback",
			MaxAge:   600,
			HttpOnly: true,
			Secure:   !a.Config.DevMode,
			SameSite: http.SameSiteLaxMode,
		})

		// Build state: "<stateValue>:<requestID>:<relayState>"
		state := stateValue + ":" + req.Request.ID
		if req.RelayState != "" {
			state += ":" + req.RelayState
		}

		span.AddEvent("oidc_redirect_initiated")
		logger.Debugw("No valid session found, redirecting to OIDC provider")
		http.Redirect(w, r, a.OIDC.AuthCodeURL(state, nonceValue), http.StatusFound)
		return nil
	}

	// 3. Apply per-SP attribute mapping if configured
	if req.Request.Issuer != nil && req.Request.Issuer.Value != "" {
		mapped, mapErr := a.Mapping.ApplyMapping(ctx, domainSession, req.Request.Issuer.Value)
		if mapErr != nil {
			// Returning nil aborts the SAML flow per the
			// crewjam/saml SessionProvider contract.
			span.RecordError(mapErr)
			logger.Errorw("Attribute mapping failed; aborting SAML response",
				"entityID", req.Request.Issuer.Value,
				"error", mapErr,
			)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return nil
		}
		domainSession = mapped
		span.AddEvent("attribute_mapping_applied")
	}

	// 4. Convert domain.Session → saml.Session
	return domainSessionToSAML(domainSession).toSAML()
}

// -------------------------------------------------------------------------
// SAML Service Provider Adapter
// -------------------------------------------------------------------------

// SAMLSPAdapter implements the crewjam/saml ServiceProviderProvider interface.
type SAMLSPAdapter struct {
	SPs service.ServiceProviderService
}

// GetServiceProvider implements saml.ServiceProviderProvider. It looks up
// the service provider by entity ID and returns a saml.EntityDescriptor.
// Returns os.ErrNotExist when the SP is not found, as required by the
// crewjam/saml library contract.
func (a *SAMLSPAdapter) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	sp, err := a.SPs.GetByEntityID(ctx, serviceProviderID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, os.ErrNotExist
	}

	return &saml.EntityDescriptor{
		EntityID: sp.EntityID,
		SPSSODescriptors: []saml.SPSSODescriptor{
			{
				AssertionConsumerServices: []saml.IndexedEndpoint{
					{
						Binding:  sp.ACSBinding,
						Location: sp.ACSURL,
						Index:    1,
					},
				},
			},
		},
	}, nil
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// HandleSSO intercepts the incoming SAML authentication request. If the SAMLRequest
// parameter is missing or empty, it returns a 400 Bad Request JSON response,
// avoiding a decompression failure in the underlying SAML library.
func (h *Handlers) HandleSSO(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Query().Get("SAMLRequest") == "" {
		WriteJSON(w, http.StatusBadRequest, APIError{
			Status:  http.StatusBadRequest,
			Message: "missing or expired SAMLRequest parameter",
		})
		return
	}
	h.samlIDP.ServeSSO(w, r)
}

// HandleMetadata serves the SAML IdP metadata XML, enriched with OASIS algorithm support
// extensions (<alg:SigningMethod> and <alg:DigestMethod>) while preserving the standard
// <KeyDescriptor use="signing"> for universal compatibility across all service providers.
func (h *Handlers) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	if h.samlIDP == nil {
		h.logger.Errorw("HandleMetadata: SAML IdP not initialized")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rawXML, err := xml.MarshalIndent(h.samlIDP.Metadata(), "", "  ")
	if err != nil {
		h.logger.Errorw("HandleMetadata: failed to marshal metadata", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	sigMethod := h.config.SAMLSignatureMethod
	if sigMethod == "" && h.samlIDP.SignatureMethod != "" {
		sigMethod = h.samlIDP.SignatureMethod
	}

	digestMethod := h.config.SAMLDigestMethod

	enrichedXML, err := EnrichMetadataXML(rawXML, sigMethod, digestMethod)
	if err != nil {
		h.logger.Warnw("HandleMetadata: failed to enrich metadata XML, serving raw XML", "error", err)
		enrichedXML = rawXML
	}

	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	if _, err := w.Write(enrichedXML); err != nil {
		h.logger.Errorw("HandleMetadata: failed to write response", "error", err)
	}
}

// EnrichMetadataXML injects OASIS SAML 2.0 Metadata Profile for Algorithm Support
// (<alg:SigningMethod> and <alg:DigestMethod>) into the <IDPSSODescriptor> element,
// positioned immediately before the first <KeyDescriptor> element to adhere to the
// SAML 2.0 metadata schema sequence requirements, while preserving all existing
// <KeyDescriptor> elements intact.
func EnrichMetadataXML(rawXML []byte, sigMethodURI, digestMethodURI string) ([]byte, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return nil, fmt.Errorf("read metadata XML: %w", err)
	}

	root := doc.Root()
	if root == nil {
		return nil, errors.New("metadata XML document is empty")
	}

	if sigMethodURI == "" && digestMethodURI == "" {
		return rawXML, nil
	}

	var idpSSO *etree.Element
	if root.Tag == "IDPSSODescriptor" {
		idpSSO = root
	} else {
		idpSSO = root.FindElement("//IDPSSODescriptor")
	}

	if idpSSO == nil {
		return rawXML, nil
	}

	// Locate or create <Extensions> within <IDPSSODescriptor>.
	extensions := idpSSO.SelectElement("Extensions")
	if extensions == nil {
		extensions = etree.NewElement("Extensions")
		if idpSSO.Space != "" {
			extensions.Space = idpSSO.Space
		}
		extensions.CreateAttr("xmlns:alg", "urn:oasis:names:tc:SAML:metadata:algsupport")

		// SAML metadata schema requires <Extensions> to precede <KeyDescriptor>.
		keyDesc := idpSSO.SelectElement("KeyDescriptor")
		if keyDesc != nil {
			idpSSO.InsertChildAt(keyDesc.Index(), extensions)
		} else {
			idpSSO.InsertChildAt(0, extensions)
		}
	} else {
		if extensions.SelectAttr("xmlns:alg") == nil {
			extensions.CreateAttr("xmlns:alg", "urn:oasis:names:tc:SAML:metadata:algsupport")
		}
	}

	// Add <alg:SigningMethod> if provided and not already present.
	if sigMethodURI != "" && extensions.FindElement("alg:SigningMethod") == nil {
		sm := extensions.CreateElement("alg:SigningMethod")
		sm.CreateAttr("Algorithm", sigMethodURI)
	}

	// Add <alg:DigestMethod> if provided and not already present.
	if digestMethodURI != "" && extensions.FindElement("alg:DigestMethod") == nil {
		dm := extensions.CreateElement("alg:DigestMethod")
		dm.CreateAttr("Algorithm", digestMethodURI)
	}

	doc.Indent(2)
	return doc.WriteToBytes()
}
