# Changelog

## [1.15.0](https://github.com/sablierapp/sablier/compare/v1.14.0...v1.15.0) (2026-07-09)


### Features

* add anti-affinity ([#997](https://github.com/sablierapp/sablier/issues/997)) ([4c183c2](https://github.com/sablierapp/sablier/commit/4c183c2348c10da39e4ae037975e56854873168f))
* add next build tag and multi-versionned doc ([#1009](https://github.com/sablierapp/sablier/issues/1009)) ([b14ff78](https://github.com/sablierapp/sablier/commit/b14ff78b3b88ef74fda224f3272b9d6e6a01d1b8))
* add running-days ([#994](https://github.com/sablierapp/sablier/issues/994)) ([c981a92](https://github.com/sablierapp/sablier/commit/c981a924eff8c95e414b1b135fa20f7961187563))
* **docker:** add blkio throttling support to scale mode ([#935](https://github.com/sablierapp/sablier/issues/935)) ([60987e2](https://github.com/sablierapp/sablier/commit/60987e2c1536ac4c867332602af8e631184a674a))
* **docker:** respect restart policy and resolve `depends_on` on start ([#956](https://github.com/sablierapp/sablier/issues/956)) ([b050d14](https://github.com/sablierapp/sablier/commit/b050d1486b2c3882a8f847631a89f20444597ad3))
* **metrics:** expose per-session expiry as a Prometheus gauge ([#987](https://github.com/sablierapp/sablier/issues/987)) ([89215db](https://github.com/sablierapp/sablier/commit/89215db336b5913e3adb64db4f7b8580782410e6))
* **provider:** add auto-warm-externally-started option ([#993](https://github.com/sablierapp/sablier/issues/993)) ([26cc2cf](https://github.com/sablierapp/sablier/commit/26cc2cf15b683d4b7c6467226a256fa4695d747a)), closes [#985](https://github.com/sablierapp/sablier/issues/985)
* **sablier:** add ready-on-start support via sablier.ready-on-start label ([#984](https://github.com/sablierapp/sablier/issues/984)) ([3d700e8](https://github.com/sablierapp/sablier/commit/3d700e8dd7c6b4afb04b82d589f810b5d030cc40))


### Bug Fixes

* **kubernetes:** honor names-based replica count on instance start ([#1003](https://github.com/sablierapp/sablier/issues/1003)) ([3e4dab9](https://github.com/sablierapp/sablier/commit/3e4dab9be8c56b2f11ce136852f007160324acd7)), closes [#258](https://github.com/sablierapp/sablier/issues/258)


### Documentation

* add giscus ([#1007](https://github.com/sablierapp/sablier/issues/1007)) ([66fdf34](https://github.com/sablierapp/sablier/commit/66fdf34f13f35e4f15b1a879741b568aa21173bd))
* use version template ([#1008](https://github.com/sablierapp/sablier/issues/1008)) ([e63413f](https://github.com/sablierapp/sablier/commit/e63413fa2d1f1eff6d774ceea889c5daacb3f43c))
* website rework using Hugo ([#1000](https://github.com/sablierapp/sablier/issues/1000)) ([5abdcbb](https://github.com/sablierapp/sablier/commit/5abdcbb27d9188741e0a92661d3d3392cdb56c4d))


### Tests

* fix test failures ([#998](https://github.com/sablierapp/sablier/issues/998)) ([adcb35b](https://github.com/sablierapp/sablier/commit/adcb35bfe40e64aec3c5fc0f300d4c28e5fb70fd))

## [1.14.0](https://github.com/sablierapp/sablier/compare/v1.13.0...v1.14.0) (2026-06-12)


### Features

* **provider:** pause redis-operator reconciliation during StatefulSet scale-to-zero ([#963](https://github.com/sablierapp/sablier/issues/963)) ([f3f1af0](https://github.com/sablierapp/sablier/commit/f3f1af0118742d1f0d539c3100612fbe2771d4b6))


### Bug Fixes

* **kubernetes:** address code review findings on redis-operator skip-reconcile ([f3f1af0](https://github.com/sablierapp/sablier/commit/f3f1af0118742d1f0d539c3100612fbe2771d4b6))

## [1.13.0](https://github.com/sablierapp/sablier/compare/v1.12.0...v1.13.0) (2026-05-29)


### ⚠ BREAKING CHANGES

* sablier.idle.cpu / sablier.idle.memory no longer enable throttling on their own. Add sablier.idle.replicas=1 to existing workloads that relied on CPU/memory throttling.

### Features

* add --auto-stop-externally-started flag ([#906](https://github.com/sablierapp/sablier/issues/906)) ([d2fd51d](https://github.com/sablierapp/sablier/commit/d2fd51d09692cee90604199aba7d7f61b7bf9ef7))
* add `--provider.reject-unlabeled-requests` and `--provider.verify-enabled-on-expiration`  ([#901](https://github.com/sablierapp/sablier/issues/901)) ([f1a4e61](https://github.com/sablierapp/sablier/commit/f1a4e617e38a40c2f6a1749face383cf3d9b2c5a))
* add OpenTelemetry distributed tracing support ([#929](https://github.com/sablierapp/sablier/issues/929)) ([68faefd](https://github.com/sablierapp/sablier/commit/68faefd4bcfb57fa76754bf9aa011334abb11ac4))
* add replica count to scale mode, gate resource throttling on idle.replicas &gt;= 1 ([#909](https://github.com/sablierapp/sablier/issues/909)) ([5722c6f](https://github.com/sablierapp/sablier/commit/5722c6f831d9c8729d8e2a914e47f9e134e6c5e3))
* add running-hours keep-warm windows and timezone support ([#907](https://github.com/sablierapp/sablier/issues/907)) ([ed9c611](https://github.com/sablierapp/sablier/commit/ed9c611e180ead41f567f6bbc204ca8dc3a94bbf))
* add theme options schema ([#917](https://github.com/sablierapp/sablier/issues/917)) ([20828ca](https://github.com/sablierapp/sablier/commit/20828ca6b62010bfb171c07a9d003414508e7889))
* add webhooks ([#920](https://github.com/sablierapp/sablier/issues/920)) ([a608a8c](https://github.com/sablierapp/sablier/commit/a608a8ccc6bf3a8f6ca4b43e60361ba9dfb9b56e))
* **api:** add SSE instance events endpoint ([#916](https://github.com/sablierapp/sablier/issues/916)) ([2eccf89](https://github.com/sablierapp/sablier/commit/2eccf89a9cb44b15012d17235bfed6f8ade0bb99))
* **metrics:** add sablier_instance_active_seconds_total metric ([#932](https://github.com/sablierapp/sablier/issues/932)) ([e92f438](https://github.com/sablierapp/sablier/commit/e92f438b4ca6c86b594058c22523e23cab2b1b8f))
* **provider:** add ready-after instance configuration ([#904](https://github.com/sablierapp/sablier/issues/904)) ([ff60d31](https://github.com/sablierapp/sablier/commit/ff60d31728f99eb3206c7153dcb80cadeeca89c6))
* **provider:** support CloudNativePG Cluster hibernation in Kubernetes ([#944](https://github.com/sablierapp/sablier/issues/944)) ([199f12e](https://github.com/sablierapp/sablier/commit/199f12ea5f33b049d2aafacd90ff7c5847d5477f)), closes [#943](https://github.com/sablierapp/sablier/issues/943)
* scale mode to throttle CPU/memory instead of stopping containers ([#908](https://github.com/sablierapp/sablier/issues/908)) ([d39804f](https://github.com/sablierapp/sablier/commit/d39804f10ea2b2e605cfa0007d03eb867c92a616))
* support multiple groups per instance ([#913](https://github.com/sablierapp/sablier/issues/913)) ([5b0f896](https://github.com/sablierapp/sablier/commit/5b0f896d2f63759be3051fa9c9e9e302798d36c4))
* **theme:** bundle external CSS, JS, and images into custom themes ([#928](https://github.com/sablierapp/sablier/issues/928)) ([e9ba051](https://github.com/sablierapp/sablier/commit/e9ba0515fb49d557790e684598641afedc9b7048))


### Bug Fixes

* add missing benchmarks folder ([3b1b421](https://github.com/sablierapp/sablier/commit/3b1b4219b8cfabd3a62e7da67ece0d6069887418))
* **api:** handle multiple error types ([#921](https://github.com/sablierapp/sablier/issues/921)) ([2dba936](https://github.com/sablierapp/sablier/commit/2dba936e3a72dc8b07d8191df1c6d23b5de3d12a))
* go routine leak in session request ([#927](https://github.com/sablierapp/sablier/issues/927)) ([91c57c3](https://github.com/sablierapp/sablier/commit/91c57c3b5eb1658df04b2d022e51bf32eb52a4a4))
* multiple sonarqube fixes ([#924](https://github.com/sablierapp/sablier/issues/924)) ([e5d7546](https://github.com/sablierapp/sablier/commit/e5d7546dd120bb71044b98357b6a15edf2317814))
* remove built-in configuration file ([#905](https://github.com/sablierapp/sablier/issues/905)) ([9f3dd79](https://github.com/sablierapp/sablier/commit/9f3dd79537e16580a8abae77ecc651c72bb02433))
* run go fix ([#926](https://github.com/sablierapp/sablier/issues/926)) ([02c1223](https://github.com/sablierapp/sablier/commit/02c12238b462f37c1a59fe1b11d99cc31a558d8a))
* **storage:** properly save and restore file ([#933](https://github.com/sablierapp/sablier/issues/933)) ([5a8f480](https://github.com/sablierapp/sablier/commit/5a8f480f9cf786c4bba29b3e94d2a878f80c68aa))
* **webhooks:** avoid gorouting leaks ([#925](https://github.com/sablierapp/sablier/issues/925)) ([c02d98b](https://github.com/sablierapp/sablier/commit/c02d98b95c58a824f531d99bce78c782f1e76f30))


### Documentation

* add benchmarking/performance ([#931](https://github.com/sablierapp/sablier/issues/931)) ([33abb7e](https://github.com/sablierapp/sablier/commit/33abb7e0f36855087d0836f0d0dff0156b0f93df))
* add docker socket proxy example ([#902](https://github.com/sablierapp/sablier/issues/902)) ([af8127b](https://github.com/sablierapp/sablier/commit/af8127b92546c973dde11c190ad46391b955af1a))
* add table of content divider ([44134a3](https://github.com/sablierapp/sablier/commit/44134a30ccc09a3c8ca43510f38e962cc7bbe4eb))
* **examples:** example verify enabled on expiration ([#911](https://github.com/sablierapp/sablier/issues/911)) ([3e64b9e](https://github.com/sablierapp/sablier/commit/3e64b9e9098d57dae7d8d46197190d250f1ca70c))
* release please update version typo ([b92169a](https://github.com/sablierapp/sablier/commit/b92169a028a249db5fa8f194546e511abd07bb82))
* update configuration docs and README ([#930](https://github.com/sablierapp/sablier/issues/930)) ([57a65d9](https://github.com/sablierapp/sablier/commit/57a65d97ed8e7b1c844db6c6b780195795dc022b))


### Code Refactoring

* watch groups through events ([#910](https://github.com/sablierapp/sablier/issues/910)) ([44b57a9](https://github.com/sablierapp/sablier/commit/44b57a99615dd7f7e91af56aedb4fd2fed1a14e4))


### Chores

* add OpenTelemetry patterns to dependabot config ([68866ea](https://github.com/sablierapp/sablier/commit/68866ea6372bae243b2a5308bb9989f97fb5b555))

## [1.12.0](https://github.com/sablierapp/sablier/compare/v1.11.2...v1.12.0) (2026-05-13)


### Features

* add Prometheus /metrics endpoint ([#884](https://github.com/sablierapp/sablier/issues/884)) ([b0a0237](https://github.com/sablierapp/sablier/commit/b0a023707d28859dd33dcf84635dbcef8d777cb4))
* add raw instance data in InstanceInfo ([#892](https://github.com/sablierapp/sablier/issues/892)) ([d09b56c](https://github.com/sablierapp/sablier/commit/d09b56c10f51bf684e3395276dc32d38d3aca696))
* **docker:** add event stream reconnection ([#891](https://github.com/sablierapp/sablier/issues/891)) ([0510380](https://github.com/sablierapp/sablier/commit/0510380bfe9d4cf71817e63fa91bf54f85cb80a1)), closes [#535](https://github.com/sablierapp/sablier/issues/535)
* Proxmox VE LXC provider ([#868](https://github.com/sablierapp/sablier/issues/868)) ([4692a09](https://github.com/sablierapp/sablier/commit/4692a09e463ff8355e75eac4c48eb6198557d6fe))


### Bug Fixes

* async InstanceStart calls ([#869](https://github.com/sablierapp/sablier/issues/869)) ([3c1430f](https://github.com/sablierapp/sablier/commit/3c1430f18d2573f326b9eb6f7c6565a4e51ad9ac))
* **log:** add INFO logs for started instances ([#871](https://github.com/sablierapp/sablier/issues/871)) ([4d1ea62](https://github.com/sablierapp/sablier/commit/4d1ea622a2ca436cee4408aa71d8e6231660ea98))
* remove volume declaration from Dockerfile ([#899](https://github.com/sablierapp/sablier/issues/899)) ([9ed4d11](https://github.com/sablierapp/sablier/commit/9ed4d11f49ece5c26d22e30855338c7ada63f40c))
* use moby/moby/client ([#753](https://github.com/sablierapp/sablier/issues/753)) ([a5babb9](https://github.com/sablierapp/sablier/commit/a5babb9678d2f3a079e3d4283eaed2709da9deb7))


### Code Refactoring

* **podman:** use moby/moby/client instead of podman bindings ([#885](https://github.com/sablierapp/sablier/issues/885)) ([858f12a](https://github.com/sablierapp/sablier/commit/858f12a6e4d56c5b7895883d3524ab51e1ed752b))


### Tests

* **kubernetes:** run all tests in a single cluster ([#889](https://github.com/sablierapp/sablier/issues/889)) ([aaf20b4](https://github.com/sablierapp/sablier/commit/aaf20b4e1130b6c1486d19809e67d6004fe37d6c))
* **sonar:** remove generated code from coverage ([2c25b04](https://github.com/sablierapp/sablier/commit/2c25b0468adfb710f2118fbb9eb3e8814fcfb26d))

## [1.11.2](https://github.com/sablierapp/sablier/compare/v1.11.1...v1.11.2) (2026-04-03)


### Bug Fixes

* **cmd:** return an error instead of panic ([#820](https://github.com/sablierapp/sablier/issues/820)) ([d7f6403](https://github.com/sablierapp/sablier/commit/d7f640382c2f82f1f3918aec3899a661bcf0e9bb))
* **deps:** bump the k8s-io group across 1 directory with 3 updates ([#828](https://github.com/sablierapp/sablier/issues/828)) ([13bd6b2](https://github.com/sablierapp/sablier/commit/13bd6b2c46ca502911e9e318f0489753833313c3))
* handle instance start errors in dynamic and blocking strategies ([#854](https://github.com/sablierapp/sablier/issues/854)) ([69b3d57](https://github.com/sablierapp/sablier/commit/69b3d57035c6036c925f47c791b8e4f4219407ea))

## [1.11.1](https://github.com/sablierapp/sablier/compare/v1.11.0...v1.11.1) (2026-01-14)


### Bug Fixes

* **config:** use SABLIER_ environment variable prefix ([#790](https://github.com/sablierapp/sablier/issues/790)) ([e9c2213](https://github.com/sablierapp/sablier/commit/e9c2213d49320bba9cf7ecd0f6585c221da13f64))
* **logging:** access log level is set to debug ([#791](https://github.com/sablierapp/sablier/issues/791)) ([6e46cdb](https://github.com/sablierapp/sablier/commit/6e46cdba313605e01af52072e695db32d84282e3))


### Documentation

* clarify configuration source evaluation order ([#788](https://github.com/sablierapp/sablier/issues/788)) ([ecdfd37](https://github.com/sablierapp/sablier/commit/ecdfd37901fff470ded9690e62f414330b5afbf6))

## [1.11.0](https://github.com/sablierapp/sablier/compare/v1.10.5...v1.11.0) (2025-12-05)


### Features

* **docker:** add docker pause strategy  ([#755](https://github.com/sablierapp/sablier/issues/755)) ([0d699ef](https://github.com/sablierapp/sablier/commit/0d699effc34260be35aba0d3acb57d8775bf8f8b))


### Documentation

* add mimic healthcheck ([ff5c447](https://github.com/sablierapp/sablier/commit/ff5c4476bd0a45f4495fdb8e32e1d18e0b32ec4e))
* add quick start ([58896b9](https://github.com/sablierapp/sablier/commit/58896b9feb27577ff8a41e5a8f3e3438761b2f5f))
* update provider features ([d7685f0](https://github.com/sablierapp/sablier/commit/d7685f04a1afd4df1b4acb511ecd9c6051b17e99))

## [1.10.5](https://github.com/sablierapp/sablier/compare/v1.10.4...v1.10.5) (2025-11-23)


### Bug Fixes

* **dockerfile:** remove running as non-root ([b3d356a](https://github.com/sablierapp/sablier/commit/b3d356ac63d636791a7eed20adac29fde184fe26))
* warn when the custom theme path does not exist ([112bdaa](https://github.com/sablierapp/sablier/commit/112bdaaf8f1eb6259a4cd8ff6f8b6e179add0700))


### Documentation

* set version to 1.10.4 ([d66a143](https://github.com/sablierapp/sablier/commit/d66a143a9c22711724a5217fa58c1233a56e1d21))

## [1.10.4](https://github.com/sablierapp/sablier/compare/v1.10.3...v1.10.4) (2025-11-23)


### Bug Fixes

* **dockerfile:** copy file with proper permissions ([#746](https://github.com/sablierapp/sablier/issues/746)) ([12d00e0](https://github.com/sablierapp/sablier/commit/12d00e05bee50f8e3814ce828f4d468457c1ef7b))


### Documentation

* add signature verification ([3417baf](https://github.com/sablierapp/sablier/commit/3417baf0898e218b34cc5f2b72a04161bc69d289))

## [1.10.3](https://github.com/sablierapp/sablier/compare/v1.10.2...v1.10.3) (2025-11-23)


### Bug Fixes

* **goreleaser:** reverse --snapshot ternary expression ([d834d17](https://github.com/sablierapp/sablier/commit/d834d178efdda758a6ccf850b617b91e289da101))


### Documentation

* fix podman documentation link ([#743](https://github.com/sablierapp/sablier/issues/743)) ([080e4d1](https://github.com/sablierapp/sablier/commit/080e4d1a1f7702b63a146b6607064bf2876a51c6))

## [1.10.2](https://github.com/sablierapp/sablier/compare/v1.10.1...v1.10.2) (2025-11-23)


### ⚠ BREAKING CHANGES

* remove plugins from the repository

### fix\

* remove plugins from the repository ([6d88092](https://github.com/sablierapp/sablier/commit/6d880928c7ec1992b998ef78a43334b1f4823026))


### Bug Fixes

* bump go 1.25 ([#723](https://github.com/sablierapp/sablier/issues/723)) ([0588627](https://github.com/sablierapp/sablier/commit/0588627d3623109aa792ae20c16be9fa4b2814a2))


### Documentation

* add CONTRIBUTING.md ([a4d7dda](https://github.com/sablierapp/sablier/commit/a4d7ddae05fd5df141386139a5aee80304499476))
* add discord server and link pugin repositories ([c5ac357](https://github.com/sablierapp/sablier/commit/c5ac3578822089db228c82f8c0a282202476b7cf))
* add github and docker icons ([da5d2d1](https://github.com/sablierapp/sablier/commit/da5d2d113a83558697387da35141053699e82093))
* add helm chart ([71a35ca](https://github.com/sablierapp/sablier/commit/71a35ca8dd273901d7e0469ff68e61bfa77241ff))
* add OpenSSF scorecard ([dc5fb22](https://github.com/sablierapp/sablier/commit/dc5fb22b4007814f9db92eb626ab0c1000c3c7cd))
* add release please glob file ([64550ff](https://github.com/sablierapp/sablier/commit/64550ff25bb82571d49b99d058b4a5c9731f7481))
* add release please tags to update version ([8bc21fb](https://github.com/sablierapp/sablier/commit/8bc21fbaa91e62b5ea83991c46e134e79ede271a))
* add sponsor section ([e2b7965](https://github.com/sablierapp/sablier/commit/e2b7965f7a54194ba2b053f7cef0df8b750ffd8f))
* add support section ([09f99f6](https://github.com/sablierapp/sablier/commit/09f99f69ba7b3089ed9d3a1cdcf97dd4184ec961))
* improve documentation clarity ([a6614de](https://github.com/sablierapp/sablier/commit/a6614de3ad4e3d0119a43e1c0b85145a58f4a187))
* reference plugin repositories ([33209ac](https://github.com/sablierapp/sablier/commit/33209acc6c7c597a15ec1c9846683e8cdd9c5b7a))
* **traefik:** set traefik plugin version to v1.10.1 ([#654](https://github.com/sablierapp/sablier/issues/654)) ([8af2f32](https://github.com/sablierapp/sablier/commit/8af2f3265da6478a9224e2639452c83aae485544))
* update documentation ([96750a5](https://github.com/sablierapp/sablier/commit/96750a50da79fe0a4594480d82a40dee5855c017))


### Code Refactoring

* add sabliercmd pkg ([#727](https://github.com/sablierapp/sablier/issues/727)) ([0f4a3a2](https://github.com/sablierapp/sablier/commit/0f4a3a2e930aa41915d5eb46c60d39fd664a7143))
