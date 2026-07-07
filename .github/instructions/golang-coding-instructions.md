---
description: Golang-specific coding standards.
applyTo:
  - "**/*.go"
---

# Go Coding Standards & Patterns

This instruction file governs all Go source code files (`*.go`) in this
repository.

## 1. Style & Precedence

All Go code must adhere to the following standards, in order of precedence:

1. Project-specific conventions defined in this file.
1. Local Canonical Go Style Guide
   (See: [../agents/go-style-guide.md](../agents/go-style-guide.md)).
1. Effective Go conventions.

---

## 2. License Header

Every new Go file (excluding `mocks/` and `vendor/`) must start with this exact
header as the first lines of the file, followed by a single blank line:

```go
// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only
```

*(Note: Do not update the year when modifying existing files).*

---

## 3. Context & Resource Handling

- **Context Parameter:** Always pass `context.Context` as the first parameter to
  service/repository methods.
- **Context Storage:** *Never* store context inside a struct field.

---

## 4. Error Handling

Services return typed domain errors; handlers map them to HTTP statuses. Do not
wrap or obscure domain errors.

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

---

## 5. Logging & Context

Use the structured `logging.Logger` interface. Extract request-scoped loggers
from context.

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

---

## 6. Dependency Injection

Use constructor injection of interfaces exclusively. Functions accept
interfaces, return concrete structs.

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

---

## 7. Mocks

- Generated via `go.uber.org/mock/mockgen`.
- Add the `//go:generate` directive above interface declarations.
- Run `make generate` to recreate. Never edit files in `mocks/` manually.
