# Software Design Patterns, Principles, and Go Idioms

> [!NOTE]
> This document serves as the official reference for the PR-Agent `/improve`
> and `/review` tools to guide AI-generated code suggestions and reviews.

This document outlines the high-level design patterns, software engineering
principles, and idiomatic Go practices to follow when designing, refactoring, or
reviewing code in this repository.

---

## 1. Design Patterns

### 1.1 Creational: Constructor Functions (Factory Pattern)

Constructors should be used consistently to enforce invariant validation upon
initialization, initialize internal state (e.g., slices, maps, channels), and
return a ready-to-use struct or interface.

#### Un-idiomatic Creational Pattern

```go
// Direct initialization allows invalid or uninitialized state (e.g., nil logger/nil map)
service := &SessionService{
    repo: postgresRepo,
}
```

#### Idiomatic Creational Pattern

```go
// Constructor enforces correct interface dependencies, default values, and validates state
func NewSessionService(
    repo repository.SessionRepository,
    logger logging.Logger,
) (*SessionService, error) {
    if repo == nil {
        return nil, fmt.Errorf("repository cannot be nil")
    }
    if logger == nil {
        return nil, fmt.Errorf("logger cannot be nil")
    }
    return &SessionService{
        repo:   repo,
        logger: logger,
    }, nil
}
```

### 1.2 Structural: Decorator / Middleware Pattern

Cross-cutting concerns (e.g., telemetry, logging, caching, authorization,
tracing) must be decoupled from the core business logic of Services using
decorators or HTTP middlewares.

#### Un-idiomatic Structural Pattern

```go
// Core service logic is cluttered with caching, logging, and metrics telemetry
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, error) {
    s.logger.Debugw("fetching session", "id", id)
    s.metrics.IncSessionLookups()

    session, err := s.repo.Get(ctx, id)
    if err != nil {
        s.metrics.IncSessionLookupFailures()
        return nil, err
    }
    return session, nil
}
```

#### Idiomatic Structural Pattern

```go
// Core business service is strictly focused on orchestration
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, error) {
    return s.repo.Get(ctx, id)
}

// LoggingDecorator wraps the service and adds logging/telemetry transparently
type LoggingSessionServiceDecorator struct {
    next   service.SessionService
    logger logging.Logger
}

func (d *LoggingSessionServiceDecorator) Get(ctx context.Context, id string) (*domain.Session, error) {
    d.logger.Debugw("fetching session", "id", id)
    return d.next.Get(ctx, id)
}
```

### 1.3 Behavioral: Strategy (Adapter) Pattern

The domain core must remain decoupled from specific external data contracts
(e.g., Hydra OIDC Claims, SAML representations). Use adapters to bridge external
structures to internal domain entities.

#### Un-idiomatic Behavioral Pattern

```go
// Domain or service layer directly knows about external third-party SDK representation
func MapOidcClaims(claims *hydrasdk.IDTokenClaims) *domain.User {
    return &domain.User{
        Email: claims.Email,
        Roles: claims.Extra["roles"].([]string),
    }
}
```

#### Idiomatic Behavioral Pattern

```go
// External mapping is isolated in an Adapter in the handler or infrastructure layer
type HydraOidcClaimsAdapter struct {
    claims *hydrasdk.IDTokenClaims
}

func (a *HydraOidcClaimsAdapter) ToDomainUser() *domain.User {
    return &domain.User{
        Email: a.claims.Email,
        Roles: a.claims.Extra["roles"].([]string),
    }
}
```

---

## 2. Software Engineering Principles

### 2.1 Single Responsibility Principle (SRP)

Keep Handlers, Services, and Repositories focused on a single layer of
responsibility:

* **Handlers**: Parse HTTP transport parameters, validate input shapes (DTOs),
  delegate to Services, and map domain errors to HTTP responses.
* **Services**: Orchestrate business rules, apply domain assertions, and return
  typed domain errors.
* **Repositories**: Handle persistence mapping and queries (e.g., SQL execution,
  in-memory maps).

### 2.2 Dependency Inversion Principle (DIP)

High-level business logic must not depend on low-level details (e.g., postgres,
memory, external HTTP clients). Both must depend on abstractions (interfaces).

* **Correct**: Handlers depend on `service.SessionService` interfaces. Services
  depend on `repository.SessionRepository` interfaces.
* **Incorrect**: Services directly import or reference concrete
  `postgres.SessionRepo` structs.

### 2.3 Interface Segregation Principle (ISP)

Define narrow, highly cohesive interfaces. "The bigger the interface, the
weaker the abstraction." Avoid monolithic interfaces that bundle unrelated
capabilities.

#### Un-idiomatic Interface Pattern

```go
// A single massive interface containing unrelated actions
type Datastore interface {
    SaveSession(ctx context.Context, s *domain.Session) error
    DeleteSession(ctx context.Context, id string) error
    GetUser(ctx context.Context, id string) (*domain.User, error)
    HealthCheck(ctx context.Context) error
}
```

#### Idiomatic Interface Pattern

```go
// Segmented, small, and highly focused interfaces
type SessionRepository interface {
    Save(ctx context.Context, s *domain.Session) error
    Delete(ctx context.Context, id string) error
}

type UserRepository interface {
    Get(ctx context.Context, id string) (*domain.User, error)
}
```

---

## 3. Go Idioms

### 3.1 Return Early (Guard Clauses)

Avoid nested `if` statements by using early returns and guard clauses. This
reduces indentation depth, keeping the "happy path" aligned to the left of the
page.

#### Nested Conditional Pattern

```go
func Process(ctx context.Context, req *Request) error {
    if req != nil {
        if req.ID != "" {
            err := s.execute(ctx, req)
            if err == nil {
                return nil
            } else {
                return err
            }
        } else {
            return ErrInvalidID
        }
    } else {
        return ErrNilRequest
    }
}
```

#### Return Early Pattern

```go
func Process(ctx context.Context, req *Request) error {
    if req == nil {
        return ErrNilRequest
    }
    if req.ID == "" {
        return ErrInvalidID
    }
    return s.execute(ctx, req)
}
```

### 3.2 Avoid Interface Pollution

Do not define interfaces where a single concrete struct is sufficient and no
dynamic dispatch or unit testing mocks are required. Define interfaces where
boundaries are crossed (e.g., Handlers to Services, Services to Repositories).

* **Rule of Thumb**: "Accept interfaces, return structs."
* **Rule of Thumb**: "Interfaces should be discovered, not designed up front."

### 3.3 Goroutine & Channel Lifecycle Ownership

When spawning goroutines, ensure their lifetime is bounded, ownership is clear,
and cleanup is deterministic to avoid goroutine leaks.

#### Unbounded Goroutine Lifecycle

```go
// Spawns a background worker without any shutdown mechanism or cancel propagation
func StartWorker() {
    go func() {
        for {
            time.Sleep(1 * time.Second)
            poll()
        }
    }()
}
```

#### Bounded Goroutine Lifecycle

```go
// Lifecycle is bounded by context cancellation
func StartWorker(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return // Safe exit when context is cancelled
            case <-ticker.C:
                poll()
            }
        }
    }()
}
```

### 3.4 Table-Driven Tests

Tests should be clear, declarative, and table-driven, isolating inputs and
expectations into clean data slices.

#### Idiomatic Table-Driven Test Pattern

```go
func TestService_Get(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        mockSetup func(*mocks.MockSessionRepository)
        wantErr   error
    }{
        {
            name: "success",
            id:   "session-1",
            mockSetup: func(repo *mocks.MockSessionRepository) {
                repo.EXPECT().Get(gomock.Any(), "session-1").Return(&domain.Session{}, nil)
            },
            wantErr: nil,
        },
        {
            name: "not found",
            id:   "session-unknown",
            mockSetup: func(repo *mocks.MockSessionRepository) {
                repo.EXPECT().Get(gomock.Any(), "session-unknown").Return(nil, domain.ErrNotFound("session", "session-unknown"))
            },
            wantErr: domain.ErrNotFound("session", "session-unknown"),
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Run test case
        })
    }
}
```
