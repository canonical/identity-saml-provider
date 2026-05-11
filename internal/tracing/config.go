package tracing

import "github.com/canonical/identity-saml-provider/internal/logging"

type Config struct {
	OtelHTTPEndpoint string
	OtelGRPCEndpoint string
	OtelSampler      string
	OtelSamplerRatio float64
	Logger           logging.Logger

	Enabled bool
}

func NewConfig(enabled bool, otelGRPCEndpoint, otelHTTPEndpoint, otelSampler string, otelSamplerRatio float64, logger logging.Logger) *Config {
	c := new(Config)
	c.OtelGRPCEndpoint = otelGRPCEndpoint
	c.OtelHTTPEndpoint = otelHTTPEndpoint
	c.OtelSampler = otelSampler
	c.OtelSamplerRatio = otelSamplerRatio
	c.Logger = logger
	c.Enabled = enabled
	return c
}

func NewNoopConfig() *Config {
	c := new(Config)
	c.Enabled = false
	return c
}
