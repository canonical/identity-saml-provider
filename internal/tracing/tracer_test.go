// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package tracing

import (
	"context"
	"runtime/debug"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/logging"
)

func TestNewTracer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cfg           *Config
		wantNilTracer bool
		wantRecording bool
	}{
		{
			name:          "disabled returns noop",
			cfg:           NewNoopConfig(),
			wantNilTracer: false,
			wantRecording: false,
		},
		{
			name: "enabled with stdout fallback initialises tracer",
			cfg: &Config{
				Enabled: true,
				Logger:  logging.NewNopLogger(),
			},
			wantNilTracer: false,
		},
		{
			name: "enabled with HTTP endpoint initialises tracer",
			cfg: &Config{
				Enabled:          true,
				OtelHTTPEndpoint: "localhost:4318",
				Logger:           logging.NewNopLogger(),
			},
			wantNilTracer: false,
		},
		{
			name: "enabled with always_on sampler produces recording spans",
			cfg: &Config{
				Enabled:     true,
				OtelSampler: "always_on",
				Logger:      logging.NewNopLogger(),
			},
			wantNilTracer: false,
			wantRecording: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := NewTracer(context.Background(), tt.cfg)
			defer tracer.Shutdown() //nolint:errcheck

			if (tracer.tracer == nil) != tt.wantNilTracer {
				t.Errorf("tracer.tracer nil = %v, want nil = %v", tracer.tracer == nil, tt.wantNilTracer)
			}

			ctx, span := tracer.Start(context.Background(), "test.span")
			defer span.End()

			if ctx == nil {
				t.Error("expected non-nil context from Start")
			}

			if tt.wantRecording && !span.IsRecording() {
				t.Error("expected recording span")
			}
		})
	}
}

func TestNewNoopTracer(t *testing.T) {
	tracer := NewNoopTracer()
	defer tracer.Shutdown() //nolint:errcheck

	_, span := tracer.Start(context.Background(), "noop.span")
	defer span.End()

	if span.IsRecording() {
		t.Error("NewNoopTracer should produce noop spans")
	}
}

func TestTracer_Shutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tracer *Tracer
	}{
		{
			name:   "nil shutdown func does not error",
			tracer: &Tracer{},
		},
		{
			name:   "noop tracer shuts down cleanly",
			tracer: NewNoopTracer(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.tracer.Shutdown(); err != nil {
				t.Errorf("Shutdown() returned error: %v", err)
			}
		})
	}
}

func TestNewTracer_ExporterError_ReturnsGracefully(t *testing.T) {
	// Use a cancelled context to trigger exporter init failure for gRPC.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &Config{
		Enabled:          true,
		OtelGRPCEndpoint: "localhost:4317",
		Logger:           logging.NewNopLogger(),
	}

	// Should not panic — returns a tracer even on exporter error.
	tracer := NewTracer(ctx, cfg)
	defer tracer.Shutdown() //nolint:errcheck

	// Verify Shutdown doesn't panic.
	_ = tracer.Shutdown()
}

func TestBuildSampler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sampler string
		ratio   float64
	}{
		{"always_on", "always_on", 0.5},
		{"alwayson", "alwayson", 0.5},
		{"always_off", "always_off", 0.5},
		{"alwaysoff", "alwaysoff", 0.5},
		{"traceidratio", "traceidratio", 0.5},
		{"traceid_ratio", "traceid_ratio", 0.3},
		{"parentbased_traceidratio", "parentbased_traceidratio", 0.1},
		{"parentbasedtraceidratio", "parentbasedtraceidratio", 0.1},
		{"parentbased", "parentbased", 0.1},
		{"empty string defaults to parentbased", "", 0.1},
		{"unknown falls back to parentbased", "unknown_strategy", 0.1},
		{"invalid negative ratio clamps to default", "parentbased", -0.5},
		{"invalid ratio above 1 clamps to default", "parentbased", 1.5},
	}

	logger := logging.NewNopLogger()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tr := &Tracer{logger: logger}
			cfg := &Config{
				OtelSampler:      tt.sampler,
				OtelSamplerRatio: tt.ratio,
			}
			sampler := tr.buildSampler(cfg)
			if sampler == nil {
				t.Fatal("expected non-nil sampler")
			}
		})
	}
}

func TestBuildSampler_Descriptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sampler string
	}{
		{"always_on", "always_on"},
		{"always_off", "always_off"},
		{"traceidratio", "traceidratio"},
		{"parentbased default", ""},
	}

	logger := logging.NewNopLogger()
	tr := &Tracer{logger: logger}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := tr.buildSampler(&Config{
				OtelSampler:      tt.sampler,
				OtelSamplerRatio: 0.5,
			})
			if desc := s.Description(); desc == "" {
				t.Error("sampler description should not be empty")
			}
		})
	}
}

func TestGitRevision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "found revision",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123def"},
				{Key: "vcs.time", Value: "2026-01-01T00:00:00Z"},
			},
			want: "abc123def",
		},
		{
			name: "no revision key",
			settings: []debug.BuildSetting{
				{Key: "vcs.time", Value: "2026-01-01T00:00:00Z"},
			},
			want: "n/a",
		},
		{
			name:     "empty settings",
			settings: nil,
			want:     "n/a",
		},
	}

	tr := &Tracer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tr.gitRevision(tt.settings)
			if got != tt.want {
				t.Errorf("gitRevision() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		serviceName string
		wantAttrKey string
		wantAttrVal string
	}{
		{
			name:        "with explicit service name",
			serviceName: "test-service",
			wantAttrKey: "service.name",
			wantAttrVal: "test-service",
		},
		{
			name:        "empty service name uses build info",
			serviceName: "",
			wantAttrKey: "service.name",
			// Value comes from build info; just verify resource is non-nil.
		},
	}

	tr := &Tracer{logger: logging.NewNopLogger()}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := tr.buildResource(tt.serviceName)
			if res == nil {
				t.Fatal("expected non-nil resource")
			}

			if tt.wantAttrVal != "" {
				found := false
				for _, attr := range res.Attributes() {
					if string(attr.Key) == tt.wantAttrKey && attr.Value.AsString() == tt.wantAttrVal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("resource should contain %s = %s", tt.wantAttrKey, tt.wantAttrVal)
				}
			}
		})
	}
}

func TestTracingInterface_Compliance(t *testing.T) {
	// Verify that *Tracer implements TracingInterface at compile time.
	var _ TracingInterface = (*Tracer)(nil)

	// Verify Start and Shutdown work via the interface.
	var iface TracingInterface = NewNoopTracer()
	ctx, span := iface.Start(context.Background(), "interface.test")
	span.End()
	_ = ctx

	if err := iface.Shutdown(); err != nil {
		t.Errorf("Shutdown() via interface returned error: %v", err)
	}
}
