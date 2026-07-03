// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/canonical/identity-saml-provider/internal/crypto"
	"github.com/canonical/identity-saml-provider/internal/logging"
)

// HandleOIDCCallback handles GET /saml/callback — the OIDC redirect from Hydra.
func (h *Handlers) HandleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

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
		WriteJSON(w, http.StatusBadRequest, APIError{Status: http.StatusBadRequest, Message: "missing authorization code in callback"})
		return
	}

	// 2. Read oauth_nonce cookie
	nonceCookie, err := r.Cookie("oauth_nonce")
	if err != nil {
		span.SetStatus(codes.Error, "missing oauth_nonce cookie")
		WriteJSON(w, http.StatusForbidden, APIError{Status: http.StatusForbidden, Message: "missing oauth_nonce cookie"})
		return
	}

	// 3. Clear cookie immediately
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    "",
		Path:     "/saml/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.config.DevMode,
		SameSite: http.SameSiteLaxMode,
	})

	// 4. Decode cookie into state and nonce components
	cookieState, cookieNonce, err := crypto.DecodeCookieValue(nonceCookie.Value)
	if err != nil {
		span.SetStatus(codes.Error, "malformed oauth_nonce cookie")
		WriteJSON(w, http.StatusForbidden, APIError{Status: http.StatusForbidden, Message: "malformed oauth_nonce cookie"})
		return
	}

	// 5. Parse state: "<stateValue>:<requestID>:<relayState>"
	stateValue, requestID, relayState, err := parseState(state)
	if err != nil {
		span.SetStatus(codes.Error, "malformed state parameter")
		WriteJSON(w, http.StatusForbidden, APIError{Status: http.StatusForbidden, Message: "malformed state parameter"})
		return
	}

	// 6. Compare cookie state vs state parameter (constant-time)
	if !crypto.NonceEqual(cookieState, stateValue) {
		span.SetStatus(codes.Error, "state mismatch")
		WriteJSON(w, http.StatusForbidden, APIError{Status: http.StatusForbidden, Message: "state validation failed"})
		return
	}

	// 7. Exchange authorization code for OIDC claims (nonce verified inside)
	claims, err := h.oidc.ExchangeCode(ctx, code, cookieNonce)
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
		Secure:   !h.config.DevMode,
		SameSite: http.SameSiteLaxMode,
	})

	if requestID != "" {
		logger.Debugw("OIDC callback for SAML request", "requestID", requestID)
	}

	// 6. Build redirect URL back to SAML SSO
	bridgeURL, err := url.Parse(h.config.BridgeBaseURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid BridgeBaseURL")
		logger.Errorw("Invalid BridgeBaseURL", "url", h.config.BridgeBaseURL, "error", err)
		WriteJSON(w, http.StatusInternalServerError, APIError{Status: http.StatusInternalServerError, Message: "internal server error"})
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

// parseState splits the OAuth2 state parameter into state value, request ID, and relay state.
// Format: "stateValue:requestID" or "stateValue:requestID:relayState".
// Returns an error if the state is missing or has fewer than two parts.
func parseState(state string) (stateValue, requestID, relayState string, err error) {
	if state == "" {
		return "", "", "", errors.New("empty state")
	}
	parts := strings.SplitN(state, ":", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", errors.New("malformed state")
	}
	stateValue = parts[0]
	requestID = parts[1]
	if len(parts) > 2 {
		relayState = parts[2]
	}
	return stateValue, requestID, relayState, nil
}
