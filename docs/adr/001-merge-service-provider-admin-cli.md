# ADR 001: Merge Service Provider Admin CLI into Main Binary

## Status

Accepted

## Context

The project currently ships two separate CLI binaries:

- **`identity-saml-provider`**
  (`cmd/identity-saml-provider/`) — the main binary with
  `serve`, `migrate`, and `version` subcommands, wired
  through `internal/cmd/`.
- **`service-provider-admin`**
  (`cmd/service-provider-admin/`) — a standalone binary
  for registering SAML service providers via the admin
  HTTP API.

The admin CLI was introduced to provide an
operator-friendly interface for registering service
providers. However, several design issues have been
identified:

1. **Separate binary increases distribution complexity.**
   Two binaries must be built, versioned, and packaged
   (e.g., into the rock/container image) independently.

2. **HTTP-client architecture is fragile.** The admin CLI
   sends HTTP requests to the running server's
   `/admin/service-providers` endpoint. This means:
   - The server must already be running before an SP can
     be registered (a chicken-and-egg problem during
     bootstrap).
   - Network errors, TLS configuration, and firewall
     rules add failure modes.
   - Error messages are opaque HTTP response strings
     rather than typed domain errors.

3. **No code reuse with the main binary.** The admin CLI
   builds its own cobra command tree, flag handling, and
   output formatting from scratch. It uses package-level
   global variables for flags, writes directly to
   `fmt.Printf` (not `io.Writer`), and has no tests.

4. **Inconsistent output formatting.** The main binary's
   `migrate` command uses a strategy-based
   `MigrateOutputFormatter` interface with
   `--format text|json` and writes to `io.Writer`. The
   admin CLI uses ad-hoc `if outputFormat == "json"`
   branching with `--output human|json` — different flag
   name, different format values, different output style
   (pretty-printed vs compact JSON).

5. **Limited scope.** The admin CLI only supports a
   single `add` command with no list, get, update, or
   delete operations, offering marginal value over a
   direct `curl` call.

## Decision

We will make the following changes:

### 1. Merge the admin CLI as a subcommand of the main binary

The service provider management commands will be added
to `identity-saml-provider` under a new `sp` subcommand
group:

```text
identity-saml-provider sp add    --entity-id ... --acs-url ...
identity-saml-provider sp list   (future)
identity-saml-provider sp delete (future)
```

This follows the existing pattern established by
`migrate up`, `migrate down`, `migrate status`, and
`migrate check`. The `sp` noun sits alongside `migrate`
and `serve` at the top level — consistent depth and
style.

We chose `sp` over `admin sp` because:

- Every subcommand in this binary is an admin operation
  (`serve`, `migrate`, `sp`). Grouping only SP commands
  under `admin` creates a false distinction.
- `sp add` is two levels deep, matching `migrate up`.
  `admin sp add` would be three levels — inconsistent.
- A generic `admin` parent adds no information and
  forces future resource types (sessions, certs) into
  an unnecessary umbrella.

The resulting command tree is:

```text
identity-saml-provider
├── serve
├── migrate
│   ├── up
│   ├── down
│   ├── status
│   └── check
├── sp
│   ├── add
│   ├── list     (future)
│   └── delete   (future)
└── version
```

The `cmd/service-provider-admin/` directory and its
separate binary will be removed.

### 2. Use direct service-layer calls instead of HTTP

The `sp` subcommand will connect directly to the database
and call the service layer:

```go
var cfg app.Config
envconfig.MustProcess("", &cfg)
dsn := cfg.DatabaseDSN()

pool := openDB(dsn)
spRepo := postgres.NewServiceProviderRepo(pool)
spService := service.NewServiceProviderService(spRepo, logger)
err := spService.Register(ctx, &domain.ServiceProvider{...})
```

This eliminates the HTTP client code, removes the runtime
dependency on a running server, and provides typed domain
errors.

#### Database configuration via environment variables

The `sp` subcommand will read database connection details
from the same `SAML_PROVIDER_DB_*` environment variables
that `serve` already uses (via `app.Config` and
`envconfig`), rather than requiring a `--dsn` flag.

This approach was chosen over a raw `--dsn` flag because:

- **Already set in production.** Operators deploying via
  k8s/rock already have `SAML_PROVIDER_DB_*` configured
  for the `serve` command. No extra configuration needed.
- **No secrets on the command line.** A `--dsn` flag
  exposes the database password in `ps` output and shell
  history. Environment variables are the standard way to
  pass secrets to processes.
- **Consistent with `serve`.** Both `serve` and `sp` read
  the same env vars — one source of truth for database
  connectivity.
- **`Config.DatabaseDSN()` already exists.** The DSN
  construction logic is already implemented and tested.

Note: the existing `migrate --dsn` flag predates this
decision and should be considered for the same env-var
migration in a future change for consistency.

### 3. Reuse and extend the output formatter pattern

The existing `MigrateOutputFormatter` interface will be
generalized into a broader `OutputFormatter` interface
(or the admin commands will define their own formatter
following the same strategy pattern) with consistent
conventions:

- Flag: `--format` with values `text` or `json`
  (not `--output` with `human`/`json`)
- Output written to `io.Writer` (via
  `cmd.OutOrStdout()`) for testability
- Text format: human-readable messages
- JSON format: structured, machine-parseable output via
  `json.NewEncoder`

This ensures all CLI subcommands (`migrate`, `sp`) share
a consistent output contract.

## Consequences

### Benefits

- **Single binary to build, ship, and version.**
  Simplifies CI/CD, rock packaging, and operational
  documentation.
- **Bootstrapping works.** Operators can register SPs
  before the server is running, as long as the database
  is reachable — useful for initial setup and
  infrastructure-as-code workflows.
- **Code reuse.** The `sp` subcommand reuses the existing
  cobra tree, flag conventions, DB connection via
  `Config.DatabaseDSN()`, formatter strategy, and
  `internal/cmd` package structure.
- **Testable.** Output written to `io.Writer` can be
  captured in unit tests. Direct service calls can be
  tested with mock repositories.
- **Typed errors.** Domain errors (e.g.,
  `ErrDuplicateEntityID`) propagate directly instead
  of being lost in HTTP response string parsing.
- **Consistent UX.** All subcommands use the same
  `--format text|json` flag and output contract.

### Drawbacks

- **Operator needs DB credentials.** The `sp` subcommand
  requires `SAML_PROVIDER_DB_*` environment variables to
  be set. In practice, operators who can register SPs
  already have DB access, and these env vars are
  typically already configured for the `serve` command.
- **No remote management via CLI.** Operators can no
  longer point the CLI at a remote server URL to register
  SPs. If remote management is needed, the admin HTTP API
  remains available and can be called directly with
  `curl` or similar tools.
- **Migration effort.** Existing documentation and
  scripts (e.g., `test/saml-service/Makefile`) that
  reference `service-provider-admin` must be updated.
