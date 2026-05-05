package handler

import (
	"github.com/canonical/identity-saml-provider/internal/logging"
	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/canonical/identity-saml-provider/internal/service"
	"github.com/canonical/identity-saml-provider/internal/tracing"
	"github.com/crewjam/saml"
)

// HandlerConfig holds handler-layer configuration.
type HandlerConfig struct {
	BridgeBaseURL string
}

// Handlers groups all HTTP handler functions for the SAML-OIDC bridge.
type Handlers struct {
	sessions         service.SessionService
	serviceProviders service.ServiceProviderService
	mapping          service.MappingService
	oidc             service.OIDCService
	pending          service.PendingRequestService
	samlIDP          *saml.IdentityProvider
	config           HandlerConfig
	logger           logging.Logger
	monitor          monitoring.MonitorInterface
	tracer           tracing.TracingInterface
}

// NewHandlers creates a new Handlers instance with all dependencies injected.
func NewHandlers(
	sessions service.SessionService,
	sps service.ServiceProviderService,
	mapping service.MappingService,
	oidc service.OIDCService,
	pending service.PendingRequestService,
	samlIDP *saml.IdentityProvider,
	cfg HandlerConfig,
	logger logging.Logger,
	monitor monitoring.MonitorInterface,
	tracer tracing.TracingInterface,
) *Handlers {
	return &Handlers{
		sessions:         sessions,
		serviceProviders: sps,
		mapping:          mapping,
		oidc:             oidc,
		pending:          pending,
		samlIDP:          samlIDP,
		config:           cfg,
		logger:           logger,
		monitor:          monitor,
		tracer:           tracer,
	}
}
