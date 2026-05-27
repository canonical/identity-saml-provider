# ADR 004: Hydra OIDC Client Redesign

## Status

Proposed

## Context

The Identity SAML Provider communicates with Ory Hydra as
an OIDC Relying Party via the authorization code flow.
The current implementation has several design issues that
affect testability, layering, and correctness.

### Problem 1: Application-level CA certificate handling

The `hydra` infrastructure package builds a custom
`*http.Client` with application-level TLS configuration:

```go
// internal/infrastructure/hydra/client.go

func NewClient(cfg Config) (*http.Client, error) {
    // Read custom CA cert from file
    // Append to system cert pool
    // Set InsecureSkipVerify flag
    // Return custom *http.Client with modified TLS transport
}
```

This is controlled by two environment variables:

- `SAML_PROVIDER_HYDRA_CA_CERT_PATH` — path to a custom
  CA certificate PEM file
- `SAML_PROVIDER_HYDRA_INSECURE_SKIP_TLS_VERIFY` —
  disables TLS verification entirely

Normally, applications do not manage CA certificates in
code. Go's `net/http` and `crypto/tls` automatically use
the system trust store (`/etc/ssl/certs/` on Linux). The
standard practice is to install custom CA certificates
into the container's system trust store at build time or
via an init container, making TLS transparent to the
application.

The `InsecureSkipVerify` flag is a security anti-pattern.
Even with a warning log, exposing a "disable TLS
verification" option in production-reachable configuration
is a footgun.

### Problem 2: The Hydra client is not interface-driven

The `hydra` package exposes two free functions:

```go
func NewClient(cfg Config) (*http.Client, error)
func DiscoverOIDC(ctx, httpClient, cfg, oidcCfg) (*DiscoveryResult, error)
```

These return concrete types (`*http.Client`,
`*oauth2.Config`, `*oidc.IDTokenVerifier`). There is no
interface representing the Hydra/OIDC integration as a
whole. This makes it impossible to swap or mock the
infrastructure boundary in integration tests without
spinning up an `httptest.Server`.

### Problem 3: Token exchange lives in the service layer

The `oidcService` in the service layer directly depends
on concrete third-party types:

```go
type oidcService struct {
    oauth2Config *oauth2.Config       // from golang.org/x/oauth2
    verifier     OIDCTokenVerifier     // wraps coreos/go-oidc
    logger       logging.Logger
}
```

The `ExchangeCode` method performs token exchange via
`s.oauth2Config.Exchange(ctx, code)`, extracts the raw
ID token, verifies it, and unmarshals claims. This is
infrastructure orchestration — calling two third-party
libraries — not business logic. The service layer should
not import `golang.org/x/oauth2`.

This also causes a correctness issue: the custom HTTP
client (with CA cert configuration) is only injected into
the context during OIDC discovery at startup. During
runtime token exchange, `oauth2.Config.Exchange()` uses
`http.DefaultClient` because no custom client is in the
context. If Hydra uses a custom CA, discovery succeeds
but every token exchange fails.

### Problem 4: Adapter boilerplate for testability

Because the verifier is a third-party concrete type
(`*oidc.IDTokenVerifier`), the service layer defines two
interfaces (`OIDCTokenVerifier`, `OIDCIDToken`) and an
adapter (`idTokenVerifierAdapter`) solely to make token
verification mockable. If token exchange and verification
lived in the infrastructure layer, these interfaces would
be unnecessary in the service package.

## Decision

### 1. Remove application-level CA certificate handling

> **Note**: This decision is partially superseded by
> [ADR 010](010-hydra-custom-ca-certificate.md), which
> reintroduces application-level custom CA support using
> an isolated certificate pool. `InsecureSkipVerify`
> remains removed.

Remove the `CACertPath` and `InsecureSkipTLSVerify`
configuration fields and all associated TLS code from the
`hydra` package. The application will rely on Go's default
system cert pool for all outbound HTTPS connections.

Custom CA certificates should be installed into the
container's system trust store:

- **Rock (rockcraft)**: Add the CA cert to
  `/usr/local/share/ca-certificates/` and run
  `update-ca-certificates` during the build.
- **Kubernetes**: Use an init container to copy the CA
  cert into a shared volume, or use a custom base image.
- **Local dev**: Hydra runs on plain HTTP
  (`http://hydra:4444`) — no TLS is involved.

Affected environment variables to remove:

- `SAML_PROVIDER_HYDRA_CA_CERT_PATH`
- `SAML_PROVIDER_HYDRA_INSECURE_SKIP_TLS_VERIFY`

### 2. Move token exchange into the infrastructure layer

Move the OAuth2 token exchange and ID token verification
from `internal/service/oidc.go` into the
`internal/infrastructure/hydra/` package. The `hydra`
package will own all interactions with the
`golang.org/x/oauth2` and `coreos/go-oidc/v3` libraries.

The hydra client is responsible for **infrastructure
concerns only**:

- Exchanging an authorization code for an OAuth2 token
  (`oauth2Config.Exchange`)
- Extracting and verifying the raw ID token
  (`verifier.Verify`)
- Mapping the verified `*oidc.IDToken` into a domain
  type (`*domain.IDToken`) that preserves structured
  token metadata alongside the raw claims

The hydra client is **not** responsible for:

- Deciding which custom claims are relevant (email,
  name, groups)
- Structuring claims into service-layer types
  (`*OIDCClaims`)
- Per-SP attribute mapping

Those remain in the service layer, which interprets the
ID token according to business rules.

#### Domain ID token type

A new domain type represents the verified ID token. It
preserves the structured metadata that `*oidc.IDToken`
exposes as exported fields (`Issuer`, `Subject`,
`Expiry`, etc.) alongside the full raw claims map:

```go
// internal/domain/oidc.go

type IDToken struct {
    Issuer   string
    Subject  string
    Expiry   time.Time
    IssuedAt time.Time
    Claims   map[string]interface{}
}
```

This avoids two problems with returning a raw
`map[string]interface{}`:

- **Type safety**: `raw["sub"].(string)` can panic or
  silently produce a zero value if the claim is missing
  or has an unexpected type. A struct with
  `Subject string` is safer.
- **Lost structure**: An ID token is a well-defined OIDC
  concept with standard fields. Flattening it into a map
  erases that meaning.

#### Client interface defined at the consumer

Following Go convention, the interface is defined where
it is consumed — in the service layer — not in the
`hydra` package. This avoids coupling the infrastructure
layer to service-layer types and prevents circular
imports:

```go
// internal/service/interfaces.go

// HydraClient abstracts the Hydra OIDC infrastructure
// for token exchange and auth URL generation.
type HydraClient interface {
    AuthCodeURL(state string) string
    ExchangeCode(ctx context.Context, code string) (*domain.IDToken, error)
}
```

`ExchangeCode` returns `*domain.IDToken` — a domain
type that both layers can depend on without introducing
circular imports.

#### Infrastructure implementation

The `hydra` package exposes a `Client` struct. The name
`Client` is chosen because the package is `hydra`, so
consumers reference it as `hydra.Client` — which reads
naturally and follows Go conventions (e.g.,
`http.Client`, `redis.Client`):

```go
// internal/infrastructure/hydra/client.go

type Client struct {
    oauth2Config *oauth2.Config
    verifier     *oidc.IDTokenVerifier
    httpClient   *http.Client
    logger       logging.Logger
}

func NewClient(
    ctx context.Context,
    cfg Config,
    oidcCfg OIDCConfig,
    logger logging.Logger,
) (*Client, error) {
    // Build *http.Client (simple timeout, no custom TLS)
    // Perform OIDC discovery
    // Return fully initialised *Client
}

func (c *Client) AuthCodeURL(state string) string {
    return c.oauth2Config.AuthCodeURL(state)
}

func (c *Client) ExchangeCode(
    ctx context.Context, code string,
) (*domain.IDToken, error) {
    // Inject httpClient into context for oauth2 library
    if c.httpClient != nil {
        ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
    }
    // Exchange authorization code for OAuth2 token
    // Extract raw ID token string
    // Verify ID token signature and claims
    // Extract raw claims into map[string]interface{}
    // Return *domain.IDToken with structured fields + raw claims
}
```

`*Client` satisfies `service.HydraClient` implicitly via
Go structural typing — no explicit declaration needed.

#### Service layer

The `OIDCService` interface and `oidcService` struct
remain in the service layer. The service now depends on
`HydraClient` instead of `*oauth2.Config` and
`OIDCTokenVerifier`:

```go
// internal/service/oidc.go

type oidcService struct {
    hydra  HydraClient
    logger logging.Logger
}

func (s *oidcService) ExchangeCode(
    ctx context.Context, code string,
) (*OIDCClaims, error) {
    // Delegate infrastructure work to hydra client
    idToken, err := s.hydra.ExchangeCode(ctx, code)
    // Map idToken into *OIDCClaims:
    //   Sub = idToken.Subject (direct field access)
    //   Email, Name, Groups extracted from idToken.Claims
    //   RawClaims = idToken.Claims
    // Return domain-typed result
}
```

Key properties of this design:

- The `Client` holds the `*http.Client` and injects
  it into the context on every call to `Exchange()`,
  fixing the bug where the custom client was only used
  during discovery.
- All third-party library imports (`oauth2`, `go-oidc`)
  are confined to the infrastructure layer.
- The service layer consumes the `HydraClient` interface
  and never touches HTTP clients, OAuth2 configs, or
  token verifiers directly.
- Claims interpretation (which fields matter, how to
  structure them) stays in the service layer as business
  logic.
- Token verification is retained within the
  infrastructure layer as defense-in-depth, even though
  the token is received via a direct server-to-server
  TLS channel.

### 3. Remove adapter boilerplate from the service layer

With token verification moved to the infrastructure
layer, the following types become unnecessary in the
service package and should be removed:

- `OIDCTokenVerifier` interface
- `OIDCIDToken` interface
- `idTokenVerifierAdapter` struct
- `verifier_adapter.go` file

The mock for `OIDCService` (already generated) remains
the only test double needed for the handler layer.

### 4. Simplify app wiring

The `app.Build()` function currently performs three steps
for OIDC setup:

```go
hydraClient, err := hydra.NewClient(cfg.HydraConfig())
discovery, err := hydra.DiscoverOIDC(ctx, hydraClient, ...)
oidcSvc := service.NewOIDCService(
    discovery.OAuth2Config,
    service.NewIDTokenVerifierAdapter(discovery.Verifier),
    logger,
)
```

After the redesign, this simplifies to:

```go
hydraClient, err := hydra.NewClient(ctx, cfg.HydraConfig(), cfg.OIDCConfig(), logger)
oidcSvc := service.NewOIDCService(hydraClient, logger)
```

The `hydra.Client` constructor performs OIDC discovery
internally and returns a ready-to-use `*hydra.Client`
that satisfies `service.HydraClient`. The OIDC service
wraps it with business logic for claims interpretation.

## Consequences

### Benefits

- **Correct TLS handling**: Removing application-level CA
  cert code eliminates a class of configuration errors.
  TLS trust is managed at the infrastructure level where
  it belongs.
- **Security improvement**: Removing the
  `InsecureSkipVerify` flag eliminates a production
  security risk.
- **Proper layering**: The service layer contains only
  business logic and depends on interfaces. All
  third-party OIDC/OAuth2 library usage is confined to
  the infrastructure layer.
- **Bug fix**: The custom HTTP client is consistently
  used for both OIDC discovery and token exchange,
  fixing the current inconsistency.
- **Less boilerplate**: Two interfaces (`OIDCTokenVerifier`,
  `OIDCIDToken`), the adapter struct, and its file are
  replaced by a single `HydraClient` interface. The app
  wiring is shorter.
- **Testability preserved**: The handler layer mocks
  `OIDCService` as before. The infrastructure layer can
  be tested with `httptest.Server` for the token
  endpoint, which is the existing pattern.

### Drawbacks

- **Operational change**: Operators who currently use
  `SAML_PROVIDER_HYDRA_CA_CERT_PATH` must instead
  install the CA certificate into the container's system
  trust store. This requires changes to the container
  build process or Kubernetes manifests.
- **Lost escape hatch**: Removing `InsecureSkipVerify`
  means developers cannot quickly bypass TLS issues
  during debugging. The alternative is to use plain HTTP
  for local development (which is already the default)
  or install the CA cert into the system trust store.
- **Infrastructure package grows**: The `hydra` package
  takes on more responsibility (token exchange, claim
  extraction). This is appropriate for an infrastructure
  package but increases its size.
