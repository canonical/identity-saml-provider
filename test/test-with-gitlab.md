# How to test with GitLab CE

## 1) Start GitLab CE with SAML enabled

Use the compose file at [test/docker-compose.gitlab-ce.yml](docker-compose.gitlab-ce.yml).

First, set the IdP certificate fingerprint expected by GitLab:

```bash
export IDP_CERT_FP="$(
  curl -fsSL http://localhost:8082/saml/metadata \
  | perl -0777 -ne 'if (/<(?:\w+:)?X509Certificate\b[^>]*>([^<]+)<\/(?:\w+:)?X509Certificate>/) { print $1; exit }' \
  | base64 -d \
  | openssl x509 -inform DER -noout -fingerprint -sha1 \
  | cut -d= -f2
)"
```

Start GitLab:

```bash
docker compose -f test/docker-compose.gitlab-ce.yml up -d
```

## 2) Register the GitLab SP in identity-saml-provider

Register this Service Provider with the bridge:

```bash
curl -sS -X POST http://localhost:8082/admin/service-providers \
  -H 'Content-Type: application/json' \
  -d '{
    "entity_id": "http://localhost:8929",
    "acs_url": "http://localhost:8929/users/auth/saml/callback",
    "acs_binding": "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
  }'
```

## 3) Values to use (summary)

- **SP Entity ID**: `http://localhost:8929`
- **SP ACS URL**: `http://localhost:8929/users/auth/saml/callback`
- **IdP SSO URL**: `http://localhost:8082/saml/sso`
- **IdP metadata URL**: `http://localhost:8082/saml/metadata`

Open GitLab at `http://localhost:8929` and use the SAML login option.
