package tracing

import "go.uber.org/zap"

type Config struct {
	OtelHTTPEndpoint string
	OtelGRPCEndpoint string
	OtelSampler      string
	OtelSamplerRatio float64
	Logger           *zap.SugaredLogger

	Enabled bool
}

func NewConfig(enabled bool, otelGRPCEndpoint, otelHTTPEndpoint, otelSampler string, otelSamplerRatio float64, logger *zap.SugaredLogger) *Config {
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
