package handler

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/canonical/identity-saml-provider/internal/domain"
	"github.com/canonical/identity-saml-provider/internal/logging"
)

// HandleOIDCCallback handles GET /saml/callback — the OIDC redirect from Hydra.
func (h *Handlers) HandleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.oidc_callback")
	defer span.End()

	logger := logging.FromContext(ctx, h.logger)
	logger.Debugw("Handling OIDC callback from Hydra")

	// 1. Extract authorization code
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	span.SetAttributes(
		attribute.Bool("handler.callback.has_code", code != ""),
		attribute.Bool("handler.callback.has_state", state != ""),
	)

	if code == "" {
		span.SetStatus(codes.Error, "missing authorization code")
		http.Error(w, "No code in callback", http.StatusBadRequest)
		return
	}

	// 2. Exchange authorization code for OIDC claims
	claims, err := h.oidc.ExchangeCode(ctx, code)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "oidc code exchange failed")
		h.monitor.IncrementBridgeOperation("oidc_code_exchange", "error")
		WriteError(w, err)
		return
	}
	h.monitor.IncrementBridgeOperation("oidc_code_exchange", "success")
	span.AddEvent("oidc_code_exchanged")

	// 3. Create SAML session from OIDC claims
	session, err := h.sessions.CreateFromOIDC(ctx, claims)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "session creation failed")
		h.monitor.IncrementBridgeOperation("session_create", "error")
		WriteError(w, err)
		return
	}
	h.monitor.IncrementBridgeOperation("session_create", "success")
	span.AddEvent("session_created",
		trace.WithAttributes(
			attribute.String("session_id", session.ID),
		),
	)

	// 4. Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "saml_session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(time.Until(session.ExpireTime).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// 5. Parse state → request ID + optional RelayState
	requestID, relayState := parseState(state)

	if requestID != "" {
		logger.Debugw("OIDC callback for SAML request", "requestID", requestID)
	}

	// 6. Build redirect URL back to SAML SSO
	bridgeURL, err := url.Parse(h.config.BridgeBaseURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid BridgeBaseURL")
		logger.Errorw("Invalid BridgeBaseURL", "url", h.config.BridgeBaseURL, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	bridgeURL.Path = path.Join(bridgeURL.Path, "saml", "sso")

	query := url.Values{}
	if requestID != "" {
		pending, err := h.pending.Retrieve(ctx, requestID)
		if err == nil && pending != nil {
			h.monitor.IncrementBridgeOperation("pending_request_retrieve", "success")
			query.Set("SAMLRequest", pending.SAMLRequest)
			if pending.RelayState != "" {
				query.Set("RelayState", pending.RelayState)
			}
		} else {
			h.monitor.IncrementBridgeOperation("pending_request_retrieve", "error")
			if relayState != "" {
				query.Set("RelayState", relayState)
			}
		}
	} else if relayState != "" {
		query.Set("RelayState", relayState)
	}

	if len(query) > 0 {
		bridgeURL.RawQuery = query.Encode()
	}
	redirectURL := bridgeURL.String()

	logger.Debugw("Session created, redirecting back to SAML SSO handler")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// parseState splits the OAuth2 state parameter into request ID and relay state.
// Format: "requestID" or "requestID:relayState".
func parseState(state string) (requestID, relayState string) {
	if state == "" {
		return "", ""
	}
	parts := strings.SplitN(state, ":", 2)
	requestID = parts[0]
	if len(parts) > 1 {
		relayState = parts[1]
	}
	return requestID, relayState
}

// domainSessionToSAML converts a domain.Session to a *saml.Session.
// This function is used by the SAML adapters to bridge between domain types
// and the crewjam/saml library types.
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
