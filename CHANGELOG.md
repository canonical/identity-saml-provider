# Changelog

## [0.2.0](https://github.com/canonical/identity-saml-provider/compare/v0.1.7...v0.2.0) (2026-08-31)


### ⚠ BREAKING CHANGES

* Refactor CLI Command Structure and Error Handling

### Features

* allows specifying PostgreSQL CA certificate location ([0ac3187](https://github.com/canonical/identity-saml-provider/commit/0ac3187d4c450851c9b5dcad85b8e1a6f112eba3))
* configurable persistent NameID mode ([3b3fd0e](https://github.com/canonical/identity-saml-provider/commit/3b3fd0e04b19812601d5e1ff19e73b820a5b7265))
* persist pending requests ([0bc95b8](https://github.com/canonical/identity-saml-provider/commit/0bc95b8f1cc1c08286c44c305b4655374b0f361d))
* standardize cli formatting and output streams ([9cc96ee](https://github.com/canonical/identity-saml-provider/commit/9cc96ee1ad5170218df7076ae502c1b5279f0b54))


### Bug Fixes

* **deps:** update go deps ([4d915bb](https://github.com/canonical/identity-saml-provider/commit/4d915bb825e1045adb9c86d00cccd2281d2625a7))
* **deps:** update go deps ([d6f57da](https://github.com/canonical/identity-saml-provider/commit/d6f57da41ac58784b0abbac468cde928ad4169ba))
* **deps:** update go deps to v1.45.0 ([9d3f622](https://github.com/canonical/identity-saml-provider/commit/9d3f6227ffbcb59147915aee98306613e3e7b2a3))
* **deps:** update go deps to v1.45.0 ([#167](https://github.com/canonical/identity-saml-provider/issues/167)) ([18b0f0b](https://github.com/canonical/identity-saml-provider/commit/18b0f0bc14536284c8f6fd237ce9eb0e7948d6c8))
* **deps:** update go deps to v1.46.0 ([922dac8](https://github.com/canonical/identity-saml-provider/commit/922dac8cade9f1e1d934a3d9ae2277e8625ff1fc))
* **deps:** update go deps to v1.46.0 ([#203](https://github.com/canonical/identity-saml-provider/issues/203)) ([38bb362](https://github.com/canonical/identity-saml-provider/commit/38bb362be2ebc5e8d374d57917bb867e5371239c))
* **deps:** update module github.com/go-chi/chi/v5 to v5.3.2 ([1e55ba7](https://github.com/canonical/identity-saml-provider/commit/1e55ba7b579aa08e423e167b75c13d6b59844f9c))
* **deps:** update module github.com/go-chi/chi/v5 to v5.3.2 ([#194](https://github.com/canonical/identity-saml-provider/issues/194)) ([44e5bdd](https://github.com/canonical/identity-saml-provider/commit/44e5bdd6450a09f8a9379d452b4d61d53a9bf73a))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.3 ([d77b5b8](https://github.com/canonical/identity-saml-provider/commit/d77b5b80ac141f59ec7713cfa2c78c7288b32c0c))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.3 ([#164](https://github.com/canonical/identity-saml-provider/issues/164)) ([2608f7d](https://github.com/canonical/identity-saml-provider/commit/2608f7d5a7f7221d80a94cc5c159b32a599979a7))
* **deps:** update module github.com/prometheus/client_golang to v1.24.0 ([e8c7bc0](https://github.com/canonical/identity-saml-provider/commit/e8c7bc085944402644326647700a103a33b2130a))
* **deps:** update module github.com/prometheus/client_golang to v1.24.0 ([#161](https://github.com/canonical/identity-saml-provider/issues/161)) ([4040ccf](https://github.com/canonical/identity-saml-provider/commit/4040ccfbf2d72d21ba05c916d4ac0735a4026f2a))
* **deps:** update module github.com/prometheus/client_golang to v1.24.1 ([4c1a03e](https://github.com/canonical/identity-saml-provider/commit/4c1a03eda26341c80b4f4a5bdded3aecc9f52d36))
* **deps:** update module github.com/prometheus/client_golang to v1.24.1 ([#165](https://github.com/canonical/identity-saml-provider/issues/165)) ([752782f](https://github.com/canonical/identity-saml-provider/commit/752782f368693337b980eb3850099b2fa9ea94a6))
* **deps:** update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.71.0 ([48c1417](https://github.com/canonical/identity-saml-provider/commit/48c1417f00b6430c9fb6af73dccb36760bee389e))
* **deps:** update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.71.0 ([#207](https://github.com/canonical/identity-saml-provider/issues/207)) ([838c6d6](https://github.com/canonical/identity-saml-provider/commit/838c6d65459560cf11024f0d1fb2558254185f6e))
* **deps:** update module go.opentelemetry.io/contrib/propagators/jaeger to v1.46.0 ([ca6ae9f](https://github.com/canonical/identity-saml-provider/commit/ca6ae9f4b6dfcff35bfb78f008e70d43e1514a5b))
* **deps:** update module go.opentelemetry.io/contrib/propagators/jaeger to v1.46.0 ([#205](https://github.com/canonical/identity-saml-provider/issues/205)) ([308d550](https://github.com/canonical/identity-saml-provider/commit/308d550c4990cbaf2dd9def8c2c0840a5a51ee76))


### Code Refactoring

* Refactor CLI Command Structure and Error Handling ([227f0f8](https://github.com/canonical/identity-saml-provider/commit/227f0f818d78d6a77af068c554652b9132f087bb))

## [0.1.7](https://github.com/canonical/identity-saml-provider/compare/v0.1.6...v0.1.7) (2026-07-13)


### Bug Fixes

* Admin APIs for SP attribute mapping ([8c07899](https://github.com/canonical/identity-saml-provider/commit/8c07899bb8bca91b5950fd3a1ffb67aa6317cb66))
* apply the deployment order in skaffold ([efd1657](https://github.com/canonical/identity-saml-provider/commit/efd1657f11982717d60d1e568b926c0c33bdd6c0))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.2 ([fb80926](https://github.com/canonical/identity-saml-provider/commit/fb8092645643c0adac0d69684d5b69efc1a58318))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.2 ([#133](https://github.com/canonical/identity-saml-provider/issues/133)) ([7bdc905](https://github.com/canonical/identity-saml-provider/commit/7bdc9051124f42632f12fe1ae56eaa02358709fe))
* internal user model and rich saml attributes ([71c6ce0](https://github.com/canonical/identity-saml-provider/commit/71c6ce0c47f0ea83b2d40ff7b3de2536f6fa98f4))
* persistent id ([9c07a4e](https://github.com/canonical/identity-saml-provider/commit/9c07a4e0981f3993c60bc89ef64eda2f800edffa))
* relocate saml session conversion ([2097fa4](https://github.com/canonical/identity-saml-provider/commit/2097fa4402ba0aabd64f693069a5b9ec429fa7dd))
* unify service provider registration cli option ([898b28e](https://github.com/canonical/identity-saml-provider/commit/898b28e848ed39e7bf99738bd1255c748e540549))
* use custom assertion maker ([e2d22cd](https://github.com/canonical/identity-saml-provider/commit/e2d22cdd4f9f7497e7b3c40b3d7c818721d9dc62))
* validate attribute mapping fields ([ef78fcd](https://github.com/canonical/identity-saml-provider/commit/ef78fcdb60fe43b8339c6927c6f6166c0f3c9aac))

## [0.1.6](https://github.com/canonical/identity-saml-provider/compare/v0.1.5...v0.1.6) (2026-06-05)


### Bug Fixes

* pin dependencies ([c4213a4](https://github.com/canonical/identity-saml-provider/commit/c4213a405ec7f62c8123ad7667f91e2062dbba54))

## [0.1.5](https://github.com/canonical/identity-saml-provider/compare/v0.1.4...v0.1.5) (2026-06-05)


### Bug Fixes

* add healthiness and readiness endpoints ([e4cfd70](https://github.com/canonical/identity-saml-provider/commit/e4cfd703e48ac68cd93180fff01c3e132ee532ec))
* align the error paths using the custom json helper ([fb5244e](https://github.com/canonical/identity-saml-provider/commit/fb5244ece4832a58ddaeaad84176b817e0e2a5b6))
* application graceful shutdown ([cc6e783](https://github.com/canonical/identity-saml-provider/commit/cc6e78369be1f8f4a501b56c6b94c448dcaef435))
* fix and improve the Makefiles ([3ef59d5](https://github.com/canonical/identity-saml-provider/commit/3ef59d55444fd962d7b50e3f34fd7ae3f5d3a01e))
* fix the id token claims type ([b8a7f68](https://github.com/canonical/identity-saml-provider/commit/b8a7f68140877613e854237600248b1b7d7c8778))
* fix the oauth2 csrf vulnerability and id token replay ([777a60e](https://github.com/canonical/identity-saml-provider/commit/777a60e356fa775a345512b412f2badb460010e2))
* fix the permission issues for the copilot update workflow ([61198de](https://github.com/canonical/identity-saml-provider/commit/61198def294ce752409d181a9371cb5409f49883))
* fix the pre-commit workflow ([4d6a610](https://github.com/canonical/identity-saml-provider/commit/4d6a61006d92667ac68169c2adfb277367870af8))
* fix the pre-commit workflow ([#96](https://github.com/canonical/identity-saml-provider/issues/96)) ([63ef566](https://github.com/canonical/identity-saml-provider/commit/63ef566647977274a1b1f68855400893c1af4155))
* fix the README.md ([c954fab](https://github.com/canonical/identity-saml-provider/commit/c954fab8d22a8df4813bd274081cc7af5eaea0c1))
* improve application configurations ([dbf2de1](https://github.com/canonical/identity-saml-provider/commit/dbf2de1fee0efcad7853880d99d78f64ef0cf711))
* introduce dev flag ([853a894](https://github.com/canonical/identity-saml-provider/commit/853a894b7a8d18fba160e01f4d28ebf8de6774de))
* logging enhancement ([eec2773](https://github.com/canonical/identity-saml-provider/commit/eec277384d169e3468d3c85b375abffb85fb6b9b))
* merge the service provider admin cli ([3701e8a](https://github.com/canonical/identity-saml-provider/commit/3701e8abb2288842bed7d8b9c67c4261297c404e))
* metrics enhancement ([e1db759](https://github.com/canonical/identity-saml-provider/commit/e1db75908ec02c3f93797fd30842447fffb084b9))
* redesign the hydra oidc client for TLS support ([b8fb27a](https://github.com/canonical/identity-saml-provider/commit/b8fb27a7a55c16754361c95e95696ee05bd0d364))
* remove the redundant child span in the handler layer ([257149d](https://github.com/canonical/identity-saml-provider/commit/257149ddf09c881ca03ff141e695a80b9fe2c3f4))
* remove the unnecessary utility functions in unit tests ([1e925c7](https://github.com/canonical/identity-saml-provider/commit/1e925c7c466aa4f6b23ad949f690384951553db5))
* secure the session id generation ([8c6a530](https://github.com/canonical/identity-saml-provider/commit/8c6a530c545904fefbd4c0ba148fbf2fd75eb01e))
* tracing enhancement ([9aba131](https://github.com/canonical/identity-saml-provider/commit/9aba1318865e52f4c9297700d6b353b9105d1900))

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
