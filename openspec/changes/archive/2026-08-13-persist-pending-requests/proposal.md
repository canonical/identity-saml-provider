# Proposal: Persist pending requests

## Why

The SAML provider currently stores in-flight authentication requests (SAML
AuthnRequest) in local memory. This prevents the application from scaling
horizontally, as the OIDC callback flow will fail if routed to a different
pod. Additionally, the current implementation lacks client metadata auditing
(Issue #104), making it impossible to track the originating client IP and
User-Agent for security purposes. This change solves these scaling and
auditing limitations by persisting pending requests in PostgreSQL.

## What Changes

- Introduce a new PostgreSQL table `pending_requests` with an `expire_at`
  column to store and expire SAML authentication requests.
- Add a flexible `client_metadata` JSONB column to capture request metadata,
  initially supporting `client_ip` and `user_agent`.
- Update the SAML authentication flow to `INSERT` into the database instead of
  memory when a user initiates login, pre-calculating the `expire_at`
  timestamp.
- Update the OIDC callback handler to consume (`DELETE RETURNING`) the pending
  request from the database, filtering by `expire_at >= NOW()`. This ensures
  that even if the background janitor has not run yet, expired requests are
  rejected consistently.
- Remove the `internal/repository/memory` package completely.
- Add an extensible background `janitor pending-requests` subcommand to clean
  up abandoned authentication requests.

## Capabilities

## New Capabilities

- `pending-requests-persistence`: The capability to persist SAML
  authentication requests across a multi-node deployment, ensuring stateless
  application servers during the login process, and capturing client
  metadata for security auditing with consistent time-based expiration
  enforcement.

## Modified Capabilities

## Non-goals

- Implementing full session management in Redis (this change is strictly
  scoped to the transient `pending_requests` using PostgreSQL).
- Introducing complex reporting dashboards for the collected client metadata;
  this proposal only covers the data collection and storage.
- Adding read-replica configuration support. We will maintain the current
  single-pool architecture for simplicity, as the login flow naturally binds
  to the primary writer node.

## Success Metrics

- **Scalability**: The application can be deployed with >1 replica behind a
  load balancer without dropping active login flows.
- **Auditing**: 100% of completed SAML sessions can be correlated with the
  originating client's IP and User-Agent.
- **Reliability & Consistency**: Replay attacks on the `/saml/callback`
  endpoint fail consistently. Stale requests (older than TTL) are guaranteed to
  be rejected on callback retrieval even if the background janitor hasn't
  executed yet.

## Impact

- **Database**: Adds a new table `pending_requests` and an index on
  `expire_at` for efficient passive retrieval and background cleanup.
- **Storage**: Increased write load on the primary PostgreSQL node due to the
  transient nature of authentication requests.
- **Codebase**: Replaces the in-memory repository implementation with a
  PostgreSQL-backed one, and introduces the subcommand-based `janitor` CLI with
  structured text/json outputs.
