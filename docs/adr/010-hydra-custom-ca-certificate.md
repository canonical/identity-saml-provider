# ADR 010: Hydra Custom CA Certificate Support

## Status

Proposed

## Context

ADR 004 removed application-level CA certificate handling
from the Hydra client, relying entirely on the system
trust store for HTTPS connections. While this simplified
the codebase, it limits deployment flexibility and does
not fully address the security goal of minimising the
trusted certificate authority surface.

### Problem 1: Deployment inflexibility

The system trust store approach requires installing
custom CA certificates into the container image at build
time or via an init container at runtime. This works for
some deployment models but creates friction or outright
breaks in others:

- **Rock (distroless-like images)**: The filesystem is
  read-only and no shell is available. Running
  `update-ca-certificates` at runtime is not possible.
  The CA must be baked in at build time, coupling the
  image to a specific CA.
- **Juju charms**: Charm relation data may provide a CA
  certificate dynamically. Writing it to the system
  trust store requires elevated privileges and a
  writable filesystem.
- **Pure Docker**: Operators must either rebuild the
  image or use bind mounts into `/usr/local/share/
  ca-certificates/` plus an entrypoint script that runs
  `update-ca-certificates` — adding operational
  complexity.

The application should run in any deployment model
(pure Kubernetes, Juju charm, Docker container, bare
metal) without requiring modifications to the system
trust store.

### Problem 2: Excessive trust surface

A typical Linux system trust store contains over 100
root CA certificates. The SAML provider communicates
with exactly one external endpoint: Ory Hydra. Trusting
over 100 CAs for a single outbound connection violates
the principle of least privilege. Any of those CAs
could, in theory, issue a certificate for Hydra's
hostname that the application would accept.

While the practical risk is bounded by Certificate
Transparency and CA/Browser Forum rules, a
security-sensitive identity bridge should minimise its
trust surface where possible.

### Problem 3: No `InsecureSkipVerify` alternative

ADR 004 correctly removed the `InsecureSkipVerify`
flag. However, without a custom CA path, operators
deploying Hydra behind a private CA (internal PKI,
Vault-issued, self-signed) have no supported path to
establish TLS trust. The only workarounds are modifying
the system trust store (see Problem 1) or disabling
TLS entirely by using `http://` — both undesirable in
production.

## Options Considered

### Option A: Isolated pool only

When `CACertPath` is provided, create a fresh
`*x509.CertPool` containing only the specified CA.
The system pool is not consulted.

```go
pool := x509.NewCertPool()
pool.AppendCertsFromPEM(caCert)
transport := &http.Transport{
    TLSClientConfig: &tls.Config{RootCAs: pool},
}
```

**Pros:**

- Minimal trust surface — only the specified CA is
  trusted
- No dependency on the host's system trust store
- Clear operator intent: "trust this CA and nothing
  else"

**Cons:**

- If Hydra sits behind a publicly-trusted CA, the
  operator must explicitly provide that public CA's
  root PEM — additional operational toil
- CA rotation becomes the operator's responsibility
  (no automatic updates from the system store)

### Option B: System pool copy plus append

When `CACertPath` is provided, call
`x509.SystemCertPool()` to obtain a read-only
in-process copy of the system trust store, then append
the custom CA to that copy.

```go
pool, _ := x509.SystemCertPool()
pool.AppendCertsFromPEM(caCert)
transport := &http.Transport{
    TLSClientConfig: &tls.Config{RootCAs: pool},
}
```

**Pros:**

- Trusts both the custom CA and all system-trusted CAs
- No modification to the on-disk system store
- Public CAs work without operator intervention

**Cons:**

- The trust surface includes 100+ CAs the application
  does not need for its single Hydra connection
- In distroless images, the system pool may be empty
  or minimal, making behavior unpredictable
- Contradicts the goal of reducing the attack vector

### Option C: CA cert via environment variable

Accept the CA certificate PEM as a base64-encoded
environment variable instead of a file path.

**Pros:**

- No filesystem access needed — ideal for distroless
  images and Kubernetes Secrets as env vars
- Works in every deployment model

**Cons:**

- Large PEM values in environment variables are
  ergonomically awkward
- Can be combined with either Option A or Option B
  for the pool strategy (orthogonal concern)
- Does not address the trust surface question

### Option D: Isolated pool when CA provided, system pool when not

When `CACertPath` is set, use Option A (isolated pool
with only the specified CA). When `CACertPath` is not
set, fall back to Go's default behavior (system trust
store).

**Pros:**

- Combines the security benefit of Option A with the
  convenience of the default path
- Single configuration value controls the behavior —
  no extra boolean flag
- Operator intent is unambiguous: providing a CA means
  "trust only this CA"
- Not providing a CA means "use the platform default"
- Works in every deployment model

**Cons:**

- Operators using a publicly-trusted CA on Hydra who
  also want to provide an explicit CA file will only
  trust that one CA, not the full system pool. This is
  intentional and desirable for this application's
  threat model, but may surprise operators unfamiliar
  with the design.

## Decision

Adopt **Option D**: isolated pool when a CA is
provided, system pool fallback when not.

### TLS transport behaviour

The issuer URL scheme determines whether TLS is
configured. The `CACertPath` value determines which
certificate pool is used. Together they produce four
distinct behaviours:

| `CACertPath` | Issuer URL | TLS behaviour |
| --- | --- | --- |
| unset | `http://` | No TLS (plain HTTP) |
| unset | `https://` | System pool (Go default) |
| set | `http://` | CA ignored, no TLS (log warning) |
| set | `https://` | Isolated pool (only provided CA) |

Using the URL scheme as the toggle is idiomatic:

- The deployer already controls `HYDRA_PUBLIC_URL` as
  the single source of truth for the Hydra endpoint.
- No separate `TLS_ENABLED` boolean is needed,
  eliminating conflicting configuration states (e.g.,
  `TLS_ENABLED=true` with an `http://` URL).
- Go's `net/http` natively handles the scheme
  distinction — `http://` means no TLS handshake,
  `https://` means TLS is required.

### Configuration

A single new environment variable:

- `SAML_PROVIDER_HYDRA_CA_CERT_PATH` — path to a PEM
  file containing the CA certificate(s) to trust for
  Hydra HTTPS connections.

When set and the issuer URL uses `https://`:

1. Read the PEM file at startup.
2. Create an isolated `*x509.CertPool` via
   `x509.NewCertPool()`.
3. Append the PEM contents to the pool.
4. Build a `*http.Transport` with
   `tls.Config{RootCAs: pool}`.
5. Wrap with `otelhttp.NewTransport` for tracing.
6. Use this transport for the Hydra `*http.Client`.

When unset, the existing behaviour is preserved: Go's
default transport uses the system certificate pool.

### Startup validation

The following checks run during `NewClient`
initialisation, before OIDC discovery. The issuer URL
scheme is determined by parsing the URL with
`net/url.Parse`, not by string prefix matching:

- If the issuer URL scheme is not `http` or `https`,
  fail fast with a clear error.
- If `CACertPath` is set and the issuer URL uses
  `https://`, verify the file exists and is readable.
  Fail fast with a clear error if not.
- If `CACertPath` is set and the issuer URL uses
  `https://`, verify the file contains at least one
  valid PEM block. Fail fast if the file is empty or
  contains no parseable certificates.
- If `CACertPath` is set but the issuer URL uses
  `http://`, log a warning that the CA certificate
  will not be used because the connection is plain
  HTTP. Do not validate the file in this case.

### Startup logging

Log the active TLS mode at startup to make the
configuration auditable:

- `"hydra TLS: custom CA from /path/to/ca.pem
  (isolated pool)"` — when CA is provided and issuer
  is HTTPS.
- `"hydra TLS: system certificate pool"` — when no CA
  is provided and issuer is HTTPS.
- `"hydra transport: plain HTTP (no TLS)"` — when
  issuer is HTTP.

### Implementation scope

Changes are confined to three packages:

- **`internal/infrastructure/hydra/`**: Add
  `CACertPath` to `Config`. In `NewClient`, build a
  custom TLS transport when the field is set and the
  issuer scheme is `https://`. Add validation and
  logging.
- **`internal/app/config.go`**: Add the
  `SAML_PROVIDER_HYDRA_CA_CERT_PATH` field to
  `Config`. Wire it into `HydraConfig()`. Validate
  that `HydraPublicURL` uses `http` or `https`
  scheme in `Validate()`. CA file validation belongs
  to `hydra.NewClient`, not `Validate()`.
- **`internal/infrastructure/hydra/client_test.go`**:
  Add tests for the new TLS transport paths.

No changes to the service layer, handler layer, or
domain layer. The custom transport is scoped to the
Hydra `*http.Client` only — no other outbound
connections are affected.

### Relationship to ADR 004

This ADR supersedes the CA certificate portion of
ADR 004 (Decision 1: "Remove application-level CA
certificate handling"). The remaining decisions in
ADR 004 (interface-driven client, token exchange in
infrastructure layer, adapter removal, simplified
wiring) are unaffected and remain in force.

The key differences from the pre-ADR-004
implementation:

- **No `InsecureSkipVerify`**: Not reintroduced. There
  is no configuration option to bypass TLS
  verification.
- **Isolated pool, not system pool append**: The
  previous implementation used
  `x509.SystemCertPool()` plus append, trusting the
  custom CA in addition to all system CAs. This ADR
  uses `x509.NewCertPool()`, trusting only the
  provided CA.
- **Scheme-gated**: The TLS transport is only built
  when the issuer URL uses `https://`. The previous
  implementation always built a custom transport.

## Consequences

### Benefits

- **Deployment flexibility**: The application runs in
  any environment (Kubernetes, Juju, Docker, Rock,
  bare metal) without requiring system trust store
  modifications.
- **Reduced attack surface**: When a custom CA is
  provided, only that CA is trusted — not the 100+
  CAs in the system pool.
- **No `InsecureSkipVerify`**: The security
  improvement from ADR 004 is preserved.
- **Fail-fast validation**: Invalid or missing CA
  files produce clear errors at startup, not cryptic
  TLS handshake failures at runtime.
- **Auditable configuration**: Startup logs clearly
  indicate which TLS mode is active.
- **Minimal code change**: The change is confined to
  the infrastructure layer and configuration. No
  service or domain layer changes.

### Drawbacks

- **Operator responsibility for CA management**: When
  a custom CA is provided, the operator must update
  `CACertPath` when the CA rotates. This is standard
  for private PKI deployments.
- **Public CA requires explicit provision**: If Hydra
  uses a publicly-trusted CA and the operator sets
  `CACertPath`, they must provide that public CA's
  root PEM. Omitting `CACertPath` entirely and
  relying on the system pool is the simpler path for
  public CAs.
- **Partial ADR 004 supersession**: This ADR reverses
  one specific decision from ADR 004 while keeping
  the rest. The interaction between the two ADRs must
  be understood together.
