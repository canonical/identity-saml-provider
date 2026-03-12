package tracing

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

const serviceName = "identity-saml-provider"

type Tracer struct {
	tracer   trace.Tracer
	logger   *zap.SugaredLogger
	shutdown func(context.Context) error
}

var _ TracingInterface = (*Tracer)(nil)

func (t *Tracer) init(service string, sampler sdktrace.Sampler, e sdktrace.SpanExporter) {
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(e),
		sdktrace.WithResource(t.buildResource(service)),
	)

	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}, jaeger.Jaeger{}))

	t.tracer = otel.Tracer(service)
	t.shutdown = traceProvider.Shutdown
}

func (t *Tracer) buildSampler(cfg *Config) sdktrace.Sampler {
	strategy := strings.ToLower(strings.TrimSpace(cfg.OtelSampler))

	ratio := cfg.OtelSamplerRatio
	if ratio < 0 || ratio > 1 {
		t.logger.Warnw("invalid sampler ratio, using default", "ratio", ratio, "default", 0.1)
		ratio = 0.1
	}

	switch strategy {
	case "always_on", "alwayson":
		return sdktrace.AlwaysSample()
	case "always_off", "alwaysoff":
		return sdktrace.NeverSample()
	case "traceidratio", "traceid_ratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_traceidratio", "parentbasedtraceidratio", "", "parentbased":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		t.logger.Warnw("unknown sampler strategy, using default", "strategy", cfg.OtelSampler, "default", "parentbased_traceidratio")
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

func (t *Tracer) gitRevision(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}

	return "n/a"
}

func (t *Tracer) buildResource(service string) *resource.Resource {
	version := "n/a"

	var attrs []attribute.KeyValue

	if info, ok := debug.ReadBuildInfo(); ok {
		if service == "" {
			service = info.Path
		}

		if v := strings.TrimSpace(info.Main.Version); v != "" && v != "(devel)" {
			version = v
		}

		attrs = append(attrs,
			semconv.ServiceName(service),
			attribute.String("git_sha", t.gitRevision(info.Settings)),
			attribute.String("app", info.Main.Path),
		)
	} else {
		attrs = append(attrs, semconv.ServiceName(service))
	}

	attrs = append(attrs, semconv.ServiceVersion(version))

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	)
}

func (t *Tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

func (t *Tracer) Shutdown() error {
	if t.shutdown == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return t.shutdown(ctx)
}

func NewTracer(cfg *Config) *Tracer {
	t := new(Tracer)
	t.logger = cfg.Logger

	if !cfg.Enabled {
		t.tracer = noop.NewTracerProvider().Tracer(serviceName)
		t.shutdown = func(context.Context) error { return nil }
		return t
	}

	var err error
	var exporter sdktrace.SpanExporter

	if cfg.OtelGRPCEndpoint != "" {
		exporter, err = otlptrace.New(
			context.TODO(),
			otlptracegrpc.NewClient(
				otlptracegrpc.WithEndpoint(cfg.OtelGRPCEndpoint),
				otlptracegrpc.WithInsecure(),
			),
		)
	} else if cfg.OtelHTTPEndpoint != "" {
		exporter, err = otlptrace.New(
			context.TODO(),
			otlptracehttp.NewClient(
				otlptracehttp.WithEndpoint(cfg.OtelHTTPEndpoint),
				otlptracehttp.WithInsecure(),
			),
		)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}

	if err != nil {
		t.logger.Errorw("unable to initialize tracing exporter", "error", err)
		return t
	}

	t.init(serviceName, t.buildSampler(cfg), exporter)
	return t
}

func NewNoopTracer() *Tracer {
	return NewTracer(NewNoopConfig())
}
