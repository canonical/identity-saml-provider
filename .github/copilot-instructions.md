---
description: 'Core project instructions for the Identity SAML Provider, a SAML-to-OIDC bridge via Ory Hydra'
applyTo: '**'
---

# AI Coding Agent Instructions - Identity SAML Provider

## Project Overview

**Identity SAML Provider** is a SAML-to-OIDC bridge that
translates SAML authentication requests to OIDC flows via
Ory Hydra. It enables SAML Service Providers to
authenticate through modern OIDC providers.

## General Instructions

- Clean architecture: Domain → Repository → Service →
  Handler → App (composition root)
- All dependencies flow inward via interfaces
- Config via environment variables prefixed
  `SAML_PROVIDER_`
- Domain layer has zero external dependencies
- Handlers delegate to services; services return typed
  domain errors

## Architecture

The application follows clean architecture with strict
layer separation:

- **Domain Layer** (`internal/domain/`): Entities, value
  objects, typed errors. No external dependencies.
  Entities own `Validate()` methods.
- **Repository Layer** (`internal/repository/`):
  Persistence interfaces and implementations
  (Postgres, in-memory). Defined in `interfaces.go`.
- **Service Layer** (`internal/service/`): Business logic
  and orchestration. Defined in `interfaces.go`. Returns
  domain errors only.
- **Handler Layer** (`internal/handler/`): HTTP handlers,
  SAML adapters, request/response DTOs. Maps domain
  errors to HTTP status codes.
- **App Layer** (`internal/app/`): Composition root.
  Wires pool → repos → services → handlers → HTTP
  server.

Dependencies flow strictly inward. Outer layers depend
on inner layer interfaces, never on concrete
implementations.

### Authentication Flow

1. SAML Service Provider sends AuthnRequest →
   `/saml/sso`
2. Bridge stores pending request, redirects user to
   Hydra login
3. User authenticates with Hydra (OIDC provider)
4. Hydra returns ID token with user claims →
   `/saml/callback`
5. Bridge converts OIDC claims to SAML assertion
6. Bridge returns SAML Response to Service Provider
   ACS URL

See `docs/authentication-flow/` for detailed sequence
diagrams and edge cases.

## Code Standards

Follow these references in order of precedence:

1. Project-specific conventions (below)
2. [Canonical Go Style Guide](https://github.com/canonical/copilot-collections/blob/main/assets/agents/go-style-guide.md)
3. [Effective Go](https://go.dev/doc/effective_go)

### License Header

Every new Go file (excluding generated files in `mocks/`
and `vendor/`) must start with the following license
header:

```go
// Copyright <YEAR> Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only
```

Replace `<YEAR>` with the current calendar year at the
time the file is created. Do not update the year when
modifying existing files.

The header must be the very first lines of the file,
followed by a blank line before the `package` statement.

### Error Handling

Use typed domain errors; services return domain errors,
handlers map them to HTTP status codes.

#### Correct Error Handling

```go
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, error) {
    session, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, domain.ErrNotFound("session", id)
    }
    return session, nil
}
```

#### Incorrect Error Handling

```go
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, error) {
    session, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("session not found: %w", err) // loses typed error semantics
    }
    return session, nil
}
```

### Logging

Use `logging.Logger` interface with structured key-value
pairs. Use `logging.FromContext(ctx)` for request-scoped
loggers.

#### Correct Logging

```go
func (h *Handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
    logger := logging.FromContext(r.Context())
    logger.Infow("processing OIDC callback", "request_id", requestID)
}
```

#### Incorrect Logging

```go
func (h *Handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
    zap.L().Info("processing OIDC callback") // bypasses interface, loses request context
}
```

### Dependency Injection

Use constructor injection of interfaces everywhere. Use
`logging.Logger`, `monitoring.MonitorInterface`,
`tracing.TracingInterface` (not concrete types).

#### Correct Dependency Injection

```go
type SessionService struct {
    repo   repository.SessionRepository
    logger logging.Logger
}

func NewSessionService(repo repository.SessionRepository, logger logging.Logger) *SessionService {
    return &SessionService{repo: repo, logger: logger}
}
```

#### Incorrect Dependency Injection

```go
type SessionService struct {
    repo   *postgres.SessionRepo  // concrete type, not interface
    logger *zap.SugaredLogger     // concrete logger, not interface
}
```

### Mocks

- Generated via `go.uber.org/mock/mockgen`
- Run `make generate` after changing interfaces
- Files in `mocks/` are generated — never edit manually

## Design Principles & Patterns

### SOLID Principles in Go

| Principle | Go Interpretation | Project Example |
| --- | --- | --- |
| **Single Responsibility** | Each struct/package does one thing | `SessionService` handles session logic only; `Handlers` handles HTTP only |
| **Open/Closed** | Extend behavior via interfaces, not by modifying existing code | Add new repository implementations without changing service code |
| **Liskov Substitution** | Any interface implementation is interchangeable | `postgres.SessionRepo` and `memory.SessionRepo` both satisfy `repository.SessionRepository` |
| **Interface Segregation** | Define small, focused interfaces | `repository.SessionRepository` is separate from `repository.ServiceProviderRepository` |
| **Dependency Inversion** | Depend on interfaces, not concrete types | Services accept `repository.X` interfaces, never `*postgres.XRepo` |

### Common Go Design Patterns

- **Constructor Pattern**: Use `NewXxx()` functions that
  accept interface dependencies and return concrete
  types. Every service, repository, and handler follows
  this.
- **Dependency Injection**: Use constructor injection
  exclusively. Accept interfaces, wire in
  `internal/app/app.go` (composition root). No service
  locators, no global state, no init-time side effects.
- **Adapter Pattern**: Wrap external library interfaces
  with project-specific implementations. Used for
  `crewjam/saml` via `SAMLSPAdapter` and
  `SAMLSessionAdapter`.
- **Repository Pattern**: Abstract persistence behind
  interfaces defined in `repository/interfaces.go`.
  Implementations live in `postgres/` or `memory/`.
- **Accept Interfaces, Return Structs**: Functions accept
  interface parameters and return concrete types. This
  enables testability and follows Go convention.
- **Context Propagation**: Pass `context.Context` as the
  first parameter to all service and repository methods.
  Use it for cancellation, timeouts, and request-scoped
  values (logger, tracing).

## Adding a Feature

1. Define domain entities/errors in `internal/domain/`
2. Add repository interface in
   `repository/interfaces.go`, implement in `postgres/`
   or `memory/`
3. Add service interface in `service/interfaces.go`,
   implement service
4. Run `make generate` to regenerate mocks
5. Add handler in `internal/handler/`, register routes
   in `routes.go`
6. Wire in `internal/app/app.go`
7. Add migration in `migrations/` if needed (Goose
   format, sequential numbering)
8. Write tests

## Testing

- Place unit tests in `*_test.go` alongside source files
- Run tests: `make test` (verbose: `make test V=1`)
- Regenerate mocks after interface changes:
  `make generate`
- Use table-driven tests as the default pattern for
  unit tests
- Integration test flow via `test/example-sp/`

### Table-Driven Tests

```go
func TestSessionService_Get(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        setup   func(repo *mocks.MockSessionRepository)
        want    *domain.Session
        wantErr error
    }{
        {
            name: "existing session",
            id:   "abc-123",
            setup: func(repo *mocks.MockSessionRepository) {
                repo.EXPECT().Get(gomock.Any(), "abc-123").Return(&domain.Session{ID: "abc-123"}, nil)
            },
            want: &domain.Session{ID: "abc-123"},
        },
        {
            name: "not found",
            id:   "missing",
            setup: func(repo *mocks.MockSessionRepository) {
                repo.EXPECT().Get(gomock.Any(), "missing").Return(nil, errors.New("not found"))
            },
            wantErr: domain.ErrNotFound("session", "missing"),
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            repo := mocks.NewMockSessionRepository(ctrl)
            tt.setup(repo)

            svc := service.NewSessionService(repo, logging.NewNoop())
            got, err := svc.Get(context.Background(), tt.id)

            if tt.wantErr != nil {
                assert.Equal(t, tt.wantErr, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Integration Testing Flow

1. Start services: `make dev`
2. Run provider: `make run`
3. In another terminal:
   `cd test/example-sp && make register && make run`
4. Access example service at
   `https://localhost:8083/hello`
5. Verify: Service → SAML provider → Hydra login →
   user data flows back

## Development Setup

```bash
make dev  # Start Docker containers
make run  # Run migrations and start the SAML provider
```

All commands are defined in the root `Makefile`. Update
the `Makefile` when adding new commands or changing
existing ones.

## Validation and Verification

| Command | Purpose |
| ------- | ------- |
| `make build` | Build binary |
| `make test` | Run unit tests |
| `make lint` | Run golangci-lint |
| `make fmt` | Format source |
| `make generate` | Regenerate mocks |
| `make license-check` | Verify all Go files have license headers |
| `make license-add` | Add missing license headers (current year) |
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Roll back last migration |
| `make certs` | Generate local dev certificates |
| `make clean` | Remove build artifacts |
