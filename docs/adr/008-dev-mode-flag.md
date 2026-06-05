# ADR 008: Development Mode Flag

## Status

Proposed

## Context

The SAML provider currently has several behaviors that
are appropriate for local development but represent
security concerns in production, and vice versa. There
is no single mechanism to distinguish between the two
environments, so insecure defaults leak into production
or secure defaults break local development.

### Session cookie missing `Secure` attribute

In `internal/handler/oidc_callback.go`, the session
cookie is set without the `Secure` attribute:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "saml_session",
    Value:    session.ID,
    Path:     "/",
    MaxAge:   int(time.Until(session.ExpireTime).Seconds()),
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
})
```

This is a problem in both directions:

- **Production:** Without `Secure: true`, the browser
  sends the cookie over plain HTTP, exposing the session
  ID to network sniffers. This is a session hijacking
  vulnerability.
- **Development:** If `Secure: true` is set
  unconditionally, the cookie is silently dropped on
  `http://localhost` since there is no TLS, breaking
  the entire authentication flow.

### Unconditional insecure OIDC issuer validation

In `internal/infrastructure/hydra/client.go`,
`oidc.InsecureIssuerURLContext` is called
unconditionally:

```go
ctx = oidc.InsecureIssuerURLContext(ctx, cfg.IssuerURL)
```

This bypasses issuer URL validation, allowing the OIDC
provider's reported issuer to differ from the configured
URL. While necessary in local development (where Hydra
may be accessed via different hostnames inside and
outside Docker), this should not be active in production
as it weakens OIDC token validation.

### Logger output format coupled to log level

In `internal/logging/zap.go`, the output format is
determined by the log level:

```go
zapCfg := zap.NewProductionConfig()
if level == "debug" {
    zapCfg = zap.NewDevelopmentConfig()
}
```

This conflates two independent concerns: verbosity
(which level to log at) and format (JSON for machine
consumption vs. human-readable for local development).
A production operator may temporarily need `debug` level
without switching to development output format, and a
developer may want `info` level without JSON output.

## Proposed Solutions

### Option A: Single boolean environment variable

Add `SAML_PROVIDER_DEV_MODE` (default `false`) to
`internal/app/config.go`. Propagate it through the
existing config structs (`HandlerConfig`, `hydra.Config`)
to the three affected locations.

When `DevMode` is `true`:

- Session cookie: `Secure: false`
- Hydra client: `InsecureIssuerURLContext` is applied
- Logger: uses development config (human-readable)

When `DevMode` is `false` (production, the default):

- Session cookie: `Secure: true`
- Hydra client: strict issuer URL validation
- Logger: uses production config (JSON)

### Option B: Per-feature environment variables

Add three separate environment variables:

- `SAML_PROVIDER_COOKIE_SECURE` (default `true`)
- `SAML_PROVIDER_OIDC_INSECURE_ISSUER` (default `false`)
- `SAML_PROVIDER_LOG_FORMAT` (default `json`)

Each behavior is independently configurable.

### Option C: Infer from `BridgeBaseURL` scheme

Detect whether `SAML_PROVIDER_BRIDGE_BASE_URL` uses
`http://` or `https://` and derive security settings
automatically. No new environment variable needed.

### Comparison

| Criteria | Option A (single flag) | Option B (per-feature) | Option C (infer) |
| --- | --- | --- | --- |
| Config complexity | 1 new env var | 3 new env vars | 0 new env vars |
| Operator clarity | High — one toggle | Moderate — three toggles | Low — implicit |
| Override granularity | All-or-nothing | Full | None |
| Risk of misconfiguration | Low | Moderate | Low |
| Discoverability | High | Low | Very low |

## Decision

Use **Option A: single boolean environment variable**
with the name `SAML_PROVIDER_DEV_MODE`.

Rationale:

1. **Single toggle.** One environment variable controls
   all dev-vs-prod behaviors. Operators cannot
   accidentally enable insecure cookies while leaving
   issuer validation strict, or vice versa. The security
   posture is consistent and auditable.

2. **Safe default.** `DevMode` defaults to `false`,
   meaning production security is the default. Insecure
   behaviors require an explicit opt-in. This follows
   the principle of secure-by-default.

3. **Minimal config surface.** Adding one boolean is
   simpler to document, test, and reason about than
   three independent knobs (Option B) or implicit
   inference that may surprise operators (Option C).

4. **Discoverable.** A single `SAML_PROVIDER_DEV_MODE`
   variable is easy to find in documentation. Option C
   hides the behavior behind URL parsing, making it
   difficult to understand why the application behaves
   differently across environments.

5. **Extensible.** Future dev-vs-prod behaviors (e.g.,
   relaxed config validation, verbose startup logging)
   can be gated on the same flag without adding more
   environment variables.

## Implementation

### Files to modify

| File | Change |
| --- | --- |
| `internal/app/config.go` | Add `DevMode bool` field |
| `internal/app/config.go` | Log a warning in `Validate()` when `DevMode` is `true` |
| `internal/app/app.go` | Propagate `DevMode` to `HandlerConfig` and `hydra.Config` |
| `internal/handler/handler.go` | Add `DevMode` to `HandlerConfig` |
| `internal/handler/oidc_callback.go` | Set `Secure: !h.config.DevMode` on the session cookie |
| `internal/infrastructure/hydra/client.go` | Gate `InsecureIssuerURLContext` on `DevMode` |
| `internal/infrastructure/hydra/client.go` | Add `DevMode` to `hydra.Config` |
| `internal/logging/zap.go` | Add `devMode` parameter to `BuildLogger` |
| `internal/cmd/serve.go` | Pass `cfg.DevMode` to `BuildLogger` |

### Startup warning

When `DevMode` is `true`, the application must log a
prominent warning at startup:

```text
WARNING: Running in development mode. Secure cookie
attribute is disabled and OIDC issuer validation is
relaxed. Do not use in production.
```

This ensures operators are aware of the reduced security
posture and can catch accidental dev-mode deployments.

## Consequences

- The session cookie gains the `Secure` attribute in
  production, fixing a session hijacking vulnerability
  that exists today.
- `InsecureIssuerURLContext` is no longer called in
  production, strengthening OIDC token validation.
- Logger format becomes independent of log level,
  allowing `debug`-level logging in production without
  switching to human-readable output.
- Local development requires setting
  `SAML_PROVIDER_DEV_MODE=true` in addition to existing
  environment variables. The `Makefile` `run` target
  should set this automatically.
- Existing deployments that do not set the variable
  default to production mode, which is the secure
  choice. However, any deployment currently relying
  on `InsecureIssuerURLContext` without setting the
  flag will experience a behavior change — OIDC
  discovery will fail if the issuer URL does not
  match exactly.
