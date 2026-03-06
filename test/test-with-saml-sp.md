# How to test with SimpleSAMLphp SP

## 1) Start the SimpleSAMLphp SP with SAML enabled

Use the compose file at [test/docker-compose.saml-sp.yml](docker-compose.saml-sp.yml).

Start the SP:

```bash
docker compose -f test/docker-compose.saml-sp.yml up -d
```

(Optional) if you want a non-default port:

```bash
SIMPLESAMLPHP_SP_PORT=3001 docker compose -f test/docker-compose.saml-sp.yml up -d
```

## 2) Register the SimpleSAMLphp SP in identity-saml-provider

Register this Service Provider with the bridge:

```bash
curl -sS -X POST http://localhost:8082/admin/service-providers \
  -H 'Content-Type: application/json' \
  -d '{
    "entity_id": "http://localhost:3001",
    "acs_url": "http://localhost:3001/simplesaml/module.php/saml/sp/saml2-acs.php/default-sp",
    "acs_binding": "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
  }'
```

## 3) Values to use (summary)

- **SP Entity ID**: `http://localhost:3001`
- **SP ACS URL**: `http://localhost:3001/simplesaml/module.php/saml/sp/saml2-acs.php/default-sp`
- **SP metadata URL**: `http://localhost:3001/simplesaml/module.php/saml/sp/metadata.php/default-sp`
- **IdP SSO URL**: `http://localhost:8082/saml/sso`
- **IdP metadata URL**: `http://localhost:8082/saml/metadata`

Open `http://localhost:3001/simplesaml` and authenticate using the `default-sp` source.
