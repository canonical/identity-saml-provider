# ADR 009: OIDC State and Nonce Validation

## Status

Proposed

## Context

The SAML-to-OIDC bridge delegates user authentication
to Hydra via the OIDC authorization code flow. Two
security mechanisms required by the OAuth2 and OIDC
specifications are currently missing, leaving the bridge
vulnerable to two distinct but related attacks.

### Missing OAuth2 state validation (login CSRF)

In `internal/handler/saml_adapters.go`, the `state`
parameter is constructed from the SAML request ID and
relay state, then passed to `AuthCodeURL`:

```go
state := req.Request.ID
if req.RelayState != "" {
    state += ":" + req.RelayState
}
http.Redirect(w, r, a.OIDC.AuthCodeURL(state), http.StatusFound)
```

No cryptographic nonce is generated and no value is
stored to later verify the callback.

In `internal/handler/oidc_callback.go`, the returned
state is parsed but never compared against an expected
value:

```go
state := r.URL.Query().Get("state")
requestID, relayState := parseState(state)
```

The callback proceeds regardless of whether the state
matches any previously issued value. An attacker can
craft a callback URL with their own authorization code
and an arbitrary state, tricking the victim's browser
into completing the flow under the attacker's identity.
This is a login CSRF attack as described in
RFC 6749 §10.12.

### Missing OIDC nonce (ID token replay)

In `internal/infrastructure/hydra/client.go`, the
authorization URL is generated without a `nonce`
parameter:

```go
func (c *Client) AuthCodeURL(state string) string {
    return c.oauth2Config.AuthCodeURL(state)
}
```

During token exchange, the ID token is verified for
signature, issuer, audience, and expiry, but the `nonce`
claim is never checked:

```go
idToken, err := c.verifier.Verify(ctx, rawIDToken)
```

The `go-oidc` library's `Verify()` method does not
check the `nonce` automatically — the caller must
compare `idToken.Nonce` against the expected value.
Without this check, if an ID token from a previous flow
is substituted during the token-exchange-to-verification
boundary (e.g. via a compromised or malicious token
endpoint response), the bridge would accept it. The
nonce binds each ID token to the specific flow that
requested it.

### Manual library wiring

The current implementation manually stitches together
`golang.org/x/oauth2` and `github.com/coreos/go-oidc/v3`
in the Hydra client. While this is the standard Go
approach (the two libraries are designed to complement
each other), it requires the developer to opt in to each
security mechanism individually. Both the `nonce`
parameter and the state validation were omitted, which
illustrates the risk of this manual wiring.

## Proposed Solutions

All three options below address both vulnerabilities
simultaneously. They differ in where the nonce is stored
for later verification.

### Option A: HMAC-signed state (stateless, server-secret)

Generate a cryptographic HMAC over the state payload
(`nonce + request_id + relay_state + timestamp`) using a
server-side secret key. Encode the HMAC alongside the
payload in the `state` parameter. On callback, recompute
the HMAC and compare. Send the same nonce as the OIDC
`nonce` parameter and verify it against the ID token's
`nonce` claim.

- No storage required — fully stateless verification.
- Requires a persistent secret key; rotation invalidates
  in-flight states.
- Replay protection is partial — relies on a timestamp
  window rather than true single-use semantics.
- State parameter grows to accommodate the HMAC.

### Option B: Cookie-bound nonce (stateless, no secret)

Generate two independent cryptographically random values
using `crypto/rand`: one for OAuth2 state binding and one
for the OIDC nonce. Store both in a single short-lived
`HttpOnly` cookie (delimited by `:`). Embed the state
value in the `state` parameter. Send the nonce value as
the OIDC `nonce` parameter. On callback, split the
cookie, compare the state value from the cookie against
the one in the state parameter (constant-time), pass the
nonce value to the token exchange for ID token `nonce`
verification, then clear the cookie.

- No server-side storage — the browser carries the
  proof.
- No secret key to manage or rotate.
- True cryptographic independence — the value exposed
  in the URL (`state`) is entirely independent of the
  value stamped into the ID token (`nonce`). Leaking
  one does not compromise the other.
- Practical single-use behavior — the cookie is
  consumed on callback. This is browser-enforced, not
  server-authoritative; see Consequences for
  limitations.
- Cookie dependency is already a prerequisite (the
  bridge relies on `saml_session` cookies).
- Changes are confined to the handler and infrastructure
  layers.
- A new `internal/crypto` package provides nonce
  generation and comparison, importable by both layers.

### Option C: Server-side nonce via PendingRequestRepository

Generate a random nonce, store it alongside the
`PendingAuthnRequest` in the existing repository, and
embed it in the state. On callback, look up the nonce
from the repository, verify it, and delete the record.

- Server-authoritative — strongest single-use guarantee.
- Requires adding a `Nonce` field to
  `domain.PendingAuthnRequest`.
- Requires a database migration.
- Touches domain, repository, and migration layers.
- Couples nonce validation to the pending request
  lifecycle.

### Comparison

| Criteria | Option A (HMAC) | Option B (cookie) | Option C (server) |
| --- | --- | --- | --- |
| Server-side storage | None | None | Database |
| Secret management | HMAC key required | None | None |
| Single-use guarantee | No (timestamp window) | Practical (cookie consumed) | Yes (record deleted) |
| Architecture impact | Handler only | Handler + crypto + infra | Domain + repo + migration |
| New dependencies | None (stdlib) | None (stdlib) | None |
| Migration required | No | No | Yes |
| Cookie dependency | No | Yes (already exists) | No |

## Decision

Use **Option B: cookie-bound nonce** with two
independent random values — one for OAuth2 state
binding and one for the OIDC nonce — stored together
in a single cookie.

Rationale:

1. **Industry standard.** Cookie-bound state validation
   is the recommended approach in RFC 6749 §10.12 and
   is used by Auth0, Dex, and other production OIDC
   relying parties. The pattern is well-understood and
   widely deployed.

2. **Cryptographic independence.** Two separate
   128-bit random values ensure that the value exposed
   in the URL (`state`) is entirely independent of the
   value stamped into the ID token (`nonce`). If
   server logs, the Referer header, or browser history
   leak the `state` parameter, the attacker learns
   nothing about the OIDC nonce. If a downstream
   service exposes the ID token payload, the observer
   learns nothing about the state value or the cookie
   contents. This follows the OIDC specification's
   guidance in Section 15.5.2, which warns against
   exposing raw cookie values as nonces.

3. **Simple implementation.** Both values are stored
   in a single cookie separated by `:`. No hashing
   boilerplate is needed — just `crypto/rand`
   generation, string concatenation on the outbound
   redirect, and `strings.SplitN` on the callback.
   The code is straightforward and readable.

4. **Minimal architecture impact.** Changes are confined
   to a new cross-cutting `internal/crypto` package
   (nonce generation and comparison), the handler layer
   (cookie management, state construction/parsing), and
   the infrastructure layer (passing the nonce to
   `AuthCodeURL` and verifying it in `ExchangeCode`).
   No domain entity changes, no repository changes, no
   database migrations.

5. **Practical single-use behavior.** The cookie is set
   on redirect and cleared on callback. A replayed
   callback URL will fail because the cookie is already
   gone. This is browser-enforced rather than
   server-authoritative — a narrow race exists between
   the browser receiving the clearing response and
   processing it — but it is sufficient for the threat
   model. Option A cannot provide even this without
   server-side state.

6. **No secret management.** Unlike Option A, there is
   no HMAC key to provision, store, or rotate. Both
   values are generated fresh for each flow and
   discarded after verification.

7. **Cookie dependency is already present.** The bridge
   already requires cookie support for `saml_session`.
   Adding one more short-lived cookie does not introduce
   a new constraint.

## Implementation

### Flow

```text
Redirect (outbound):
  1. Generate stateValue (128-bit, crypto/rand)
  2. Generate nonceValue (128-bit, crypto/rand)
  3. Set cookie: oauth_nonce = <stateValue>:<nonceValue>
     (HttpOnly, Secure, SameSite=Lax, MaxAge=600)
  4. Build state: "<stateValue>:<requestID>:<relayState>"
  5. Redirect to AuthCodeURL(state, nonceValue)

Callback (inbound):
  1. Extract code and state from query parameters
  2. Read cookie: oauth_nonce (reject 403 if missing)
  3. Clear cookie (MaxAge=-1)
  4. Split cookie by ":" into cookieState and cookieNonce
     (reject 403 if malformed)
  5. Parse state: extract stateValue, requestID,
     relayState (reject 403 if malformed)
  6. Compare cookieState vs stateValue from state param
     (crypto/subtle.ConstantTimeCompare)
     (reject 403 if mismatch)
  7. Exchange code:
     ExchangeCode(ctx, code, cookieNonce)
     — code exchange MUST NOT execute until steps 2–6
     pass
  8. Proceed with session creation
```

### Error classification

A nonce mismatch detected during ID token verification
in the Hydra client must be returned as
`domain.ErrAuthentication`, not as a generic
`fmt.Errorf`. The OIDC service layer must preserve
`ErrAuthentication` errors from the Hydra client instead
of wrapping them as `ErrUpstream`. This ensures the
handler maps nonce failures to `403 Forbidden` (via the
existing `WriteError` switch) rather than
`502 Bad Gateway`.

### Interface changes

The `HydraClient` and `OIDCService` interfaces must be
updated to accept and verify the nonce:

| Interface | Current signature | New signature |
| --- | --- | --- |
| `HydraClient.AuthCodeURL` | `AuthCodeURL(state string) string` | `AuthCodeURL(state, nonce string) string` |
| `HydraClient.ExchangeCode` | `ExchangeCode(ctx, code string) (*domain.IDToken, error)` | `ExchangeCode(ctx, code, nonce string) (*domain.IDToken, error)` |
| `OIDCService.AuthCodeURL` | `AuthCodeURL(state string) string` | `AuthCodeURL(state, nonce string) string` |
| `OIDCService.ExchangeCode` | `ExchangeCode(ctx, code string) (*OIDCClaims, error)` | `ExchangeCode(ctx, code, nonce string) (*OIDCClaims, error)` |

### Files to modify

| File | Change |
| --- | --- |
| `internal/crypto/nonce.go` | Nonce generation (crypto/rand), constant-time comparison, and cookie value encoding/decoding |
| `internal/handler/saml_adapters.go` | Generate state and nonce values, set cookie with both, embed state value in state param |
| `internal/handler/oidc_callback.go` | Read and clear cookie, split cookie into state and nonce, validate state value, pass nonce to ExchangeCode |
| `internal/service/interfaces.go` | Update `OIDCService` and `HydraClient` signatures |
| `internal/service/oidc.go` | Forward nonce to `HydraClient` methods |
| `internal/infrastructure/hydra/client.go` | Add `nonce` param to `AuthCodeURL`, verify `nonce` in `ExchangeCode` |
| `mocks/` | Regenerate via `make generate` |

### Cookie attributes

The `oauth_nonce` cookie mirrors the security attributes
of the existing `saml_session` cookie:

- `HttpOnly: true` — not accessible to JavaScript
- `Secure: !DevMode` — sent only over HTTPS in
  production
- `SameSite: Lax` — sent on top-level navigations only
- `MaxAge: 600` — 10-minute expiry covers the OIDC
  round-trip
- `Path: /saml/callback` — scoped to the callback
  endpoint

## Consequences

- The bridge becomes resistant to login CSRF attacks.
  A forged callback URL cannot succeed because the
  attacker cannot set the `oauth_nonce` cookie in the
  victim's browser for the bridge's domain.
- The bridge becomes resistant to ID token replay
  attacks. A previously issued ID token will have a
  nonce that does not match the current flow's expected
  nonce.
- The state value (exposed in URLs, logs, Referer
  headers) is cryptographically independent of the
  nonce value (stamped into the ID token). Leaking one
  does not compromise the other. This achieves true
  defense-in-depth and follows OIDC specification
  Section 15.5.2 guidance.
- No database migrations or domain model changes are
  required. The fix is confined to a new cross-cutting
  `internal/crypto` package, the handler layer, and the
  infrastructure layer.
- No secret key management is needed. Both values are
  generated fresh and discarded after verification.
- The `HydraClient` and `OIDCService` interfaces gain
  a `nonce` parameter, requiring mock regeneration and
  test updates. This is a breaking change to the service
  interfaces.
- Existing tests for `HandleOIDCCallback` and
  `SAMLSessionAdapter.GetSession` must be updated to
  set and verify the `oauth_nonce` cookie containing
  both state and nonce values.
- The `parseState` function changes from a two-part
  format (`requestID:relayState`) to a three-part format
  (`stateValue:requestID:relayState`), which is an
  internal implementation detail with no external impact.
- Browsers that block cookies entirely will not be able
  to complete authentication. This is already the case
  due to the `saml_session` cookie, so no new constraint
  is introduced.
- Concurrent login flows in the same browser (e.g. two
  tabs initiating SAML authentication simultaneously)
  will conflict: the second tab's redirect overwrites
  the `oauth_nonce` cookie, causing the first tab's
  callback to fail nonce validation. This is an accepted
  limitation of the cookie-bound approach. Users can
  retry the failed tab. If concurrent flows become a
  requirement, Option C (server-side nonce on
  `PendingAuthnRequest`) would eliminate this limitation
  at the cost of a domain/migration change.
- ID token nonce mismatch errors from the Hydra client
  must be classified as `domain.ErrAuthentication` so
  the handler returns `403 Forbidden`. The OIDC service
  must not wrap these as `domain.ErrUpstream` (which
  would incorrectly produce `502 Bad Gateway`).
