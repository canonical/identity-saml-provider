# ADR 006: OpenTelemetry Tracing Enhancement

## Status

Proposed

## Context

The application has a functional OpenTelemetry tracing
foundation in `internal/tracing/`. It supports OTLP
(gRPC and HTTP) and stdout exporters, configurable
sampling strategies, composite context propagation
(W3C TraceContext + Baggage + Jaeger), and a noop
fallback when tracing is disabled.

However, instrumentation coverage is shallow. The
current state leaves most of the request lifecycle
invisible in traces.

### Current instrumentation

| Layer          | Component                  | Traced |
|----------------|----------------------------|--------|
| HTTP (auto)    | `otelhttp.NewHandler`      | Yes    |
| HTTP (rename)  | `RouteSpanNameMiddleware`  | Yes    |
| Handler        | `HandleOIDCCallback`       | Yes    |
| Handler        | `HandleRegisterSP`         | Yes    |
| Handler        | `SAMLSessionAdapter`       | No     |
| Handler        | `SAMLSPAdapter`            | No     |
| Service        | `SessionService`           | No     |
| Service        | `OIDCService`              | No     |
| Service        | `MappingService`           | No     |
| Service        | `PendingRequestService`    | No     |
| Service        | `ServiceProviderService`   | No     |
| Repository     | `postgres.SessionRepo`     | No     |
| Repository     | `postgres.SPRepo`          | No     |
| Repository     | `memory.PendingRequestRepo`| No     |
| Infrastructure | `hydra.Client`             | No     |
| Logging        | Trace ID correlation       | Yes    |

### Identified gaps

1. **No service-layer spans.** Business logic such as
   OIDC code exchange, session creation, attribute
   mapping, and SP lookups produce no child spans.
   Operators cannot isolate which step of a multi-step
   flow is slow or failing.

2. **No repository-layer spans.** Database queries are
   invisible in traces. Slow queries or connection
   acquisition delays cannot be attributed from trace
   data alone.

3. **No SAML adapter spans.** `GetSession()` performs
   cookie checks, session lookups, pending request
   storage, OIDC redirects, and attribute mapping —
   all untraced.

4. **No outbound HTTP client instrumentation.** The
   Hydra OIDC client makes external HTTP calls without
   `otelhttp.NewTransport()`. Trace context is not
   propagated to downstream services, and outbound
   call latency is not captured.

5. **No span attributes or error recording.** The two
   existing handler spans never call
   `span.SetAttributes()`, `span.RecordError()`, or
   `span.SetStatus()`. Errors are returned as HTTP
   responses but are not reflected in the trace.

6. **No span events.** Significant state transitions
   (session found, redirecting to OIDC, pending
   request stored) are not recorded as span events.

7. **No tracing tests.** The `internal/tracing/`
   package has no test files. Sampler selection,
   exporter fallback, resource building, and
   middleware behaviour are untested.

8. **`context.TODO()` in exporter init.** `NewTracer()`
   uses `context.TODO()` when creating OTLP exporters
   instead of accepting the application context.

9. **Stale semantic conventions.** The code imports
    `semconv/v1.18.0` while the OTel SDK dependency is
    at `v1.43.0`.

10. **Dead code.** `Config.TracingConfig()` in
    `internal/app/config.go` is defined but never
    called — `app.Build()` constructs the tracing
    config inline.

11. **Unused logger field.** The tracing `Middleware`
    struct holds a `logger` field that is never
    referenced by any method.

## Decision

We will enhance tracing instrumentation across all
application layers, following the principle that each
layer should produce spans that are useful for
debugging latency and errors in the SAML-OIDC bridge
flow.

### 1. Add service-layer spans

Each public service method will create a child span
named with the convention `service.<domain>.<method>`:

| Span name                         | Service                  |
|-----------------------------------|--------------------------|
| `service.oidc.exchange_code`      | `OIDCService`            |
| `service.oidc.auth_code_url`      | `OIDCService`            |
| `service.session.create_from_oidc`| `SessionService`         |
| `service.session.get_by_id`       | `SessionService`         |
| `service.mapping.apply_mapping`   | `MappingService`         |
| `service.sp.register`             | `ServiceProviderService` |
| `service.sp.get_by_entity_id`     | `ServiceProviderService` |
| `service.pending.store`           | `PendingRequestService`  |
| `service.pending.retrieve`        | `PendingRequestService`  |

The `TracingInterface` will be injected into each
service constructor alongside the existing logger.
Services that currently accept only a repository and
a logger will gain a tracer parameter.

### 2. Add repository-layer spans

Each repository method will create a child span named
with the convention `repo.<store>.<method>`:

| Span name                    | Repository              |
|------------------------------|-------------------------|
| `repo.postgres.save_session` | `postgres.SessionRepo`  |
| `repo.postgres.get_session`  | `postgres.SPRepo`       |
| `repo.postgres.save_sp`      | `postgres.SPRepo`       |
| `repo.postgres.get_sp`       | `postgres.SPRepo`       |
| `repo.memory.store_pending`  | `memory.PendingReqRepo` |
| `repo.memory.get_pending`    | `memory.PendingReqRepo` |

Repository spans will include a `db.system` attribute
set to `"postgresql"` or `"memory"` following OTel
semantic conventions for database spans.

### 3. Add SAML adapter spans

`SAMLSessionAdapter.GetSession()` will create a span
`handler.saml.get_session` with events for key state
transitions:

- `session_cookie_found` / `session_cookie_missing`
- `session_lookup_hit` / `session_lookup_miss`
- `pending_request_stored`
- `oidc_redirect_initiated`
- `attribute_mapping_applied`

`SAMLSPAdapter.GetServiceProvider()` will create a
span `handler.saml.get_service_provider`.

The tracer will be added to both adapter structs.

### 4. Instrument outbound HTTP client

The Hydra OIDC client (`infrastructure/hydra/`) will
wrap its `http.Client` transport with
`otelhttp.NewTransport()`. This produces child spans
for outbound HTTP calls and propagates trace context
to Hydra, enabling end-to-end distributed tracing.

### 5. Record errors and attributes on spans

All spans created in handlers, services, and
repositories will follow this error-handling pattern:

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    return err
}
```

Handler spans will set attributes for key request
parameters using the convention
`<layer>.<domain>.<field>`:

- `handler.callback.has_code` (bool)
- `handler.callback.has_state` (bool)
- `handler.admin.entity_id` (string)

Service and repository spans will set relevant
domain attributes:

- `service.session.session_id` (string)
- `repo.postgres.rows_affected` (int)

All attribute values must be low-cardinality or
bounded. User-identifiable information (emails,
subjects) must not appear in span attributes.

### 6. Add span events for state transitions

Handlers and adapters will use `span.AddEvent()` to
mark significant state transitions within a span.
Events are lighter than child spans and provide
timeline markers:

```go
span.AddEvent("oidc_code_exchanged")
span.AddEvent("session_created",
    trace.WithAttributes(
        attribute.String("session_id", session.ID),
    ),
)
```

### 7. Accept context in `NewTracer()`

`NewTracer()` will accept a `context.Context` parameter
so that exporter initialization uses the application
lifecycle context. This ensures that if the application
shuts down during initialization, the exporter creation
is properly cancelled.

```go
func NewTracer(ctx context.Context, cfg *Config) *Tracer
```

### 8. Update semantic conventions

The import will be updated from `semconv/v1.18.0` to
the latest stable version compatible with the current
OTel SDK dependency. Resource attributes will be
updated to use the newer attribute keys where they
have changed.

### 9. Remove dead code

- Remove `Config.TracingConfig()` from
  `internal/app/config.go` since `app.Build()`
  constructs the config inline.
- Either use or remove the `logger` field from
  `tracing.Middleware`. If retained, use it to log
  exporter connection errors during middleware
  initialization.

### 10. Standardise span naming convention

All spans will follow a layered naming convention:

| Layer          | Pattern                           |
|----------------|-----------------------------------|
| HTTP (auto)    | `GET /saml/sso` (from middleware) |
| Handler        | `handler.<domain>.<operation>`    |
| Service        | `service.<domain>.<operation>`    |
| Repository     | `repo.<store>.<operation>`        |
| Infrastructure | Auto from `otelhttp` transport    |

This produces a consistent, hierarchical trace tree
that maps directly to the application architecture.

### 11. Add tracing tests

Tests will cover:

- Sampler selection for each strategy string
- Exporter fallback (gRPC → HTTP → stdout)
- Resource attribute population from build info
- Noop tracer behaviour when tracing is disabled
- Middleware span renaming with chi route patterns

Tests will use `sdktrace.NewTracerProvider` with a
`tracetest.InMemoryExporter` to assert span names,
attributes, status, and events without requiring an
external collector.

## Consequences

### Benefits

- **Full request lifecycle visibility.** Every layer
  of the SAML-OIDC bridge flow produces spans,
  enabling operators to pinpoint latency and errors
  to a specific service method or database query.
- **Distributed tracing across services.** Outbound
  HTTP client instrumentation propagates trace context
  to Hydra, enabling end-to-end trace correlation.
- **Error attribution in traces.** `RecordError` and
  `SetStatus` on spans make errors visible in trace
  backends (Jaeger, Tempo, etc.) without requiring
  log correlation.
- **Richer trace timelines.** Span events mark state
  transitions, making it easier to understand the
  flow within a single span without needing additional
  child spans.
- **Tested tracing infrastructure.** Unit tests
  prevent regressions in sampler logic, exporter
  selection, and middleware behaviour.
- **Consistent naming.** A standardised span naming
  convention makes traces readable and filterable
  across all layers.

### Drawbacks

- **Increased constructor complexity.** Every service
  and repository constructor gains a tracer parameter.
  This is mechanical but touches many files and their
  tests.
- **Performance overhead.** Additional spans increase
  memory allocation and export bandwidth. The default
  `parentbased_traceidratio` sampler at 10% mitigates
  this, but operators must be aware of the cost when
  using `always_on` in production.
- **Tracer dependency in service layer.** Services
  currently depend only on repositories and a logger.
  Adding a tracer introduces a new cross-cutting
  dependency. This is mitigated by the existing
  `TracingInterface` abstraction and the noop
  implementation for tests.
- **Span attribute cardinality risk.** Adding
  attributes to spans requires discipline to avoid
  high-cardinality values. The attribute policy in
  Decision 5 mitigates this, but it must be enforced
  through code review.
- **Breaking change in `NewTracer()` signature.**
  Adding `context.Context` to `NewTracer()` requires
  updating all call sites. There are only two
  (`app.Build()` and `NewNoopTracer()`), so the
  impact is minimal.
