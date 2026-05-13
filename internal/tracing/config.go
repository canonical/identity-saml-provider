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
	return &Config{
		Enabled:          enabled,
		OtelGRPCEndpoint: otelGRPCEndpoint,
		OtelHTTPEndpoint: otelHTTPEndpoint,
		OtelSampler:      otelSampler,
		OtelSamplerRatio: otelSamplerRatio,
		Logger:           logger,
	}
}
