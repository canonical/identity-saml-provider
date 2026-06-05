# ADR 007: Secure Session ID Generation

## Status

Proposed

## Context

Session IDs are generated in
`internal/service/session.go` using
`time.Now().UnixNano()`:

```go
sessionID := fmt.Sprintf("_%d", time.Now().UnixNano())
```

This value is stored in the database as the primary key
and set directly as the `saml_session` HTTP cookie value
in `internal/handler/oidc_callback.go`:

```go
http.SetCookie(w, &http.Cookie{
    Name:  "saml_session",
    Value: session.ID,
    ...
})
```

Later in the authentication flow,
`SAMLSessionAdapter.GetSession()` in
`internal/handler/saml_adapters.go` reads this cookie
and performs a direct database lookup by session ID. The
session ID is the **sole authentication bearer token** —
whoever presents a valid ID in the cookie is treated as
the authenticated user.

### Problems with the current approach

1. **Predictability.** Nanosecond timestamps are
   monotonically increasing. An attacker who can estimate
   the approximate time a user authenticated only needs
   to brute-force a narrow range. At nanosecond
   resolution, each second of uncertainty is roughly
   one billion candidates — feasible for an automated
   attack.

2. **Collision risk.** Under high concurrency, two
   goroutines calling `time.Now().UnixNano()`
   simultaneously can receive the same value,
   particularly on systems with coarse clock resolution.
   The `ON CONFLICT (id) DO UPDATE` upsert in the
   repository would silently overwrite one user's
   session with another's.

3. **Session hijacking.** Because the session ID is
   the only factor used for session retrieval (no
   additional binding to IP, user-agent, or HMAC), a
   guessed or collided ID directly enables session
   hijacking or fixation.

### Database performance consideration

The `sessions` table uses `id TEXT PRIMARY KEY` with a
B-tree index. Sessions expire after 10 minutes and are
actively cleaned up by `CleanupExpired()`, so the table
remains small (at most a few thousand rows). At this
scale:

- Primary key point lookups are O(log n) regardless of
  whether keys are sequential or random.
- Insert-time page splits from random keys are
  negligible for a table this small.
- The only range-based query (`DeleteExpired`) uses the
  separate `idx_sessions_expire_time` index, not the
  primary key.

Therefore, the time-based format provides **no
meaningful query performance benefit** over random IDs.

## Proposed Solutions

### Option A: `crypto/rand` with base64url encoding

```go
import (
    "crypto/rand"
    "encoding/base64"
)

func generateSessionID() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate session ID: %w", err)
    }
    return "_" + base64.RawURLEncoding.EncodeToString(b), nil
}
```

Output example: `_a3Bf9x2Kp7mN...` (44 characters).

### Option B: UUIDv4 via `google/uuid`

```go
import "github.com/google/uuid"

func generateSessionID() string {
    return "_" + uuid.NewString()
}
```

Output example: `_550e8400-e29b-41d4-a716-446655440000`
(38 characters).

### Option C: `crypto/rand` with hex encoding

```go
import "crypto/rand"

func generateSessionID() (string, error) {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate session ID: %w", err)
    }
    return fmt.Sprintf("_%x", b), nil
}
```

Output example: `_a3bf9c2e7d4f...` (33 characters).

### Comparison

| Criteria | Option A (base64url) | Option B (UUIDv4) | Option C (hex) |
| --- | --- | --- | --- |
| Entropy | 256 bits | 122 bits | 128 bits |
| External deps | None | `google/uuid` | None |
| Error handling | Returns `error` | None (panics) | Returns `error` |
| Output length | 44 chars | 38 chars | 33 chars |
| Log readability | Moderate | High | High |
| Code complexity | Low | Lowest | Low |
| Cookie/URL safe | Yes | Yes | Yes |

All three options are cryptographically sufficient for
session tokens. The meaningful differences are:

- **Option A** maximises entropy with zero dependencies.
- **Option B** is simplest at the call site but adds an
  external dependency the project does not currently
  vendor, and `uuid.NewString()` panics instead of
  returning an error on RNG failure.
- **Option C** is a simpler variant of Option A with
  less entropy (still adequate) but no real advantage
  over Option A.

## Decision

Use **Option A: `crypto/rand` with base64url encoding**.

Rationale:

1. **Zero new dependencies.** Uses only Go standard
   library (`crypto/rand`, `encoding/base64`), aligning
   with the project's preference for standard-first
   solutions.

2. **Maximum entropy.** 256 bits provides a large safety
   margin against brute-force, even accounting for
   future hardware improvements.

3. **Explicit error handling.** Returning an error from
   `rand.Read` is the correct defensive pattern for
   security-sensitive code. In practice this only fails
   on catastrophic OS-level RNG issues, but surfacing
   it lets the service fail loudly rather than produce
   a weak session ID.

4. **Drop-in replacement.** The session ID is used
   purely as an opaque lookup key throughout the
   codebase. No code parses, decodes, or extracts
   meaning from its format. The change is confined to
   the `CreateFromOIDC` method in
   `internal/service/session.go`.

5. **Preserves the `_` prefix** for backward
   compatibility with any existing sessions and the
   current cookie/index conventions.

## Consequences

- `internal/service/session.go` will import
  `crypto/rand` and `encoding/base64` instead of
  `time` (if `time` is no longer needed for ID
  generation; it remains needed for `CreateTime` and
  `ExpireTime`).
- `CreateFromOIDC` will gain an additional error path
  for the unlikely case that `rand.Read` fails.
- Existing active sessions (if any) are unaffected.
  The new format is backwards-compatible because the
  database column is `TEXT` with no format constraints.
- No database migration is required.
- Unit tests that assert on session ID format (e.g.,
  matching `_\d+`) will need to be updated.
