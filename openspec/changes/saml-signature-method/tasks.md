# Tasks

## 1. Configuration & Validation

- [ ] 1.1 Add `SAMLSignatureMethod` field with `envconfig:"SAML_PROVIDER_SIGNATURE_METHOD" default:"rsa-sha256"` to `app.Config` in `internal/app/config.go` and implement case-insensitive validation in `Validate()` to accept `rsa-sha256`, `rsa-sha384`, `rsa-sha512`, and `rsa-sha1` while rejecting unsupported algorithms (Unit tests: verify env loading, default value, case normalization, and rejection of invalid values in `internal/app/config_test.go`).
- [ ] 1.2 Add helper method `Config.SAMLSignatureMethodURI()` in `internal/app/config.go` returning the corresponding W3C XMLDSIG URI constant (Unit tests: verify URI mapping for all supported algorithms in `internal/app/config_test.go`).

## 2. IdP Initialization & Signing Wiring

- [ ] 2.1 Add `SignatureMethod string` to `samlkit.Config` in `internal/infrastructure/samlkit/idp.go` and configure `idp.SignatureMethod` during `NewIdentityProvider` (Unit tests: verify `SignatureMethod` assignment and behavior in `internal/infrastructure/samlkit/idp_test.go`).
- [ ] 2.2 Update application composition root in `internal/app/app.go` to pass the resolved signature method URI from `app.Config` into `samlkit.Config`.

## 3. Metadata Algorithm Advertisement

- [ ] 3.1 Implement metadata handler `HandleMetadata` in `internal/handler/saml_adapters.go` (and wire in `internal/handler/routes.go`) that renders `/saml/metadata` with the standard `<md:KeyDescriptor use="signing">` and enriches the XML with `<alg:SigningMethod>` and `<alg:DigestMethod>` elements (Unit tests: table-driven tests in `internal/handler/saml_adapters_test.go` verifying valid metadata XML, KeyDescriptor presence, and algorithm extension attributes).

## 4. Verification Suite

- [ ] 4.1 Run `make fmt`, `make lint`, `make test`, `make build`, and `make license-check` to verify that the entire verification suite passes cleanly with zero warnings.

## 5. Documentation & Rollout

- [ ] 5.1 Document `SAML_PROVIDER_SIGNATURE_METHOD` in configuration docs and confirm backward compatibility for existing deployments.

## Implementation Notes

<!-- Record any deviations from the plan during implementation -->
