# Service Provider Admin CLI

A command-line tool for managing SAML service providers in the Identity SAML Provider system.

## Build

To build the CLI:

```bash
go build -o bin/service-provider-admin ./cmd/service-provider-admin
```

## Usage

### Adding a Service Provider

Register a new SAML service provider with the Identity SAML Provider:

```bash
./bin/service-provider-admin add --entity-id <entity-id> --acs-url <acs-url> [--acs-binding <binding>] [--server <server-url>]
```

#### Flags

- `--entity-id, -e` (required): Entity ID of the service provider. Must be a valid URL (e.g., `https://example.com`)
- `--acs-url, -a` (required): Assertion Consumer Service (ACS) URL where SAML responses are sent (e.g., `https://example.com/saml/acs`)
- `--acs-binding, -b` (optional): ACS binding type. Defaults to `urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST`
- `--server` (optional): Base URL of the Identity SAML Provider server. Defaults to `http://localhost:8082`
- `--output` (optional): Output format: `human` for human-readable output (default) or `json` for machine-readable JSON

#### Examples

**Register a service provider locally:**

```bash
./bin/service-provider-admin add \
  --entity-id https://myapp.local:8083 \
  --acs-url https://myapp.local:8083/saml/acs
```

**Register a service provider on a remote server:**

```bash
./bin/service-provider-admin add \
  --server https://saml-provider.example.com \
  --entity-id https://myapp.example.com \
  --acs-url https://myapp.example.com/saml/acs \
  --acs-binding urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect
```

#### Response

**Human-Readable Output (default):**

```
✓ Service provider registered successfully!
  Entity ID: https://myapp.local:8083
  ACS URL: https://myapp.local:8083/saml/acs
  ACS Binding: urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST
```

**JSON Output (with `--output json`):**

```json
{
  "success": true,
  "entity_id": "https://myapp.local:8083",
  "acs_url": "https://myapp.local:8083/saml/acs",
  "acs_binding": "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
  "response": {
    "entity_id": "https://myapp.local:8083",
    "message": "Service provider registered",
    "status": "success"
  }
}
```

## API Endpoint

The CLI communicates with the `/admin/service-providers` endpoint of the Identity SAML Provider:

```
POST /admin/service-providers
Content-Type: application/json

{
  "entity_id": "https://myapp.example.com",
  "acs_url": "https://myapp.example.com/saml/acs",
  "acs_binding": "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
}
```

## Requirements

- Entity ID and ACS URL must be valid URLs with `http://` or `https://` scheme
- The Identity SAML Provider server must be running and accessible

## Building for Different Platforms

```bash
# macOS (ARM64)
GOOS=darwin GOARCH=arm64 go build -o bin/service-provider-admin ./cmd/service-provider-admin

# Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o bin/service-provider-admin ./cmd/service-provider-admin

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/service-provider-admin.exe ./cmd/service-provider-admin
```
