# Contributing

## Directory Structure

This project follows the
[Standard Go Project Layout](https://github.com/golang-standards/project-layout).

### `/cmd`

Main applications for this project. The directory name for
each application should match the name of the executable
you want to have.

### `/internal`

Private application and library code. This is the code you
don't want others importing in their applications or
libraries. This layout pattern is enforced by the Go
compiler itself.

- `/internal/app` - Composition root that wires all layers
  together and starts the server
- `/internal/cmd` - CLI commands (serve, migrate, version)
- `/internal/domain` - Core business entities, value objects,
  and typed domain errors (no external dependencies)
- `/internal/handler` - HTTP handlers (controllers) that
  decode requests, delegate to services, and encode responses
- `/internal/infrastructure/hydra` - Ory Hydra HTTP client
  and OIDC provider discovery
- `/internal/infrastructure/samlkit` - SAML certificate
  loading and logger adapters
- `/internal/logging` - Logger interface for structured
  logging
- `/internal/monitoring` - Metrics and monitoring middleware
- `/internal/provider` - Legacy SAML provider implementation
  (being migrated to the new layered architecture)
- `/internal/repository` - Persistence interfaces consumed by
  the service layer
- `/internal/repository/memory` - In-memory repository
  implementations
- `/internal/repository/postgres` - PostgreSQL repository
  implementations using pgxpool and Squirrel
- `/internal/service` - Business-logic services that
  orchestrate domain operations
- `/internal/tracing` - OpenTelemetry tracing configuration
- `/internal/version` - Build version metadata

### `/mocks`

Generated mock implementations for testing. Do not edit
manually; run `make generate` to refresh.

### `/.local`

Local development artifacts that are generated and not
committed to version control. This directory is gitignored.

- `/.local/certs` - Generated SSL/TLS certificates for
  local development

### `/configs`

Configuration file templates or default configs. Put your
configuration files here that should be committed to the
repository.

### `/deployments`

IaaS, PaaS, system and container orchestration deployment
configurations and templates.

- `/deployments/docker` - Docker and docker-compose related
  configuration files

### `/test`

Additional external test apps and test data.

- `/test/saml-service` - A test SAML service provider used
  for integration testing and development

## Pre-commit Hooks

This repository uses [pre-commit](https://pre-commit.com/)
to run linters and checks automatically before each commit.

### Installation

Install the pre-commit tool, then set up both the
`pre-commit` and `commit-msg` hooks:

```shell
pip install pre-commit
pre-commit install -t pre-commit -t commit-msg
```

### Code Generation (mockgen)

This project uses
[`go.uber.org/mock/mockgen`](https://github.com/uber-go/mock)
to generate mock implementations from interfaces. Install
it once:

```shell
go install go.uber.org/mock/mockgen@latest
```

After adding or modifying `//go:generate` directives, run:

```shell
make generate
```

This invokes `go generate ./...` and refreshes all generated
files under `mocks/`.

### Running Hooks Manually

To run all hooks against every file in the repository:

```shell
pre-commit run --all-files
```

To run a specific hook:

```shell
pre-commit run <hook-id> --all-files
```

## Commits

When contributing code, please follow the
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
specification for commit messages. This is enforced by
the `conventional-pre-commit` hook at the `commit-msg`
stage.
