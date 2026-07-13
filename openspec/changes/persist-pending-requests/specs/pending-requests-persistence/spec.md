# Spec: Persist pending requests

## Purpose

The purpose of this specification is to define the required behavior for
persisting SAML authentication requests during the SAML-to-OIDC login flow.
This capability ensures that the authentication process can survive across
multiple application replicas and captures critical client metadata for
security auditing.

## ADDED Requirements

### Requirement: Cross-Replica Login Continuity

The system SHALL persist the state of an initiated SAML login such that the
OIDC callback can be successfully processed by any application replica in the
cluster, not just the replica that initiated the flow.

#### Scenario: Successful distributed login

- **WHEN** a user initiates a login on Replica A and completes OIDC
  authentication on Replica B
- **THEN** the system successfully maps the OIDC claims back to the original
  SAML request
- **AND** the system returns a valid SAML Response to the Service Provider

### Requirement: Atomic State Consumption

The system MUST consume a pending authentication request exactly once.
Replaying the callback endpoint with the same state MUST fail to prevent
replay attacks.

#### Scenario: Replayed callback request

- **WHEN** a user successfully hits the `/saml/callback` endpoint and
  establishes a session
- **AND** the user or an attacker reloads the `/saml/callback` URL with the
  exact same parameters
- **THEN** the system MUST reject the second request and return an error
- **AND** no duplicate session is created

### Requirement: Client Metadata Capture

The system SHALL capture the originating client's HTTP metadata at the moment
the SAML authentication flow is initiated and persist it alongside the
pending request.

The system MUST securely extract the real client IP address from proxy headers
(e.g., `X-Forwarded-For` or `X-Real-IP`) when deployed behind load balancers or
reverse proxies, rather than relying exclusively on the network socket's
remote IP.

#### Scenario: Standard login flow metadata capture

- **WHEN** a user sends a SAML AuthnRequest to `/saml/sso`
- **THEN** the system records the client's real IP address and User-Agent string
- **AND** makes this data available during the final SAML Assertion
  generation step for auditing

### Requirement: Automated State Cleanup

The system SHALL automatically clean up pending requests that have been
abandoned by the user (e.g., the user never completes the OIDC
authentication).

#### Scenario: User abandons login flow

- **WHEN** an authentication request remains pending for more than 15 minutes
- **THEN** the system automatically deletes the request from the persistent
  store
- **AND** attempting to complete the login flow for that request results in a
  failure

### Requirement: Metadata Extensibility

The system MUST store client metadata in a flexible format that allows
tracking additional client attributes in the future without requiring
structural database changes.

#### Scenario: Capturing additional headers

- **WHEN** the application is updated to track the `Accept-Language` header
- **THEN** the system can store this new attribute in the pending request
  persistence layer without executing a database schema migration.

### Requirement: Expired or Missing Request Graceful Rejection

The system MUST intercept authentication requests with missing or expired SAML
request tokens and return a structured JSON API error response instead of
throwing a generic parser error.

#### Scenario: Intercepting missing SAMLRequest on SSO

- **WHEN** a user navigates to `/saml/sso` without a `SAMLRequest` parameter
  (e.g. because their login request expired before callback completed)
- **THEN** the system MUST return an HTTP 400 Bad Request status code
- **AND** the response body MUST be a structured JSON error payload matching the
  standard APIError format:
  `{"status": 400, "message": "missing or expired SAMLRequest parameter"}`
