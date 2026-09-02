# Changelog

## [1.7.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.6.2...v1.7.0) (2026-09-02)


### Features

* accept provider credentials unknown until apply ([#103](https://github.com/fitbeard/terraform-provider-radosgw/issues/103)) ([fde4cf5](https://github.com/fitbeard/terraform-provider-radosgw/commit/fde4cf5bc6f5480743f146e031d4c04e6d888554))
* add bucket notification resources ([#23](https://github.com/fitbeard/terraform-provider-radosgw/issues/23)) ([d417f80](https://github.com/fitbeard/terraform-provider-radosgw/commit/d417f80770c7373fd860a5e77c00b25ecc8dac84))
* add initial account support ([#86](https://github.com/fitbeard/terraform-provider-radosgw/issues/86)) ([4ab676e](https://github.com/fitbeard/terraform-provider-radosgw/commit/4ab676ed1253cf956570a66df4a5fb5903271622))
* add radosgw_s3_bucket_cors_configuration resource ([#106](https://github.com/fitbeard/terraform-provider-radosgw/issues/106)) ([c539372](https://github.com/fitbeard/terraform-provider-radosgw/commit/c539372e64d1d9c0d2802ef685054df11cbefdf2))
* add radosgw_s3_bucket_website_configuration resource ([#24](https://github.com/fitbeard/terraform-provider-radosgw/issues/24)) ([6dd8e29](https://github.com/fitbeard/terraform-provider-radosgw/commit/6dd8e29cdbefdc02e773da0c3b66949762f86746))
* add release tools ([#1](https://github.com/fitbeard/terraform-provider-radosgw/issues/1)) ([31d774b](https://github.com/fitbeard/terraform-provider-radosgw/commit/31d774b86ea308cd05a4b473d7f26ee042344f63))
* add resources and datasources for iam_account users and access_keys ([#109](https://github.com/fitbeard/terraform-provider-radosgw/issues/109)) ([61b62cd](https://github.com/fitbeard/terraform-provider-radosgw/commit/61b62cd760129ffe685a66df24fa89530b1ecc0a))
* add resources for account users, groups and policies ([#110](https://github.com/fitbeard/terraform-provider-radosgw/issues/110)) ([d44c863](https://github.com/fitbeard/terraform-provider-radosgw/commit/d44c8631ee341966e92dc8d517f21c8a4b5c5075))
* add tags for iam_role ([#74](https://github.com/fitbeard/terraform-provider-radosgw/issues/74)) ([7d6f8f0](https://github.com/fitbeard/terraform-provider-radosgw/commit/7d6f8f013a73ed6f0a43b6de8d2a7526616918df))
* first major release ([62dc421](https://github.com/fitbeard/terraform-provider-radosgw/commit/62dc421b695d915e1bd8b1ed0a2e6b151d1ab2d9))
* initial publishing ([523e82f](https://github.com/fitbeard/terraform-provider-radosgw/commit/523e82fa2e5e41e0405330fe8db591cfbe7b2fe3))
* initial release config ([ea649ad](https://github.com/fitbeard/terraform-provider-radosgw/commit/ea649ade356ac93c60d6987f79fca573bbe09a30))
* move to golang 1.26 ([#104](https://github.com/fitbeard/terraform-provider-radosgw/issues/104)) ([7b1b914](https://github.com/fitbeard/terraform-provider-radosgw/commit/7b1b9140c6929c3067c2965e84da6bc20dcf23d8))


### Bug Fixes

* bucket versioning check during planning ([#125](https://github.com/fitbeard/terraform-provider-radosgw/issues/125)) ([4dae3b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/4dae3b7a0190509a2e9b5c9d440932339fbcbc17))
* bump hc-install to fix expired pgp key ([#62](https://github.com/fitbeard/terraform-provider-radosgw/issues/62)) ([c258303](https://github.com/fitbeard/terraform-provider-radosgw/commit/c258303d9ad418208b2a6d30baba4ac82092b9ac))
* correct account limit defaults and support capless/federated & tenant users ([#98](https://github.com/fitbeard/terraform-provider-radosgw/issues/98)) ([2d603cf](https://github.com/fitbeard/terraform-provider-radosgw/commit/2d603cf34b0ac2bd987cf0c16a34e5f00378327e))
* correctly resolve both local and tenant user id ([#71](https://github.com/fitbeard/terraform-provider-radosgw/issues/71)) ([2f337ef](https://github.com/fitbeard/terraform-provider-radosgw/commit/2f337ef04395e5c3b56999e9368bdb662f32bd98))
* documentation typos ([#12](https://github.com/fitbeard/terraform-provider-radosgw/issues/12)) ([c50b67e](https://github.com/fitbeard/terraform-provider-radosgw/commit/c50b67ef91ccefc08dc4213bf1100244dd6ea37c))
* go transitive dependencies ([#41](https://github.com/fitbeard/terraform-provider-radosgw/issues/41)) ([f501665](https://github.com/fitbeard/terraform-provider-radosgw/commit/f501665316e0dfb7041627415dc014b8ac78e9b9))
* linking for tenant owned buckets ([#105](https://github.com/fitbeard/terraform-provider-radosgw/issues/105)) ([7482722](https://github.com/fitbeard/terraform-provider-radosgw/commit/7482722ca93dc121fdf17cf57defd264e6136f6f))
* make install command ([#20](https://github.com/fitbeard/terraform-provider-radosgw/issues/20)) ([4f3ec06](https://github.com/fitbeard/terraform-provider-radosgw/commit/4f3ec06578e8b0eff79152f112cffba0cc19e42c))
* ordering for bucket cors settings ([#114](https://github.com/fitbeard/terraform-provider-radosgw/issues/114)) ([ee63f7e](https://github.com/fitbeard/terraform-provider-radosgw/commit/ee63f7e16724e938a6c5cd21cbe17572c67fbe6c))
* remove prerelease from goreleaser ([#11](https://github.com/fitbeard/terraform-provider-radosgw/issues/11)) ([27df5be](https://github.com/fitbeard/terraform-provider-radosgw/commit/27df5beeb2a269a222cb7d5f294b12a3ec04308e))
* retry transient SNS/notification errors under concurrent topic churn ([#91](https://github.com/fitbeard/terraform-provider-radosgw/issues/91)) ([fd49b0a](https://github.com/fitbeard/terraform-provider-radosgw/commit/fd49b0a0e45f58b86032e4512094d9160873b9ec))
* tests for openid_connect_provider for Reef ([#47](https://github.com/fitbeard/terraform-provider-radosgw/issues/47)) ([79c9371](https://github.com/fitbeard/terraform-provider-radosgw/commit/79c9371c7479016e4af9fd2b3f0b34ea9ac6c92b))


### Documentation

* add README ([f3009b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/f3009b7ab60b39710eedcbbd42b88d7e579bbc34))


### Miscellaneous

* **deps:** Bump actions/checkout from 6 to 7 ([#81](https://github.com/fitbeard/terraform-provider-radosgw/issues/81)) ([b68ecde](https://github.com/fitbeard/terraform-provider-radosgw/commit/b68ecdef6aa943671b762db12e0635c42ab81c3b))
* **deps:** Bump actions/setup-go from 6 to 7 ([#95](https://github.com/fitbeard/terraform-provider-radosgw/issues/95)) ([1edd12b](https://github.com/fitbeard/terraform-provider-radosgw/commit/1edd12bc7671103b70e0dee8121af082dd6184ba))
* **deps:** Bump crazy-max/ghaction-import-gpg from 6.3.0 to 7.0.0 ([#31](https://github.com/fitbeard/terraform-provider-radosgw/issues/31)) ([14523ca](https://github.com/fitbeard/terraform-provider-radosgw/commit/14523caa800d46b5b64606e66aaae3f37ff01657))
* **deps:** Bump github.com/aws/aws-sdk-go-v2 from 1.41.0 to 1.41.1 ([#6](https://github.com/fitbeard/terraform-provider-radosgw/issues/6)) ([6483a8a](https://github.com/fitbeard/terraform-provider-radosgw/commit/6483a8a2abfc58fe0bf948f0950b29bd851fdf81))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#22](https://github.com/fitbeard/terraform-provider-radosgw/issues/22)) ([80bf3df](https://github.com/fitbeard/terraform-provider-radosgw/commit/80bf3df0e9b59142911f06be9fe408a3dbdd9f07))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#35](https://github.com/fitbeard/terraform-provider-radosgw/issues/35)) ([ba60c46](https://github.com/fitbeard/terraform-provider-radosgw/commit/ba60c46190de6f14957b5c69bcbc927cdc19c6b9))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#44](https://github.com/fitbeard/terraform-provider-radosgw/issues/44)) ([02747b5](https://github.com/fitbeard/terraform-provider-radosgw/commit/02747b5368cd8d6be579ab057ae0b360225e5b1c))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#5](https://github.com/fitbeard/terraform-provider-radosgw/issues/5)) ([8dab06b](https://github.com/fitbeard/terraform-provider-radosgw/commit/8dab06b8b8a72062f7f0783d2dfb99a487a3d799))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#73](https://github.com/fitbeard/terraform-provider-radosgw/issues/73)) ([6962dbe](https://github.com/fitbeard/terraform-provider-radosgw/commit/6962dbe87739d63a6c26e0657a9cb71b4cdbf4e2))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#28](https://github.com/fitbeard/terraform-provider-radosgw/issues/28)) ([fb88338](https://github.com/fitbeard/terraform-provider-radosgw/commit/fb883385d5fce46baa937761ce5ee48aab55654c))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#32](https://github.com/fitbeard/terraform-provider-radosgw/issues/32)) ([1094041](https://github.com/fitbeard/terraform-provider-radosgw/commit/10940414bc26f32c270d24a89ae57a10726f71b0))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#4](https://github.com/fitbeard/terraform-provider-radosgw/issues/4)) ([9f026f2](https://github.com/fitbeard/terraform-provider-radosgw/commit/9f026f2df94c2ecfae5e22043660533abdd6890e))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#42](https://github.com/fitbeard/terraform-provider-radosgw/issues/42)) ([e93491a](https://github.com/fitbeard/terraform-provider-radosgw/commit/e93491a9d6249127b71d4bcc1372fd70498e772e))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#49](https://github.com/fitbeard/terraform-provider-radosgw/issues/49)) ([f0fa9b3](https://github.com/fitbeard/terraform-provider-radosgw/commit/f0fa9b376f595e253665f4df140c077ca4d8f2d7))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#69](https://github.com/fitbeard/terraform-provider-radosgw/issues/69)) ([5ab91ae](https://github.com/fitbeard/terraform-provider-radosgw/commit/5ab91ae1406798f47389f3eb67821650c970d835))
* **deps:** Bump github.com/aws/smithy-go from 1.24.2 to 1.24.3 ([#55](https://github.com/fitbeard/terraform-provider-radosgw/issues/55)) ([4b3b2c4](https://github.com/fitbeard/terraform-provider-radosgw/commit/4b3b2c483b8ad47145bb4c2e05422d5c6a62b863))
* **deps:** Bump github.com/aws/smithy-go from 1.25.0 to 1.25.1 ([#66](https://github.com/fitbeard/terraform-provider-radosgw/issues/66)) ([7e17ce6](https://github.com/fitbeard/terraform-provider-radosgw/commit/7e17ce6876fce50e1a528dbeaef5b699a3b680da))
* **deps:** Bump github.com/aws/smithy-go from 1.27.1 to 1.27.3 ([#84](https://github.com/fitbeard/terraform-provider-radosgw/issues/84)) ([679a6a9](https://github.com/fitbeard/terraform-provider-radosgw/commit/679a6a9bde9c9217ce6cdf3ecf0c82290942102d))
* **deps:** Bump github.com/aws/smithy-go from 1.27.5 to 1.27.6 ([#102](https://github.com/fitbeard/terraform-provider-radosgw/issues/102)) ([0289be6](https://github.com/fitbeard/terraform-provider-radosgw/commit/0289be6d87364793b147a26fc10e8fb987fe7a4d))
* **deps:** Bump github.com/ceph/go-ceph from 0.38.0 to 0.39.0 ([#60](https://github.com/fitbeard/terraform-provider-radosgw/issues/60)) ([aca5e97](https://github.com/fitbeard/terraform-provider-radosgw/commit/aca5e97db26bcf032eedd3850bb4f90ce235a720))
* **deps:** Bump github.com/ceph/go-ceph from 0.39.0 to 0.40.0 ([#82](https://github.com/fitbeard/terraform-provider-radosgw/issues/82)) ([4c6a4ce](https://github.com/fitbeard/terraform-provider-radosgw/commit/4c6a4ceaeeb9347fe4b9b059d27568ff7ed8f498))
* **deps:** Bump github.com/ceph/go-ceph from 0.40.0 to 0.41.0 ([#117](https://github.com/fitbeard/terraform-provider-radosgw/issues/117)) ([b38bd0d](https://github.com/fitbeard/terraform-provider-radosgw/commit/b38bd0d8edc7f3c05526053e17e24a5e84a47a16))
* **deps:** Bump github.com/cloudflare/circl from 1.6.1 to 1.6.3 ([#30](https://github.com/fitbeard/terraform-provider-radosgw/issues/30)) ([886ecfd](https://github.com/fitbeard/terraform-provider-radosgw/commit/886ecfd3aa65c60635270d658a178f7d5b964255))
* **deps:** Bump github.com/hashicorp/terraform-plugin-framework ([#2](https://github.com/fitbeard/terraform-provider-radosgw/issues/2)) ([c2e67f4](https://github.com/fitbeard/terraform-provider-radosgw/commit/c2e67f40daea2ce4e1d2736154ab8c8b88c3b9fb))
* **deps:** Bump github.com/hashicorp/terraform-plugin-framework ([#34](https://github.com/fitbeard/terraform-provider-radosgw/issues/34)) ([0f2f7f0](https://github.com/fitbeard/terraform-provider-radosgw/commit/0f2f7f0a318efe46f30c70de75e26f549e0cb085))
* **deps:** Bump github.com/hashicorp/terraform-plugin-go ([#29](https://github.com/fitbeard/terraform-provider-radosgw/issues/29)) ([3c6cd8b](https://github.com/fitbeard/terraform-provider-radosgw/commit/3c6cd8b4fa573365cf638932b2b66615dc1004b2))
* **deps:** Bump github.com/hashicorp/terraform-plugin-log ([#113](https://github.com/fitbeard/terraform-provider-radosgw/issues/113)) ([bbf504a](https://github.com/fitbeard/terraform-provider-radosgw/commit/bbf504a338d31ea8f83ff1a7a220b42cf66c3d59))
* **deps:** Bump github.com/hashicorp/terraform-plugin-sdk/v2 ([#3](https://github.com/fitbeard/terraform-provider-radosgw/issues/3)) ([bf15e90](https://github.com/fitbeard/terraform-provider-radosgw/commit/bf15e909f973b1b41a4140248cf43161e37d3937))
* **deps:** Bump github.com/hashicorp/terraform-plugin-sdk/v2 ([#33](https://github.com/fitbeard/terraform-provider-radosgw/issues/33)) ([f72b55b](https://github.com/fitbeard/terraform-provider-radosgw/commit/f72b55b2d285b11aa502ff719def022e0319d1d7))
* **deps:** Bump github.com/hashicorp/terraform-plugin-testing ([#40](https://github.com/fitbeard/terraform-provider-radosgw/issues/40)) ([bc3fc4d](https://github.com/fitbeard/terraform-provider-radosgw/commit/bc3fc4d4f08ad0e4fe668eddac9f8197ebf370f7))
* **deps:** Bump golang.org/x/crypto from 0.51.0 to 0.52.0 ([#89](https://github.com/fitbeard/terraform-provider-radosgw/issues/89)) ([e4276d0](https://github.com/fitbeard/terraform-provider-radosgw/commit/e4276d0e1197a4184dd87e06722684eb7c5faefd))
* **deps:** Bump golang.org/x/net from 0.52.0 to 0.55.0 ([#90](https://github.com/fitbeard/terraform-provider-radosgw/issues/90)) ([411784f](https://github.com/fitbeard/terraform-provider-radosgw/commit/411784f141ca201b71a0256723390d0f18f95d72))
* **deps:** Bump google.golang.org/grpc from 1.82.1 to 1.83.1 ([#124](https://github.com/fitbeard/terraform-provider-radosgw/issues/124)) ([46bf93a](https://github.com/fitbeard/terraform-provider-radosgw/commit/46bf93a368ae34eed67ff5fa1975be8093d67d51))
* **deps:** Bump googleapis/release-please-action from 4 to 5 ([#63](https://github.com/fitbeard/terraform-provider-radosgw/issues/63)) ([a39fc5e](https://github.com/fitbeard/terraform-provider-radosgw/commit/a39fc5ea1b96ad77cb2496c5ad035dbced19a923))
* **deps:** Bump goreleaser/goreleaser-action from 6 to 7 ([#26](https://github.com/fitbeard/terraform-provider-radosgw/issues/26)) ([2da4476](https://github.com/fitbeard/terraform-provider-radosgw/commit/2da4476e652154524e9e15262f0e91fbf518dc32))
* **deps:** Bump hashicorp/setup-terraform from 3 to 4 ([#27](https://github.com/fitbeard/terraform-provider-radosgw/issues/27)) ([b78faaf](https://github.com/fitbeard/terraform-provider-radosgw/commit/b78faafeaf81a8ad73c0c1e002a41b69d920d59c))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#100](https://github.com/fitbeard/terraform-provider-radosgw/issues/100)) ([f2247c2](https://github.com/fitbeard/terraform-provider-radosgw/commit/f2247c26829e91f5e626a3f38a0a41b418a225b9))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#116](https://github.com/fitbeard/terraform-provider-radosgw/issues/116)) ([3db9021](https://github.com/fitbeard/terraform-provider-radosgw/commit/3db902144aa98328d5587368ba3691367bf15b11))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#68](https://github.com/fitbeard/terraform-provider-radosgw/issues/68)) ([ce2af75](https://github.com/fitbeard/terraform-provider-radosgw/commit/ce2af7547192e8cf533ae71f0d76a67ff50348b6))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#88](https://github.com/fitbeard/terraform-provider-radosgw/issues/88)) ([3e2a4b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/3e2a4b7be074a2700eb33d6a1233e220460596d3))
* **deps:** Bump the aws-sdk group with 3 updates ([#112](https://github.com/fitbeard/terraform-provider-radosgw/issues/112)) ([0c470e2](https://github.com/fitbeard/terraform-provider-radosgw/commit/0c470e2b95742fb891dacf71acf13edc1afedc04))
* **deps:** Bump the aws-sdk group with 3 updates ([#122](https://github.com/fitbeard/terraform-provider-radosgw/issues/122)) ([fa9def1](https://github.com/fitbeard/terraform-provider-radosgw/commit/fa9def1bcdd4fc7dd7c25db52ba087b3d9084100))
* **deps:** Bump the aws-sdk group with 3 updates ([#57](https://github.com/fitbeard/terraform-provider-radosgw/issues/57)) ([bb0d8b4](https://github.com/fitbeard/terraform-provider-radosgw/commit/bb0d8b438a637e63d01c7b426a4284306a17541c))
* **deps:** Bump the aws-sdk group with 3 updates ([#59](https://github.com/fitbeard/terraform-provider-radosgw/issues/59)) ([a4f1529](https://github.com/fitbeard/terraform-provider-radosgw/commit/a4f152987de209ec284383b22012fa75f3c672c3))
* **deps:** Bump the aws-sdk group with 3 updates ([#76](https://github.com/fitbeard/terraform-provider-radosgw/issues/76)) ([4b6abb9](https://github.com/fitbeard/terraform-provider-radosgw/commit/4b6abb9ab4fe0ac5791c92ccd547e0ee80a44fdd))
* **deps:** Bump the aws-sdk group with 3 updates ([#78](https://github.com/fitbeard/terraform-provider-radosgw/issues/78)) ([0acaf24](https://github.com/fitbeard/terraform-provider-radosgw/commit/0acaf24ea76d37c7216524e343f7b59050b2ba6e))
* **deps:** Bump the aws-sdk group with 3 updates ([#80](https://github.com/fitbeard/terraform-provider-radosgw/issues/80)) ([3773feb](https://github.com/fitbeard/terraform-provider-radosgw/commit/3773feb094746df0927252e976b7725b3c9c39dc))
* **deps:** Bump the terraform group across 1 directory with 3 updates ([#67](https://github.com/fitbeard/terraform-provider-radosgw/issues/67)) ([14c4dbf](https://github.com/fitbeard/terraform-provider-radosgw/commit/14c4dbf48bcc8b27d5e8b8140d12b0a3fb780e73))
* group dependabot changes ([#56](https://github.com/fitbeard/terraform-provider-radosgw/issues/56)) ([64d3f34](https://github.com/fitbeard/terraform-provider-radosgw/commit/64d3f342a32c61ffe12276145df187d3c09acc38))
* **main:** release 0.2.0 ([#7](https://github.com/fitbeard/terraform-provider-radosgw/issues/7)) ([732728f](https://github.com/fitbeard/terraform-provider-radosgw/commit/732728f66bdaa76e12b905bd47e7119a7a80621c))
* **main:** release 1.1.0 ([#14](https://github.com/fitbeard/terraform-provider-radosgw/issues/14)) ([3cf6f34](https://github.com/fitbeard/terraform-provider-radosgw/commit/3cf6f34dc3f4ff645401a19670c80c7ba8371957))
* **main:** release 1.2.0 ([#19](https://github.com/fitbeard/terraform-provider-radosgw/issues/19)) ([173bef2](https://github.com/fitbeard/terraform-provider-radosgw/commit/173bef21f6c3a8ec5345232b33deeca070e4e62c))
* **main:** release 1.2.1 ([#25](https://github.com/fitbeard/terraform-provider-radosgw/issues/25)) ([e09ef22](https://github.com/fitbeard/terraform-provider-radosgw/commit/e09ef2248354758de1b2cc4f6d602a6d6de3c87e))
* **main:** release 1.2.2 ([#48](https://github.com/fitbeard/terraform-provider-radosgw/issues/48)) ([c713463](https://github.com/fitbeard/terraform-provider-radosgw/commit/c7134635e952afd94ae1f49bed764dbaab95be35))
* **main:** release 1.2.3 ([#58](https://github.com/fitbeard/terraform-provider-radosgw/issues/58)) ([60a6900](https://github.com/fitbeard/terraform-provider-radosgw/commit/60a6900aec730f2164277a033fda42660b9a8ea5))
* **main:** release 1.3.0 ([#72](https://github.com/fitbeard/terraform-provider-radosgw/issues/72)) ([fe169e5](https://github.com/fitbeard/terraform-provider-radosgw/commit/fe169e5fcbf748c18b702a69ce963f33f947b070))
* **main:** release 1.4.0 ([#75](https://github.com/fitbeard/terraform-provider-radosgw/issues/75)) ([f831375](https://github.com/fitbeard/terraform-provider-radosgw/commit/f831375b982c8d71cec11355419f4e0ac20dd8f7))
* **main:** release 1.4.1 ([#87](https://github.com/fitbeard/terraform-provider-radosgw/issues/87)) ([283eaed](https://github.com/fitbeard/terraform-provider-radosgw/commit/283eaed814bbc8a3c92cf08ec11a274e216cf30c))
* **main:** release 1.4.2 ([#92](https://github.com/fitbeard/terraform-provider-radosgw/issues/92)) ([947a47b](https://github.com/fitbeard/terraform-provider-radosgw/commit/947a47b89fdb668ecc6a560fe01309a4d19e1103))
* **main:** release 1.5.0 ([#99](https://github.com/fitbeard/terraform-provider-radosgw/issues/99)) ([eea6bf3](https://github.com/fitbeard/terraform-provider-radosgw/commit/eea6bf3ad5a06967fa09a295082e4c55a0c32dcb))
* **main:** release 1.6.0 ([#107](https://github.com/fitbeard/terraform-provider-radosgw/issues/107)) ([6ab6c5f](https://github.com/fitbeard/terraform-provider-radosgw/commit/6ab6c5fb6e8415bf92dafbf2fd1b6c4e8c4d052d))
* **main:** release 1.6.1 ([#111](https://github.com/fitbeard/terraform-provider-radosgw/issues/111)) ([f5dd030](https://github.com/fitbeard/terraform-provider-radosgw/commit/f5dd03040beea7509c2d3ce2ef1ebb4f4497be00))
* **main:** release 1.6.2 ([#115](https://github.com/fitbeard/terraform-provider-radosgw/issues/115)) ([c4c1e36](https://github.com/fitbeard/terraform-provider-radosgw/commit/c4c1e36611c37d1e748d68ff00b705f6741e947b))
* update go-ceph to 0.38.0 ([#17](https://github.com/fitbeard/terraform-provider-radosgw/issues/17)) ([04edb25](https://github.com/fitbeard/terraform-provider-radosgw/commit/04edb25209cb5ae392eb72999472eff999c3113b))
* update release-please config ([#18](https://github.com/fitbeard/terraform-provider-radosgw/issues/18)) ([1f8c6de](https://github.com/fitbeard/terraform-provider-radosgw/commit/1f8c6de7585832497c3e4400a0bb937fa14e6a79))

## [1.6.2](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.6.1...v1.6.2) (2026-09-02)


### Bug Fixes

* bucket versioning check during planning ([#125](https://github.com/fitbeard/terraform-provider-radosgw/issues/125)) ([4dae3b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/4dae3b7a0190509a2e9b5c9d440932339fbcbc17))


### Miscellaneous

* **deps:** Bump github.com/ceph/go-ceph from 0.40.0 to 0.41.0 ([#117](https://github.com/fitbeard/terraform-provider-radosgw/issues/117)) ([b38bd0d](https://github.com/fitbeard/terraform-provider-radosgw/commit/b38bd0d8edc7f3c05526053e17e24a5e84a47a16))
* **deps:** Bump google.golang.org/grpc from 1.82.1 to 1.83.1 ([#124](https://github.com/fitbeard/terraform-provider-radosgw/issues/124)) ([46bf93a](https://github.com/fitbeard/terraform-provider-radosgw/commit/46bf93a368ae34eed67ff5fa1975be8093d67d51))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#116](https://github.com/fitbeard/terraform-provider-radosgw/issues/116)) ([3db9021](https://github.com/fitbeard/terraform-provider-radosgw/commit/3db902144aa98328d5587368ba3691367bf15b11))
* **deps:** Bump the aws-sdk group with 3 updates ([#122](https://github.com/fitbeard/terraform-provider-radosgw/issues/122)) ([fa9def1](https://github.com/fitbeard/terraform-provider-radosgw/commit/fa9def1bcdd4fc7dd7c25db52ba087b3d9084100))

## [1.6.1](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.6.0...v1.6.1) (2026-08-11)


### Bug Fixes

* ordering for bucket cors settings ([#114](https://github.com/fitbeard/terraform-provider-radosgw/issues/114)) ([ee63f7e](https://github.com/fitbeard/terraform-provider-radosgw/commit/ee63f7e16724e938a6c5cd21cbe17572c67fbe6c))


### Miscellaneous

* **deps:** Bump github.com/hashicorp/terraform-plugin-log ([#113](https://github.com/fitbeard/terraform-provider-radosgw/issues/113)) ([bbf504a](https://github.com/fitbeard/terraform-provider-radosgw/commit/bbf504a338d31ea8f83ff1a7a220b42cf66c3d59))
* **deps:** Bump the aws-sdk group with 3 updates ([#112](https://github.com/fitbeard/terraform-provider-radosgw/issues/112)) ([0c470e2](https://github.com/fitbeard/terraform-provider-radosgw/commit/0c470e2b95742fb891dacf71acf13edc1afedc04))

## [1.6.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.5.0...v1.6.0) (2026-08-08)


### Features

* add resources and datasources for iam_account users and access_keys ([#109](https://github.com/fitbeard/terraform-provider-radosgw/issues/109)) ([61b62cd](https://github.com/fitbeard/terraform-provider-radosgw/commit/61b62cd760129ffe685a66df24fa89530b1ecc0a))
* add resources for account users, groups and policies ([#110](https://github.com/fitbeard/terraform-provider-radosgw/issues/110)) ([d44c863](https://github.com/fitbeard/terraform-provider-radosgw/commit/d44c8631ee341966e92dc8d517f21c8a4b5c5075))

## [1.5.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.4.2...v1.5.0) (2026-08-04)


### Features

* accept provider credentials unknown until apply ([#103](https://github.com/fitbeard/terraform-provider-radosgw/issues/103)) ([fde4cf5](https://github.com/fitbeard/terraform-provider-radosgw/commit/fde4cf5bc6f5480743f146e031d4c04e6d888554))
* add radosgw_s3_bucket_cors_configuration resource ([#106](https://github.com/fitbeard/terraform-provider-radosgw/issues/106)) ([c539372](https://github.com/fitbeard/terraform-provider-radosgw/commit/c539372e64d1d9c0d2802ef685054df11cbefdf2))
* move to golang 1.26 ([#104](https://github.com/fitbeard/terraform-provider-radosgw/issues/104)) ([7b1b914](https://github.com/fitbeard/terraform-provider-radosgw/commit/7b1b9140c6929c3067c2965e84da6bc20dcf23d8))


### Bug Fixes

* linking for tenant owned buckets ([#105](https://github.com/fitbeard/terraform-provider-radosgw/issues/105)) ([7482722](https://github.com/fitbeard/terraform-provider-radosgw/commit/7482722ca93dc121fdf17cf57defd264e6136f6f))


### Miscellaneous

* **deps:** Bump actions/setup-go from 6 to 7 ([#95](https://github.com/fitbeard/terraform-provider-radosgw/issues/95)) ([1edd12b](https://github.com/fitbeard/terraform-provider-radosgw/commit/1edd12bc7671103b70e0dee8121af082dd6184ba))
* **deps:** Bump github.com/aws/smithy-go from 1.27.5 to 1.27.6 ([#102](https://github.com/fitbeard/terraform-provider-radosgw/issues/102)) ([0289be6](https://github.com/fitbeard/terraform-provider-radosgw/commit/0289be6d87364793b147a26fc10e8fb987fe7a4d))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#100](https://github.com/fitbeard/terraform-provider-radosgw/issues/100)) ([f2247c2](https://github.com/fitbeard/terraform-provider-radosgw/commit/f2247c26829e91f5e626a3f38a0a41b418a225b9))

## [1.4.2](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.4.1...v1.4.2) (2026-07-22)


### Bug Fixes

* correct account limit defaults and support capless/federated & tenant users ([#98](https://github.com/fitbeard/terraform-provider-radosgw/issues/98)) ([2d603cf](https://github.com/fitbeard/terraform-provider-radosgw/commit/2d603cf34b0ac2bd987cf0c16a34e5f00378327e))

## [1.4.1](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.4.0...v1.4.1) (2026-07-14)


### Bug Fixes

* retry transient SNS/notification errors under concurrent topic churn ([#91](https://github.com/fitbeard/terraform-provider-radosgw/issues/91)) ([fd49b0a](https://github.com/fitbeard/terraform-provider-radosgw/commit/fd49b0a0e45f58b86032e4512094d9160873b9ec))


### Miscellaneous

* **deps:** Bump golang.org/x/crypto from 0.51.0 to 0.52.0 ([#89](https://github.com/fitbeard/terraform-provider-radosgw/issues/89)) ([e4276d0](https://github.com/fitbeard/terraform-provider-radosgw/commit/e4276d0e1197a4184dd87e06722684eb7c5faefd))
* **deps:** Bump golang.org/x/net from 0.52.0 to 0.55.0 ([#90](https://github.com/fitbeard/terraform-provider-radosgw/issues/90)) ([411784f](https://github.com/fitbeard/terraform-provider-radosgw/commit/411784f141ca201b71a0256723390d0f18f95d72))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#88](https://github.com/fitbeard/terraform-provider-radosgw/issues/88)) ([3e2a4b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/3e2a4b7be074a2700eb33d6a1233e220460596d3))

## [1.4.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.3.0...v1.4.0) (2026-07-06)


### Features

* add initial account support ([#86](https://github.com/fitbeard/terraform-provider-radosgw/issues/86)) ([4ab676e](https://github.com/fitbeard/terraform-provider-radosgw/commit/4ab676ed1253cf956570a66df4a5fb5903271622))


### Miscellaneous

* **deps:** Bump actions/checkout from 6 to 7 ([#81](https://github.com/fitbeard/terraform-provider-radosgw/issues/81)) ([b68ecde](https://github.com/fitbeard/terraform-provider-radosgw/commit/b68ecdef6aa943671b762db12e0635c42ab81c3b))
* **deps:** Bump github.com/aws/smithy-go from 1.27.1 to 1.27.3 ([#84](https://github.com/fitbeard/terraform-provider-radosgw/issues/84)) ([679a6a9](https://github.com/fitbeard/terraform-provider-radosgw/commit/679a6a9bde9c9217ce6cdf3ecf0c82290942102d))
* **deps:** Bump github.com/ceph/go-ceph from 0.39.0 to 0.40.0 ([#82](https://github.com/fitbeard/terraform-provider-radosgw/issues/82)) ([4c6a4ce](https://github.com/fitbeard/terraform-provider-radosgw/commit/4c6a4ceaeeb9347fe4b9b059d27568ff7ed8f498))
* **deps:** Bump the aws-sdk group with 3 updates ([#76](https://github.com/fitbeard/terraform-provider-radosgw/issues/76)) ([4b6abb9](https://github.com/fitbeard/terraform-provider-radosgw/commit/4b6abb9ab4fe0ac5791c92ccd547e0ee80a44fdd))
* **deps:** Bump the aws-sdk group with 3 updates ([#78](https://github.com/fitbeard/terraform-provider-radosgw/issues/78)) ([0acaf24](https://github.com/fitbeard/terraform-provider-radosgw/commit/0acaf24ea76d37c7216524e343f7b59050b2ba6e))
* **deps:** Bump the aws-sdk group with 3 updates ([#80](https://github.com/fitbeard/terraform-provider-radosgw/issues/80)) ([3773feb](https://github.com/fitbeard/terraform-provider-radosgw/commit/3773feb094746df0927252e976b7725b3c9c39dc))

## [1.3.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.2.3...v1.3.0) (2026-05-24)


### Features

* add tags for iam_role ([#74](https://github.com/fitbeard/terraform-provider-radosgw/issues/74)) ([7d6f8f0](https://github.com/fitbeard/terraform-provider-radosgw/commit/7d6f8f013a73ed6f0a43b6de8d2a7526616918df))


### Miscellaneous

* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#73](https://github.com/fitbeard/terraform-provider-radosgw/issues/73)) ([6962dbe](https://github.com/fitbeard/terraform-provider-radosgw/commit/6962dbe87739d63a6c26e0657a9cb71b4cdbf4e2))

## [1.2.3](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.2.2...v1.2.3) (2026-05-19)


### Bug Fixes

* bump hc-install to fix expired pgp key ([#62](https://github.com/fitbeard/terraform-provider-radosgw/issues/62)) ([c258303](https://github.com/fitbeard/terraform-provider-radosgw/commit/c258303d9ad418208b2a6d30baba4ac82092b9ac))
* correctly resolve both local and tenant user id ([#71](https://github.com/fitbeard/terraform-provider-radosgw/issues/71)) ([2f337ef](https://github.com/fitbeard/terraform-provider-radosgw/commit/2f337ef04395e5c3b56999e9368bdb662f32bd98))


### Miscellaneous

* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#69](https://github.com/fitbeard/terraform-provider-radosgw/issues/69)) ([5ab91ae](https://github.com/fitbeard/terraform-provider-radosgw/commit/5ab91ae1406798f47389f3eb67821650c970d835))
* **deps:** Bump github.com/aws/smithy-go from 1.25.0 to 1.25.1 ([#66](https://github.com/fitbeard/terraform-provider-radosgw/issues/66)) ([7e17ce6](https://github.com/fitbeard/terraform-provider-radosgw/commit/7e17ce6876fce50e1a528dbeaef5b699a3b680da))
* **deps:** Bump github.com/ceph/go-ceph from 0.38.0 to 0.39.0 ([#60](https://github.com/fitbeard/terraform-provider-radosgw/issues/60)) ([aca5e97](https://github.com/fitbeard/terraform-provider-radosgw/commit/aca5e97db26bcf032eedd3850bb4f90ce235a720))
* **deps:** Bump googleapis/release-please-action from 4 to 5 ([#63](https://github.com/fitbeard/terraform-provider-radosgw/issues/63)) ([a39fc5e](https://github.com/fitbeard/terraform-provider-radosgw/commit/a39fc5ea1b96ad77cb2496c5ad035dbced19a923))
* **deps:** Bump the aws-sdk group across 1 directory with 3 updates ([#68](https://github.com/fitbeard/terraform-provider-radosgw/issues/68)) ([ce2af75](https://github.com/fitbeard/terraform-provider-radosgw/commit/ce2af7547192e8cf533ae71f0d76a67ff50348b6))
* **deps:** Bump the aws-sdk group with 3 updates ([#59](https://github.com/fitbeard/terraform-provider-radosgw/issues/59)) ([a4f1529](https://github.com/fitbeard/terraform-provider-radosgw/commit/a4f152987de209ec284383b22012fa75f3c672c3))
* **deps:** Bump the terraform group across 1 directory with 3 updates ([#67](https://github.com/fitbeard/terraform-provider-radosgw/issues/67)) ([14c4dbf](https://github.com/fitbeard/terraform-provider-radosgw/commit/14c4dbf48bcc8b27d5e8b8140d12b0a3fb780e73))

## [1.2.2](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.2.1...v1.2.2) (2026-04-14)


### Miscellaneous

* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#49](https://github.com/fitbeard/terraform-provider-radosgw/issues/49)) ([f0fa9b3](https://github.com/fitbeard/terraform-provider-radosgw/commit/f0fa9b376f595e253665f4df140c077ca4d8f2d7))
* **deps:** Bump github.com/aws/smithy-go from 1.24.2 to 1.24.3 ([#55](https://github.com/fitbeard/terraform-provider-radosgw/issues/55)) ([4b3b2c4](https://github.com/fitbeard/terraform-provider-radosgw/commit/4b3b2c483b8ad47145bb4c2e05422d5c6a62b863))
* **deps:** Bump the aws-sdk group with 3 updates ([#57](https://github.com/fitbeard/terraform-provider-radosgw/issues/57)) ([bb0d8b4](https://github.com/fitbeard/terraform-provider-radosgw/commit/bb0d8b438a637e63d01c7b426a4284306a17541c))
* group dependabot changes ([#56](https://github.com/fitbeard/terraform-provider-radosgw/issues/56)) ([64d3f34](https://github.com/fitbeard/terraform-provider-radosgw/commit/64d3f342a32c61ffe12276145df187d3c09acc38))

## [1.2.1](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.2.0...v1.2.1) (2026-03-22)


### Bug Fixes

* go transitive dependencies ([#41](https://github.com/fitbeard/terraform-provider-radosgw/issues/41)) ([f501665](https://github.com/fitbeard/terraform-provider-radosgw/commit/f501665316e0dfb7041627415dc014b8ac78e9b9))
* tests for openid_connect_provider for Reef ([#47](https://github.com/fitbeard/terraform-provider-radosgw/issues/47)) ([79c9371](https://github.com/fitbeard/terraform-provider-radosgw/commit/79c9371c7479016e4af9fd2b3f0b34ea9ac6c92b))


### Miscellaneous

* **deps:** Bump crazy-max/ghaction-import-gpg from 6.3.0 to 7.0.0 ([#31](https://github.com/fitbeard/terraform-provider-radosgw/issues/31)) ([14523ca](https://github.com/fitbeard/terraform-provider-radosgw/commit/14523caa800d46b5b64606e66aaae3f37ff01657))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#35](https://github.com/fitbeard/terraform-provider-radosgw/issues/35)) ([ba60c46](https://github.com/fitbeard/terraform-provider-radosgw/commit/ba60c46190de6f14957b5c69bcbc927cdc19c6b9))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#44](https://github.com/fitbeard/terraform-provider-radosgw/issues/44)) ([02747b5](https://github.com/fitbeard/terraform-provider-radosgw/commit/02747b5368cd8d6be579ab057ae0b360225e5b1c))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#28](https://github.com/fitbeard/terraform-provider-radosgw/issues/28)) ([fb88338](https://github.com/fitbeard/terraform-provider-radosgw/commit/fb883385d5fce46baa937761ce5ee48aab55654c))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#32](https://github.com/fitbeard/terraform-provider-radosgw/issues/32)) ([1094041](https://github.com/fitbeard/terraform-provider-radosgw/commit/10940414bc26f32c270d24a89ae57a10726f71b0))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#42](https://github.com/fitbeard/terraform-provider-radosgw/issues/42)) ([e93491a](https://github.com/fitbeard/terraform-provider-radosgw/commit/e93491a9d6249127b71d4bcc1372fd70498e772e))
* **deps:** Bump github.com/cloudflare/circl from 1.6.1 to 1.6.3 ([#30](https://github.com/fitbeard/terraform-provider-radosgw/issues/30)) ([886ecfd](https://github.com/fitbeard/terraform-provider-radosgw/commit/886ecfd3aa65c60635270d658a178f7d5b964255))
* **deps:** Bump github.com/hashicorp/terraform-plugin-framework ([#34](https://github.com/fitbeard/terraform-provider-radosgw/issues/34)) ([0f2f7f0](https://github.com/fitbeard/terraform-provider-radosgw/commit/0f2f7f0a318efe46f30c70de75e26f549e0cb085))
* **deps:** Bump github.com/hashicorp/terraform-plugin-go ([#29](https://github.com/fitbeard/terraform-provider-radosgw/issues/29)) ([3c6cd8b](https://github.com/fitbeard/terraform-provider-radosgw/commit/3c6cd8b4fa573365cf638932b2b66615dc1004b2))
* **deps:** Bump github.com/hashicorp/terraform-plugin-sdk/v2 ([#33](https://github.com/fitbeard/terraform-provider-radosgw/issues/33)) ([f72b55b](https://github.com/fitbeard/terraform-provider-radosgw/commit/f72b55b2d285b11aa502ff719def022e0319d1d7))
* **deps:** Bump github.com/hashicorp/terraform-plugin-testing ([#40](https://github.com/fitbeard/terraform-provider-radosgw/issues/40)) ([bc3fc4d](https://github.com/fitbeard/terraform-provider-radosgw/commit/bc3fc4d4f08ad0e4fe668eddac9f8197ebf370f7))
* **deps:** Bump goreleaser/goreleaser-action from 6 to 7 ([#26](https://github.com/fitbeard/terraform-provider-radosgw/issues/26)) ([2da4476](https://github.com/fitbeard/terraform-provider-radosgw/commit/2da4476e652154524e9e15262f0e91fbf518dc32))
* **deps:** Bump hashicorp/setup-terraform from 3 to 4 ([#27](https://github.com/fitbeard/terraform-provider-radosgw/issues/27)) ([b78faaf](https://github.com/fitbeard/terraform-provider-radosgw/commit/b78faafeaf81a8ad73c0c1e002a41b69d920d59c))

## [1.2.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.1.0...v1.2.0) (2026-02-24)


### Features

* add bucket notification resources ([#23](https://github.com/fitbeard/terraform-provider-radosgw/issues/23)) ([d417f80](https://github.com/fitbeard/terraform-provider-radosgw/commit/d417f80770c7373fd860a5e77c00b25ecc8dac84))
* add radosgw_s3_bucket_website_configuration resource ([#24](https://github.com/fitbeard/terraform-provider-radosgw/issues/24)) ([6dd8e29](https://github.com/fitbeard/terraform-provider-radosgw/commit/6dd8e29cdbefdc02e773da0c3b66949762f86746))


### Bug Fixes

* make install command ([#20](https://github.com/fitbeard/terraform-provider-radosgw/issues/20)) ([4f3ec06](https://github.com/fitbeard/terraform-provider-radosgw/commit/4f3ec06578e8b0eff79152f112cffba0cc19e42c))


### Miscellaneous

* **deps:** Bump github.com/aws/aws-sdk-go-v2/credentials ([#22](https://github.com/fitbeard/terraform-provider-radosgw/issues/22)) ([80bf3df](https://github.com/fitbeard/terraform-provider-radosgw/commit/80bf3df0e9b59142911f06be9fe408a3dbdd9f07))
* **deps:** Bump github.com/aws/aws-sdk-go-v2/service/s3 ([#4](https://github.com/fitbeard/terraform-provider-radosgw/issues/4)) ([9f026f2](https://github.com/fitbeard/terraform-provider-radosgw/commit/9f026f2df94c2ecfae5e22043660533abdd6890e))
* **deps:** Bump github.com/hashicorp/terraform-plugin-sdk/v2 ([#3](https://github.com/fitbeard/terraform-provider-radosgw/issues/3)) ([bf15e90](https://github.com/fitbeard/terraform-provider-radosgw/commit/bf15e909f973b1b41a4140248cf43161e37d3937))
* update go-ceph to 0.38.0 ([#17](https://github.com/fitbeard/terraform-provider-radosgw/issues/17)) ([04edb25](https://github.com/fitbeard/terraform-provider-radosgw/commit/04edb25209cb5ae392eb72999472eff999c3113b))
* update release-please config ([#18](https://github.com/fitbeard/terraform-provider-radosgw/issues/18)) ([1f8c6de](https://github.com/fitbeard/terraform-provider-radosgw/commit/1f8c6de7585832497c3e4400a0bb937fa14e6a79))

## [1.1.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v1.0.0...v1.1.0) (2026-02-05)


### Features

* add release tools ([#1](https://github.com/fitbeard/terraform-provider-radosgw/issues/1)) ([31d774b](https://github.com/fitbeard/terraform-provider-radosgw/commit/31d774b86ea308cd05a4b473d7f26ee042344f63))
* first major release ([62dc421](https://github.com/fitbeard/terraform-provider-radosgw/commit/62dc421b695d915e1bd8b1ed0a2e6b151d1ab2d9))
* initial publishing ([523e82f](https://github.com/fitbeard/terraform-provider-radosgw/commit/523e82fa2e5e41e0405330fe8db591cfbe7b2fe3))
* initial release config ([ea649ad](https://github.com/fitbeard/terraform-provider-radosgw/commit/ea649ade356ac93c60d6987f79fca573bbe09a30))


### Bug Fixes

* documentation typos ([#12](https://github.com/fitbeard/terraform-provider-radosgw/issues/12)) ([c50b67e](https://github.com/fitbeard/terraform-provider-radosgw/commit/c50b67ef91ccefc08dc4213bf1100244dd6ea37c))
* remove prerelease from goreleaser ([#11](https://github.com/fitbeard/terraform-provider-radosgw/issues/11)) ([27df5be](https://github.com/fitbeard/terraform-provider-radosgw/commit/27df5beeb2a269a222cb7d5f294b12a3ec04308e))


### Documentation

* add README ([f3009b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/f3009b7ab60b39710eedcbbd42b88d7e579bbc34))

## [0.2.0](https://github.com/fitbeard/terraform-provider-radosgw/compare/v0.1.0...v0.2.0) (2026-02-05)


### Features

* add release tools ([#1](https://github.com/fitbeard/terraform-provider-radosgw/issues/1)) ([31d774b](https://github.com/fitbeard/terraform-provider-radosgw/commit/31d774b86ea308cd05a4b473d7f26ee042344f63))
* initial publishing ([523e82f](https://github.com/fitbeard/terraform-provider-radosgw/commit/523e82fa2e5e41e0405330fe8db591cfbe7b2fe3))
* initial release config ([ea649ad](https://github.com/fitbeard/terraform-provider-radosgw/commit/ea649ade356ac93c60d6987f79fca573bbe09a30))


### Documentation

* add README ([f3009b7](https://github.com/fitbeard/terraform-provider-radosgw/commit/f3009b7ab60b39710eedcbbd42b88d7e579bbc34))
