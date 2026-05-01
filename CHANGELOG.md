# Changelog

## [0.1.4](https://github.com/canonical/identity-saml-provider/compare/v0.1.3...v0.1.4) (2026-05-01)


### Features

* add per-SP attribute mapping mechanism (issue [#62](https://github.com/canonical/identity-saml-provider/issues/62)) ([8a16023](https://github.com/canonical/identity-saml-provider/commit/8a16023799b1e93c07aae1be4c524d9c2a85a452))
* adding additional server tests ([09d97d1](https://github.com/canonical/identity-saml-provider/commit/09d97d1c9cac8eb1c3176b588404e9cb5e905244))
* extract all OIDC claims from ID token for attribute mapping and document CLI usage in README ([b51bb51](https://github.com/canonical/identity-saml-provider/commit/b51bb517d53ef1efcd8be60b58e590264e68254e))


### Bug Fixes

* add migration for attribute mapping mechanism ([c5a20ee](https://github.com/canonical/identity-saml-provider/commit/c5a20eea52d2bb0da43225a3e5b9c7930c3750f0))
* fix the local dev testing kratos setup ([e9ce7da](https://github.com/canonical/identity-saml-provider/commit/e9ce7daee068ad6d430146c08c4e27d48a8cc957))

## [0.1.3](https://github.com/canonical/identity-saml-provider/compare/v0.1.2...v0.1.3) (2026-04-30)


### Bug Fixes

* add cli for application version ([55bd4e3](https://github.com/canonical/identity-saml-provider/commit/55bd4e334200b0d3ce588191c2561851ee84cf74))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.1 ([1599627](https://github.com/canonical/identity-saml-provider/commit/159962712aec83bb9f578f5d02f37544f01fc7f4))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.1 ([#85](https://github.com/canonical/identity-saml-provider/issues/85)) ([a6025f4](https://github.com/canonical/identity-saml-provider/commit/a6025f41666a960ed7ca9071990da3f5c480847d))
* **deps:** update module go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp to v1.43.0 [security] ([4d452f1](https://github.com/canonical/identity-saml-provider/commit/4d452f1de486a11332a2e04281fac54962a10ac3))
* **deps:** update module go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp to v1.43.0 [security] ([#70](https://github.com/canonical/identity-saml-provider/issues/70)) ([0ebe994](https://github.com/canonical/identity-saml-provider/commit/0ebe99464b20c9ce06e7206849d9a49fca6727f3))
* **deps:** update module go.opentelemetry.io/otel/sdk to v1.43.0 [security] ([9382cf3](https://github.com/canonical/identity-saml-provider/commit/9382cf34bccc4646fe8cae9249796062b03fd709))
* **deps:** update module go.opentelemetry.io/otel/sdk to v1.43.0 [security] ([#71](https://github.com/canonical/identity-saml-provider/issues/71)) ([4b2d080](https://github.com/canonical/identity-saml-provider/commit/4b2d080e7ee72a7e7bd6a5b9cdf25bc0e2a67282))
* improve the migration commands ([fdfe5b1](https://github.com/canonical/identity-saml-provider/commit/fdfe5b1a24f50c3d49e384a8961ccf720bbef318))
* remove URL-only restriction on EntityID to support any non-empty string ([157c370](https://github.com/canonical/identity-saml-provider/commit/157c370584bb816d01701e2c16c065383d200aad))

## [0.1.2](https://github.com/canonical/identity-saml-provider/compare/v0.1.1...v0.1.2) (2026-03-27)


### Features

* add metrics and tracing ([52ffa41](https://github.com/canonical/identity-saml-provider/commit/52ffa41149a6c720324fd0b4eb0c9250414aff6c))
* add metrics and tracing ([315b6b1](https://github.com/canonical/identity-saml-provider/commit/315b6b1740ba4c8ac4f4015854ffbf8496d216de))


### Bug Fixes

* adding middleware setup to routes ([c3cfebc](https://github.com/canonical/identity-saml-provider/commit/c3cfebca3e42c4e1eec19a35215e62abdd8e0aaf))
* adding param for callback ([bc95fa8](https://github.com/canonical/identity-saml-provider/commit/bc95fa80298d9b042ba7216aa1918cc1d7062025))
* adding saml prefix to callback ([2c81670](https://github.com/canonical/identity-saml-provider/commit/2c816700eba3bda3ad3ad08aed8563b51409e99f))
* changing redirect url from hydra ([bc95fa8](https://github.com/canonical/identity-saml-provider/commit/bc95fa80298d9b042ba7216aa1918cc1d7062025))
* setting static value to callback route ([bc95fa8](https://github.com/canonical/identity-saml-provider/commit/bc95fa80298d9b042ba7216aa1918cc1d7062025))
* update tests ([bc95fa8](https://github.com/canonical/identity-saml-provider/commit/bc95fa80298d9b042ba7216aa1918cc1d7062025))
* updated tests ([bc95fa8](https://github.com/canonical/identity-saml-provider/commit/bc95fa80298d9b042ba7216aa1918cc1d7062025))

## [0.1.1](https://github.com/canonical/identity-saml-provider/compare/identity-saml-provider-v0.1.0...identity-saml-provider-v0.1.1) (2026-03-06)


### Features

* allow insecure hydra connections ([e06ec27](https://github.com/canonical/identity-saml-provider/commit/e06ec27990c46c33141a2b6c9040229a1f14e5c6))
* allow insecure hydra connections ([#39](https://github.com/canonical/identity-saml-provider/issues/39)) ([3c4fa05](https://github.com/canonical/identity-saml-provider/commit/3c4fa05a9bedc8df29280103f85fc27d6423f5c3))
* allow setting hydra ca cert ([c0b444c](https://github.com/canonical/identity-saml-provider/commit/c0b444cd9293a9c47f533c69b63bab3c342e6e6c))
* allow setting hydra ca cert ([#41](https://github.com/canonical/identity-saml-provider/issues/41)) ([2a21b17](https://github.com/canonical/identity-saml-provider/commit/2a21b176f9b98dbebc3a6689b285332413dea846))

## 0.1.0 (2026-02-26)


### Features

* Add a skaffold dev environment ([#22](https://github.com/canonical/identity-saml-provider/issues/22)) ([167956b](https://github.com/canonical/identity-saml-provider/commit/167956b159843651e020a48aebfa3515a0daed3a))
* Add a verbose logging flag ([#19](https://github.com/canonical/identity-saml-provider/issues/19)) ([f5081a8](https://github.com/canonical/identity-saml-provider/commit/f5081a874b80567ba3bfb61c0662de2dd312192e))
* allow configuration via env vars ([#4](https://github.com/canonical/identity-saml-provider/issues/4)) ([4377e8f](https://github.com/canonical/identity-saml-provider/commit/4377e8f5fa7b94d761e900fcd42015e4688f1beb))
* API endpoint for adding service providers ([#10](https://github.com/canonical/identity-saml-provider/issues/10)) ([2e7f887](https://github.com/canonical/identity-saml-provider/commit/2e7f88703e4de4464543382ff02dc02d9f015322))
* CLI for adding service providers ([#18](https://github.com/canonical/identity-saml-provider/issues/18)) ([85a0b3a](https://github.com/canonical/identity-saml-provider/commit/85a0b3a74fa8c40930ab7e4eaa7c04a2cc671da4))
* cli output json or human ([#20](https://github.com/canonical/identity-saml-provider/issues/20)) ([3a14ae4](https://github.com/canonical/identity-saml-provider/commit/3a14ae4974216e79be0a82c030198956f03280bf))
* dev setup improvements ([#5](https://github.com/canonical/identity-saml-provider/issues/5)) ([b2c3d1d](https://github.com/canonical/identity-saml-provider/commit/b2c3d1d97cd300892761a4d673d56cc567cef413))
* improve logging with zap ([#11](https://github.com/canonical/identity-saml-provider/issues/11)) ([1385861](https://github.com/canonical/identity-saml-provider/commit/138586131cb9c627f5451524d4401bfe53ea4c37))
* store sessions in postgres ([#8](https://github.com/canonical/identity-saml-provider/issues/8)) ([4ce66b5](https://github.com/canonical/identity-saml-provider/commit/4ce66b5709111551f12f3f4f092417fcbecf0b64))


### Bug Fixes

* add the missing docker files ([#7](https://github.com/canonical/identity-saml-provider/issues/7)) ([45d8195](https://github.com/canonical/identity-saml-provider/commit/45d81959fcc6bfd04fb3aee9ca150b1109119239))


### Miscellaneous Chores

* initial release ([#34](https://github.com/canonical/identity-saml-provider/issues/34)) ([f489db0](https://github.com/canonical/identity-saml-provider/commit/f489db0feac26e084082d15553ead09afdcfbb8e))
