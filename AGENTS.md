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
- ⚠️ **Configuration:** All config must look for the
  environment variable prefix `SAML_PROVIDER_`.
- 🚫 **No CHANGELOG.md Edits:** Agents must *NEVER*
  edit the `CHANGELOG.md` file. It is managed
  by Google's `release-please` automation.

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

## 3. External References

| Need                           | File                                                                                |
|:-------------------------------|:------------------------------------------------------------------------------------|
| Go style & coding patterns     | [golang-coding-instructions.md](.github/instructions/golang-coding-instructions.md) |
| Local Canonical Go style guide | [go-style-guide.md](.github/agents/go-style-guide.md)                               |
| Authentication Flow            | [docs/authentication-flow/](docs/authentication-flow/)                              |

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

| Command              | Purpose                      |
|----------------------|------------------------------|
| `make build`         | Build binary                 |
| `make test`          | Run table-driven unit tests  |
| `make lint`          | Run golangci-lint            |
| `make fmt`           | Format source code           |
| `make generate`      | Regenerate Go mocks          |
| `make license-check` | Verify AGPL-3.0-only headers |
| `make migrate-up`    | Apply Goose DB migrations    |
| `make migrate-down`  | Roll back last DB migration  |
