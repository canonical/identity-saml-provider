# Proposal

## Why

Service Providers (such as Stripe, Miro, Google Workspace, and Microsoft Entra ID) increasingly require modern, collision-resistant cryptographic algorithms (specifically `RSA-SHA256`) for signing SAML Assertions and SAML Responses. The Identity SAML Provider currently relies on library defaults that fall back to `RSA-SHA1` when no signature method is explicitly configured, causing authentication failures or signature validation errors with modern enterprise Service Providers.

Configuring a secure-by-default signature method and advertising supported signature capabilities in SAML metadata ensures seamless compatibility with both strict modern SPs and legacy SPs.

## What Changes

- Add configurable SAML signature method support via the `SAML_PROVIDER_SIGNATURE_METHOD` environment variable.
- Default the signing algorithm to `rsa-sha256` (secure by default), while supporting `rsa-sha384`, `rsa-sha512`, and explicit legacy opt-in for `rsa-sha1`.
- Ensure both the XML `<saml:Assertion>` and enclosing `<saml2p:Response>` elements are consistently signed with the configured algorithm.
- Publish the active X.509 certificate in standard `<md:KeyDescriptor use="signing">` and advertise supported signature methods via standard metadata algorithm extensions (`<alg:SigningMethod>` and `<alg:DigestMethod>`) on `/saml/metadata`.
- Log an explicit warning at startup if the operator explicitly configures deprecated `rsa-sha1`.

## Capabilities

### New Capabilities
- `saml-signature-method`: Configures cryptographic signature algorithms for SAML assertions, responses, and metadata publication, defaulting to secure RSA-SHA256.

### Modified Capabilities
<!-- No existing capabilities requirements are changing -->

## Non-goals

- Support for elliptic curve (ECDSA) signatures when configured with an RSA key pair.
- Per-Service-Provider signature algorithm negotiation or per-SP database overrides (Option A global IdP configuration is implemented).
- SAML request signing verification for SP-initiated authn requests (this change focuses on IdP-issued responses and assertions).

## Success Metrics

- Outgoing SAML Assertions and Responses are signed with `RSA-SHA256` and digest `SHA-256` by default.
- Modern Service Providers (including Stripe and Miro) successfully validate the IdP's signatures without rejecting the assertion or response.
- Configuration validation accepts `rsa-sha256`, `rsa-sha384`, `rsa-sha512`, and `rsa-sha1` (case-insensitively), and rejects invalid algorithms at startup with a descriptive error.
- `/saml/metadata` successfully serves standard KeyDescriptors and algorithm extension elements while preserving full backward compatibility.

## Impact

- Environment variables: Introduces `SAML_PROVIDER_SIGNATURE_METHOD` (default: `rsa-sha256`).
- Configuration: Updates `app.Config`, validation rules, and `samlkit.Config`.
- Identity Provider: Updates `saml.IdentityProvider.SignatureMethod` initialization in `internal/infrastructure/samlkit/idp.go`.
- Metadata Handler: Updates `/saml/metadata` handler in `internal/handler/` to emit algorithm extension tags alongside KeyDescriptors.
- Database: Zero database schema changes or migrations required.
