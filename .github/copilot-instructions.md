---
description: >-
  Core project instructions for the Identity SAML
  Provider, a SAML-to-OIDC bridge via Ory Hydra
applyTo: '**'
---

# AI Coding Agent Instructions - Identity SAML Provider

## 1. Core Guardrails & System Constraints

### Global Rules

- 🚫 **No External Dependencies in Domain:** The
  domain layer must have zero external imports.
- ⚠️ **Context Handling:** Always pass
  `context.Context` as the first parameter to
  service/repository methods. *Never* store context
  inside a struct field.
- **Configuration:** All config must look for the
  environment variable prefix `SAML_PROVIDER_`.

### Verification Enforcement

- **Mandatory Checks:** Whenever you modify, create,
  or refactor Go source code, you must run the
  verification suite (`make build`, `make fmt`,
  `make lint`, `make test`, and
  `make license-check`).
- **Zero-Tolerance for Warnings:** Do not consider a
  task complete if any of these verification commands
  return a non-zero exit code or linting warnings.

### License Header

Every new Go file (excluding `mocks/` and `vendor/`)
must start with this exact header as the first lines
of the file, followed by a single blank line:

```go
// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only
```

*(Note: Do not update the year when modifying
existing files).*

---

## 2. Architectural Layout & Boundaries

The application enforces a strict **Inward Clean
Architecture** layout. Outer layers depend on inner
layer interfaces, *never* on concrete implementations.

- `internal/domain/`: Entities, value objects, and
  typed errors. Entities own `Validate() error`
  methods.
- `internal/repository/`: Persistence interfaces
  (`interfaces.go`) and implementations (`postgres/`,
  `memory/`).
- `internal/service/`: Business logic orchestration
  (`interfaces.go`). Must only return typed domain
  errors.
- `internal/handler/`: HTTP handlers, SAML adapters,
  and DTOs. Maps domain errors to HTTP status codes.
- `internal/app/`: Composition root. Wires
  pool → repos → services → handlers → HTTP server.

### Core Authentication Flow

For context on how these layers interact at runtime:

1. SP AuthnRequest → `/saml/sso` (Bridge stores
   pending request, redirects to Hydra login).
2. User authenticates via Hydra OIDC provider.
3. Hydra returns ID token with claims →
   `/saml/callback`.
4. Bridge maps OIDC claims to SAML assertion,
   returns SAML Response to SP ACS URL.

*(Note: See `docs/authentication-flow/` for
details).*

---

## 3. Coding Patterns (Correct vs. Incorrect)

### Code Style Precedence

1. Project-specific conventions defined in this file.
1. Local Canonical Go Style Guide
   (See: `.github/agents/go-style-guide.md`).
1. Effective Go conventions.

### Error Handling

Services return typed domain errors; handlers map
them to HTTP statuses. Do not wrap or obscure domain
errors.

- ✅ **CORRECT:**

```go
func (s *SessionService) Get(
    ctx context.Context,
    id string,
) (*domain.Session, error) {
    session, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, domain.ErrNotFound("session", id)
    }
    return session, nil
}
```

- ❌ **INCORRECT:**

```go
func (s *SessionService) Get(
    ctx context.Context,
    id string,
) (*domain.Session, error) {
    session, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf(
            "session not found: %w", err,
        )
    }
}
```

### Logging & Context

Use the structured `logging.Logger` interface.
Extract request-scoped loggers from context.

- ✅ **CORRECT:**

```go
logging.FromContext(r.Context()).Infow(
    "processing OIDC callback",
    "request_id", requestID,
)
```

- ❌ **INCORRECT:**

```go
zap.L().Info("processing OIDC callback")
```

### Dependency Injection

Use constructor injection of interfaces exclusively.
Functions accept interfaces, return concrete structs.

- ✅ **CORRECT:**

```go
type SessionService struct {
    repo   repository.SessionRepository
    logger logging.Logger
}

func NewSessionService(
    repo repository.SessionRepository,
    logger logging.Logger,
) *SessionService {
    return &SessionService{
        repo:   repo,
        logger: logger,
    }
}
```

- ❌ **INCORRECT:**

```go
type SessionService struct {
    repo   *postgres.SessionRepo
    logger *zap.SugaredLogger
}
```

### Mocks

- Generated via `go.uber.org/mock/mockgen`.
- Add the `//go:generate` directive above interface
  declarations.
- Run `make generate` to recreate. Never edit files
  in `mocks/` manually.

---

## 4. Developer & AI Playbooks

### 📋 Checklist - Adding a New Feature

When asked to implement a new feature or endpoint,
you must follow these steps in order:

1. Define domain entities/errors in
   `internal/domain/`.
2. Add repository interface in
   `repository/interfaces.go`, implement in
   `postgres/` or `memory/`.
3. Add service interface in
   `service/interfaces.go`, implement business logic.
4. Run `make generate` to update mocks.
5. Add handler in `internal/handler/` and register
   routes in `routes.go`.
6. Wire dependencies in `internal/app/app.go`.
7. Add sequential Goose migrations in `migrations/`
   if database changes are required.
8. Write table-driven unit tests (see `*_test.go`
   patterns).
9. Execute the verification suite (`make build`,
   `make fmt`, `make lint`, `make test`,
   `make license-check`). Fix any issues before
   presenting the solution.

### Verification Commands

| Command | Purpose |
| ------- | ------- |
| `make build` | Build binary |
| `make test` | Run table-driven unit tests |
| `make lint` | Run golangci-lint |
| `make fmt` | Format source code |
| `make generate` | Regenerate Go mocks |
| `make license-check` | Verify AGPL-3.0-only headers |
| `make migrate-up` | Apply Goose DB migrations |
| `make migrate-down` | Roll back last DB migration |
