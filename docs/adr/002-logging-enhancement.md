# ADR 002: Logging Enhancement — Consistent Interface, Request-Scoped Fields, and Log Hygiene

## Status

Accepted

## Context

The project uses `go.uber.org/zap` for structured logging
and defines a `logging.Logger` interface in
`internal/logging/interfaces.go`:

```go
type Logger interface {
    Infow(msg string, keysAndValues ...interface{})
    Warnw(msg string, keysAndValues ...interface{})
    Errorw(msg string, keysAndValues ...interface{})
    Debugw(msg string, keysAndValues ...interface{})
}
```

`*zap.SugaredLogger` satisfies this interface natively.
The service and handler layers accept `logging.Logger`
via dependency injection — a solid foundation. However,
several issues have been identified, listed below in
order of severity.

### 1. Inconsistent use of the logging interface

The service and handler layers use the `logging.Logger`
interface, but the monitoring, tracing, and prometheus
packages accept the concrete `*zap.SugaredLogger` type:

| Package                           | Logger type            |
|-----------------------------------|------------------------|
| `internal/service/*`              | `logging.Logger` ✅    |
| `internal/handler/*`              | `logging.Logger` ✅    |
| `internal/monitoring/middlewares` | `*zap.SugaredLogger` ❌|
| `internal/monitoring/noop`        | `*zap.SugaredLogger` ❌|
| `internal/monitoring/prometheus`  | `*zap.SugaredLogger` ❌|
| `internal/tracing/middleware`     | `*zap.SugaredLogger` ❌|
| `internal/tracing/config`         | `*zap.SugaredLogger` ❌|
| `internal/tracing/tracer`         | `*zap.SugaredLogger` ❌|

This tight coupling makes testing harder (requires real
zap loggers or `zaptest`) and breaks the abstraction the
project already defined.

### 2. No request-scoped log enrichment

The `logging.Logger` interface methods don't take a
`context.Context`. There is no mechanism to attach
request-scoped data (trace ID, request ID) to log lines
automatically. Handlers manually add fields like
`"requestID", requestID` ad hoc, which is inconsistent
and easy to forget.

### 3. No environment-driven log level configuration

Log level is only toggled via a `--verbose` CLI flag
(development vs production), which is a binary switch
with no granularity. There is no environment variable
(e.g., `SAML_PROVIDER_LOG_LEVEL`) to configure the log
level at runtime — a common need in container/Kubernetes
deployments where changing entrypoint arguments is
impractical.

### 4. Excessive Info-level logging

Several log statements fire on every single request at
`Infow` level:

- `"Checking for existing SAML session"` (every request)
- `"Found session cookie"` (every authenticated request)
- `"No session cookie found"` (every unauthenticated
  request)
- `"Handling OIDC callback from Hydra"` (every callback)

These are diagnostic-level messages that create noise in
production.

### 5. Potentially sensitive data in logs

User email addresses are logged at Info level in
`service/oidc.go` and `service/session.go`. Depending on
data classification policies, this may constitute PII
leakage in production logs.

### 6. No HTTP request logging middleware

The monitoring middleware tracks response time metrics,
but there is no general-purpose access log middleware that
logs method, path, status code, duration, and trace ID
for each HTTP request.

### 7. Minor issues

The following low-severity issues are grouped together:

- **Duplicate logging between layers.** The same event
  is logged in both the handler and service layers. For
  example, when registering an SP, `handler/admin.go`
  logs `"Service provider registered"` and
  `"Failed to register SP"`, while
  `service/service_provider.go` logs the same events
  with nearly identical messages. This produces duplicate
  log lines for a single operation.

- **`Fatalw` excluded from `logging.Logger` interface.**
  The interface only defines `Infow`, `Warnw`, `Errorw`,
  `Debugw`. `Fatalw` is used in `serve.go` via the
  concrete `*zap.SugaredLogger`. This is intentional —
  `Fatalw` calls `os.Exit(1)` and should be limited to
  the top-level CLI entrypoint — but it is undocumented.

- **`zapLogger.Sync()` error suppression.** In
  `serve.go`, `defer zapLogger.Sync()` carries an
  `//nolint:errcheck` comment without explaining why the
  error is safe to ignore (zap's `Sync` commonly returns
  a benign error when writing to stdout/stderr on Linux).

## Decision

### 1. Migrate all packages to `logging.Logger` interface

All internal packages that accept a logger will use the
`logging.Logger` interface instead of the concrete
`*zap.SugaredLogger`. This applies to `monitoring`,
`tracing`, and `prometheus` packages.

The `logging.Logger` interface will be extended with a
`With` method to support field enrichment:

```go
type Logger interface {
    Infow(msg string, keysAndValues ...interface{})
    Warnw(msg string, keysAndValues ...interface{})
    Errorw(msg string, keysAndValues ...interface{})
    Debugw(msg string, keysAndValues ...interface{})
    With(args ...interface{}) Logger
}
```

`*zap.SugaredLogger` already has a `With` method, but
its return type is `*zap.SugaredLogger`, not
`logging.Logger`. A thin wrapper will be needed to
satisfy the interface:

```go
type ZapLogger struct {
    *zap.SugaredLogger
}

func (z *ZapLogger) With(args ...interface{}) Logger {
    return &ZapLogger{z.SugaredLogger.With(args...)}
}
```

`Fatalw` is intentionally excluded from the interface.
Fatal logging calls `os.Exit(1)` and should only be used
at the top-level CLI entrypoint (`internal/cmd/serve.go`),
which already uses the concrete `*zap.SugaredLogger`
directly. Putting `Fatalw` in the interface would make
services untestable (tests would exit) and conflate
logging with process lifecycle control.

The `logging` package also provides convenience
constructors to centralise logger creation and keep
zap out of calling code:

- **`NewNopLogger()`** — returns a no-op `Logger` for
  CLI commands and tests.
- **`BuildLogger(level)`** — constructs a production-ready
  `Logger` from a level string (`debug`, `info`, `warn`,
  `error`), returning the wrapped logger, the underlying
  `*zap.Logger` (for `Sync`/`Fatalw`), and any error.

### 2. Add context-based logger enrichment for request-scoped fields

A context-based logger pattern will be introduced in
`internal/logging/` to carry request-scoped enriched
loggers:

```go
// internal/logging/context.go

type ctxKey struct{}

func WithLogger(ctx context.Context, l Logger) context.Context {
    return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context, fallback Logger) Logger {
    if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
        return l
    }
    return fallback
}
```

This follows the "base logger via DI + enriched logger in
context" pattern:

- **Structural dependency**: Services receive a base
  `logging.Logger` via constructor injection (unchanged).
- **Request-scoped enrichment**: An HTTP middleware
  enriches the base logger with request ID, trace ID,
  method, and path, then stores it in the context.
- **Usage**: Service methods call
  `logging.FromContext(ctx, s.logger)` to obtain the
  enriched logger, falling back to the base logger if
  no enriched logger is present (e.g., in tests or
  non-HTTP code paths).

This approach was chosen over a pure context-based logger
(no DI) because:

- **Explicit dependencies**: The base logger remains
  visible in struct fields and constructor signatures,
  following Go's preference for explicit over implicit.
- **Safe fallback**: If middleware is not applied (tests,
  CLI commands, background jobs), `FromContext` falls
  back to the injected base logger — no nil panics or
  silent noop loggers.
- **Incremental adoption**: Existing code continues to
  work with `s.logger` directly. Migration to
  `FromContext` can happen incrementally per function.

### 3. Add environment-driven log level configuration

A new config field will be added:

```go
type Config struct {
    // ...existing fields...
    LogLevel string `envconfig:"SAML_PROVIDER_LOG_LEVEL" default:"info"`
}
```

Accepted values: `debug`, `info`, `warn`, `error`.

The former `--verbose` CLI flag is removed. All
configuration in this project is environment-driven
(via `envconfig`), and `--verbose` was simply a special
case of `LOG_LEVEL=debug`. Removing it eliminates
precedence ambiguity and keeps configuration consistent.

For local development, use:

```bash
SAML_PROVIDER_LOG_LEVEL=debug make run
```

### 4. Demote diagnostic logs to Debug level

Log statements that fire on every request but provide no
actionable information will be demoted from `Infow` to
`Debugw`:

| Current message                       | Current | Target  |
|---------------------------------------|---------|---------|
| "Checking for existing SAML session"  | Infow   | Debugw  |
| "Found session cookie"                | Infow   | Debugw  |
| "No session cookie found"             | Infow   | Debugw  |
| "No valid session found, redirecting" | Infow   | Debugw  |
| "Handling OIDC callback from Hydra"   | Infow   | Debugw  |
| "Session created, redirecting back"   | Infow   | Debugw  |

Actionable events (session created, SP registered, errors)
remain at `Infow` or `Errorw`.

### 5. Remove PII from default production logs

User email addresses will no longer be logged at Info
level. The `"email"` field will either be:

- Removed from Info-level logs entirely, or
- Moved to Debug-level logs only

Identifiers like session ID and OIDC subject (`sub`) are
acceptable at Info level as they are opaque identifiers.

### 6. Add HTTP request logging middleware

A chi-compatible middleware will be added that logs each
HTTP request with:

- Method, path, status code, duration
- Request ID (generated or extracted from header)
- Trace ID (from OpenTelemetry span context, if present)

The middleware will log at `Infow` level for successful
requests and `Warnw` or `Errorw` for 4xx/5xx responses.
This replaces the ad-hoc handler-level logging of
`"Handling OIDC callback from Hydra"` etc.

### 7. Address minor issues

- **Eliminate duplicate logging between layers.** Each
  log event will be logged once, at the appropriate
  layer:

  | Event                 | Log at  | Remove from |
  |-----------------------|---------|-------------|
  | SP registered         | Service | Handler     |
  | SP registration error | Service | Handler     |
  | Session created       | Service | Handler     |
  | OIDC exchange error   | Service | Handler     |

  The handler layer will log only HTTP-specific concerns
  (e.g., invalid JSON input, redirect decisions).
  Business events are the service layer's
  responsibility.

- **Document `Fatalw` exclusion.** Add a code comment
  to `internal/logging/interfaces.go` explaining why
  `Fatalw` is intentionally omitted.

- **Improve `Sync()` lint suppression comment.** Replace
  the bare `//nolint:errcheck` on `defer zapLogger.Sync()`
  in `serve.go` with a comment explaining that `Sync`
  returns a benign error on stdout/stderr under Linux.

## Consequences

### Benefits

- **Testable throughout**: All packages accept the
  `logging.Logger` interface, enabling mock-based testing
  without real zap loggers.
- **Automatic request context**: Every log line in a
  request handler chain includes request ID and trace ID
  without manual plumbing.
- **Operator-friendly**: Log level configurable via
  environment variable — no rebuild needed.
- **Reduced production noise**: Diagnostic messages at
  Debug level keep production logs clean and actionable.
- **Privacy-safe defaults**: No PII in production logs
  at default (info) level.
- **No duplicate log lines**: Each event logged once, at
  the right layer.

### Drawbacks

- **Wrapper type for zap**: The `With` method requires a
  thin `ZapLogger` wrapper around `*zap.SugaredLogger`
  since zap's `With` returns the concrete type, not the
  interface. This is a one-time cost and is minimal.
- **Migration effort**: All monitoring/tracing packages
  must be updated to accept `logging.Logger`.
  Constructor signatures change, requiring updates to
  `app.Build()` and tests. This is mechanical but
  touches many files.
- **Context discipline**: Developers must remember to
  use `logging.FromContext(ctx, s.logger)` instead of
  `s.logger` directly when request-scoped enrichment is
  desired. Linting or code review conventions can
  enforce this over time.
- **Behaviour change for log consumers**: If operators
  have alerts or dashboards based on current Info-level
  messages that are being demoted to Debug, those will
  need updating. This should be communicated in release
  notes.

## Implementation Order

The changes should be applied in the following sequence
to minimize disruption:

1. **Extend `logging.Logger` interface** — add `With`
   method and `ZapLogger` wrapper
2. **Add context helpers** — `WithLogger`, `FromContext`
   in `internal/logging/context.go`
3. **Add `SAML_PROVIDER_LOG_LEVEL` config** — in
   `internal/app/config.go` and `internal/cmd/serve.go`
4. **Migrate monitoring/tracing to interface** — update
   constructors to accept `logging.Logger`
5. **Demote diagnostic logs to Debug** — update log
   levels across handler and adapter code
6. **Remove PII from Info logs** — move email to Debug
   level
7. **Add HTTP request logging middleware** — new
   middleware in `internal/logging/` or
   `internal/handler/`
8. **Address minor issues** — remove duplicate logs,
   add documentation comments for `Fatalw` exclusion
   and `Sync()` error suppression
9. **Adopt `FromContext` in services/handlers** —
   incremental, can be done per-file
