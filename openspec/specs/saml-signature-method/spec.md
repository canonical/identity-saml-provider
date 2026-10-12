# saml-signature-method Specification

## Purpose

Defines the cryptographic signing algorithm configuration for SAML Assertions and SAML Responses issued by the Identity SAML Provider. This capability ensures compliance with modern Service Provider (SP) security mandates (such as Stripe and Miro) that reject weak SHA-1 signatures, while providing configurable algorithm selection, universal metadata advertisement, and secure-by-default behavior.

## Requirements

### Requirement: Configurable SAML Signature Method

The system SHALL support configuring the cryptographic signature algorithm used to sign SAML Assertions and Responses via the `SAML_PROVIDER_SIGNATURE_METHOD` environment variable.

#### Scenario: Default signature method selection (Secure by default)
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is omitted or empty
- **THEN** the system defaults the signature algorithm to `rsa-sha256` (`http://www.w3.org/2001/04/xmldsig-more#rsa-sha256`)
- **AND** signs both the SAML Assertion and SAML Response elements using RSA-SHA256 and SHA-256 digest

#### Scenario: Explicitly configured valid signature method
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is set to a supported algorithm identifier (e.g., `rsa-sha256`, `rsa-sha384`, `rsa-sha512`)
- **THEN** the system configures the SAML identity provider to sign assertions and responses using the designated algorithm and corresponding digest hash

#### Scenario: Case-insensitive algorithm naming
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is provided in mixed or upper case (e.g., `RSA-SHA256`, `rsa-sha512`)
- **THEN** the system normalizes the value and applies the matching signature algorithm without error

#### Scenario: Invalid signature method provided
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is set to an unrecognized or unsupported value (e.g., `md5`, `none`, `invalid-algo`)
- **THEN** system startup validation fails with an error indicating that the signature method is invalid and listing the supported values

### Requirement: Legacy Algorithm Opt-in and Safety Guardrails

The system SHALL permit legacy `rsa-sha1` only when explicitly configured, while warning operators of its deprecated security posture.

#### Scenario: Explicit legacy SHA-1 configuration
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is explicitly set to `rsa-sha1`
- **THEN** the system signs SAML assertions and responses using RSA-SHA1 (`http://www.w3.org/2000/09/xmldsig#rsa-sha1`)
- **AND** logs a warning indicating that SHA-1 is deprecated and should only be used for backward compatibility with legacy service providers

#### Scenario: Default behavior rejects SHA-1 fallback
- **WHEN** `SAML_PROVIDER_SIGNATURE_METHOD` is omitted
- **THEN** the system MUST NOT silently fall back to RSA-SHA1

### Requirement: Dual Assertion and Response Signing Integrity

The system SHALL apply the configured signature algorithm consistently to both the XML Assertion and the enclosing XML Response.

#### Scenario: Dual signature consistency
- **WHEN** a user completes authentication and the bridge issues a SAML response
- **THEN** the `<saml:Assertion>` XML element MUST contain a `<ds:Signature>` using the configured signature method and digest method
- **AND** the outer `<saml2p:Response>` XML element MUST contain a `<ds:Signature>` using the configured signature method and digest method

### Requirement: Universal Metadata Algorithm Advertisement

The system SHALL publish the signing certificate via standard SAML 2.0 KeyDescriptors while advertising supported signature algorithms to ensure cross-SP compatibility.

#### Scenario: Standard KeyDescriptor preservation
- **WHEN** a service provider requests `/saml/metadata`
- **THEN** the metadata response includes standard `<md:KeyDescriptor use="signing">` containing the IdP X.509 certificate
- **AND** service providers strictly expecting standard KeyDescriptors (such as Stripe and Miro) successfully discover the signing key

#### Scenario: Algorithm capability advertisement
- **WHEN** `/saml/metadata` is requested
- **THEN** the metadata XML advertises the configured signature and digest algorithm capabilities alongside standard KeyDescriptors
- **AND** service providers supporting algorithm discovery recognize RSA-SHA256 support automatically

### Requirement: Backward Compatibility and Upgrade Safety

Upgrading the application without configuration changes SHALL transition deployments to secure `rsa-sha256` by default without requiring database migrations or service downtime.

#### Scenario: Existing deployment upgrade without environment variable change
- **WHEN** an existing deployment is upgraded and no `SAML_PROVIDER_SIGNATURE_METHOD` variable is set
- **THEN** outgoing SAML tokens automatically use `rsa-sha256`, satisfying requirements for modern SPs (e.g., Stripe, Miro)

#### Scenario: Interoperability with legacy SP requiring SHA-1
- **WHEN** an operator connects to an older SP that does not support SHA-256
- **THEN** the operator can restore SHA-1 behavior by setting `SAML_PROVIDER_SIGNATURE_METHOD=rsa-sha1` without modifying application source code
