package handler

import (
	"net/http"
	"os"
	"time"

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
	a.Logger.Infow("Checking for existing SAML session")

	// 1. Check session cookie
	sessionCookie, err := r.Cookie("saml_session")

	var domainSession *domain.Session
	if err == nil && sessionCookie.Value != "" {
		a.Logger.Infow("Found session cookie", "sessionID", sessionCookie.Value)
		domainSession, err = a.Sessions.GetByID(ctx, sessionCookie.Value)
		if err != nil {
			a.Logger.Infow("Session not found or expired", "sessionID", sessionCookie.Value, "error", err)
			domainSession = nil
		}
	} else {
		a.Logger.Infow("No session cookie found")
	}

	// 2. If no valid session, redirect to OIDC for authentication
	if domainSession == nil {
		samlRequest := r.FormValue("SAMLRequest")

		if samlRequest != "" {
			pendingReq := &domain.PendingAuthnRequest{
				RequestID:   req.Request.ID,
				SAMLRequest: samlRequest,
				RelayState:  req.RelayState,
				CreatedAt:   time.Now(),
			}
			if storeErr := a.Pending.Store(ctx, pendingReq); storeErr != nil {
				a.Logger.Errorw("Failed to store pending request", "error", storeErr)
			}
		}

		// Build state with request ID and optional relay state
		state := req.Request.ID
		if req.RelayState != "" {
			state += ":" + req.RelayState
		}

		a.Logger.Infow("No valid session found, redirecting to OIDC provider")
		http.Redirect(w, r, a.OIDC.AuthCodeURL(state), http.StatusFound)
		return nil
	}

	// 3. Apply per-SP attribute mapping if configured
	if req.Request.Issuer != nil && req.Request.Issuer.Value != "" {
		domainSession, _ = a.Mapping.ApplyMapping(ctx, domainSession, req.Request.Issuer.Value)
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
	sp, err := a.SPs.GetByEntityID(r.Context(), serviceProviderID)
	if err != nil {
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
