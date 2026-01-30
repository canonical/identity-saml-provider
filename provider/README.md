# SAML-OIDC Bridge

A bridge application that enables SAML-based Single Sign-On (SSO) through an OIDC provider (Ory Hydra), allowing integration between any SAML Service Provider and Hydra.

## Architecture

This application acts as an intermediary between:
1. **SAML Service Provider** - Initiates SAML authentication
2. **Ory Hydra** (OIDC Provider) - Handles user authentication
3. **This Bridge** (SAML Identity Provider) - Translates between SAML and OIDC

## Prerequisites

- Go 1.21 or later
- OpenSSL (for generating certificates)
- Ory Hydra instance running on `http://localhost:4444`

## Setup

### 1. Generate SAML Certificates

Generate a self-signed certificate and key using the Makefile:

```bash
make certs
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Run the Application

```bash
make run
```

The application will start on `http://localhost:8080`.

## Configuration

Edit the constants at the top of `main.go`:

- `BridgeBaseURL`: URL where this bridge is running
- `HydraPublicURL`: Ory Hydra public issuer URL
- `ClientID` / `ClientSecret`: Hydra OAuth2 credentials
- Service Provider configuration (ACS URL and Entity ID)

## Endpoints

- `GET /saml/metadata` - SAML metadata endpoint (provide this to your SAML Service Provider)
- `GET /saml/sso` - SAML SSO entry point (Service Provider redirects here)
- `GET /callback` - OIDC callback endpoint (Hydra redirects here)

## Flow

1. User initiates login from the SAML Service Provider
2. Service Provider sends SAML AuthnRequest to `/saml/sso`
3. Application redirects user to Hydra for authentication
4. Hydra authenticates user and redirects back to `/callback`
5. Application extracts user info and generates SAML Response
6. User is redirected back to the Service Provider with SAML assertion

## Security Notes

In production:
- Load configuration from environment variables, not hardcoded constants
- Use HTTPS instead of HTTP
- Validate and sign/encrypt the RelayState parameter
- Implement proper error handling and logging
- Use a database for dynamic Service Provider configurations
- Rotate certificates regularly
