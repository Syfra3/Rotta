# Changelog

## [1.8.0](https://github.com/Syfra3/Rotta/compare/v1.7.0...v1.8.0) (2026-07-14)


### Features

* archive terminal workflow submissions ([ebba1d8](https://github.com/Syfra3/Rotta/commit/ebba1d8a8f613c5d892cfa52904230151945d819))
* initialize scoped workflow submissions ([cfa10f2](https://github.com/Syfra3/Rotta/commit/cfa10f2d53bc6f83bb85b7adf9a78222c08b8566))
* isolate agent installation rollback ([9894639](https://github.com/Syfra3/Rotta/commit/9894639a835b0afee7317a5f499144df11da255f))
* isolate concurrent submissions ([92ffdad](https://github.com/Syfra3/Rotta/commit/92ffdad405e0e55c390d94e435edc5b241afc47a))
* isolate Rotta submissions in feature worktrees ([55e5bc7](https://github.com/Syfra3/Rotta/commit/55e5bc78fd74f2ab1ff71ff30cbbaaee113d2731))
* prepare isolated feature worktree ([afa3f4f](https://github.com/Syfra3/Rotta/commit/afa3f4f02e7d33748a9e0e73b6f16d0e6d329506))
* preserve ambiguous MCP entries ([dd8decd](https://github.com/Syfra3/Rotta/commit/dd8decd82e1af65a7c72e55f01a61d887f3876b7))
* preserve failed manual pull request handoffs ([ad0e840](https://github.com/Syfra3/Rotta/commit/ad0e840e7bbdec1c623ec987b7ab1db068aab1f0))
* preserve MCP config on Vela setup failure ([396e06f](https://github.com/Syfra3/Rotta/commit/396e06fa3fa4343b77d3793e5aaa0902c8183d14))
* record compact workflow lifecycle state ([2d4b564](https://github.com/Syfra3/Rotta/commit/2d4b564430e352c67e040048e482a8f941d0cb0a))
* reject unusable workflow submission state ([5eb22c7](https://github.com/Syfra3/Rotta/commit/5eb22c7b70718f5068836cf3339516ef4c5afc16))
* render manual pull request handoff ([512f7e7](https://github.com/Syfra3/Rotta/commit/512f7e7d12f40983199ee3467b5ae2f9ce14d676))
* report host MCP command failures ([0af5f82](https://github.com/Syfra3/Rotta/commit/0af5f82e87f7a4136020fa7a6246af38bcfaf52d))
* report unavailable MCP commands ([3444d83](https://github.com/Syfra3/Rotta/commit/3444d837fc74e7fbe335e49e1d2ba8fbb21770ed))
* report unverified OpenCode MCP PATH ([0902496](https://github.com/Syfra3/Rotta/commit/09024968f133c1fc7693add23bf630da3c660acd))
* report Vela MCP normalization ([5bad0f8](https://github.com/Syfra3/Rotta/commit/5bad0f847227dfb6ee5a82228ab4453f7cf0aa4e))
* restore partial agent configuration ([ebe7f77](https://github.com/Syfra3/Rotta/commit/ebe7f7767e327096ec417841976db66fa7a31126))
* resume scoped workflow submissions ([cfdba62](https://github.com/Syfra3/Rotta/commit/cfdba620440dccd8e775a79c08b4efc5a9f50e29))
* retain and clean workflow archives ([ec77974](https://github.com/Syfra3/Rotta/commit/ec77974602968ee02c012606ed887c1f4f7312e8))
* scope workflow review to manifest ([f7f5b93](https://github.com/Syfra3/Rotta/commit/f7f5b938d989f3705214a952cde1e521a85f7edc))
* skip unavailable new Vela MCP config ([7e89ab2](https://github.com/Syfra3/Rotta/commit/7e89ab20ae5c744b11ddb406f11e484f73867785))
* validate feature submission branches ([aadd140](https://github.com/Syfra3/Rotta/commit/aadd140252f5e4ce073f18887878cc9642dcbdb4))
* validate phase three worktree identity ([c63ef16](https://github.com/Syfra3/Rotta/commit/c63ef168c5ac01d171a59f769047219f8c954206))
* **workflow:** add resilient MCP fallbacks ([2bc3180](https://github.com/Syfra3/Rotta/commit/2bc318002cb0daefd4c229e17427af82708725ad))


### Bug Fixes

* reject colliding submission worktrees ([c492cb5](https://github.com/Syfra3/Rotta/commit/c492cb5704bd4bf59320b66cadf203de06651051))
* reject unsafe submission worktrees ([b6d114f](https://github.com/Syfra3/Rotta/commit/b6d114fb5a9f7abdac690735ae6e04de2d748e04))
* require explicit pull request remote ([b5693e0](https://github.com/Syfra3/Rotta/commit/b5693e0a7a12bafce8821b7d89c1bcbf48071a30))
* restrict scenario checkpoints to feature worktrees ([1377551](https://github.com/Syfra3/Rotta/commit/13775518034c7bba0d83cffdb764594ce488a8ff))
* serialize portable MCP commands ([3817d5b](https://github.com/Syfra3/Rotta/commit/3817d5b1943e299700f2da9ab35373dacb01b643))
* validate autonomous workflow inputs ([d251f71](https://github.com/Syfra3/Rotta/commit/d251f7156fc7d7bba57aaeb9d8009964fac3547d))


### Documentation

* approve MCP and lifecycle contracts ([667687e](https://github.com/Syfra3/Rotta/commit/667687e0b9fc63be88032ac5ed25b1596fe0953c))
* define isolated worktree handoff ([e0a8162](https://github.com/Syfra3/Rotta/commit/e0a8162a3ba9c4ad84fbfbe5d85e0a8c3f959ee0))
* specify portable MCP commands ([8f432b7](https://github.com/Syfra3/Rotta/commit/8f432b71c77fe3092a3b5560909a12360126d1d4))
* specify TDD evidence recovery ([4fdb7ba](https://github.com/Syfra3/Rotta/commit/4fdb7ba219fe039bbd283838b21137070d9b1b83))


### Code Refactoring

* **test:** split Vela guard reinstall case ([f9bbced](https://github.com/Syfra3/Rotta/commit/f9bbcede9e9e2600b313ac86bc1f74c5a98fe936))

## [1.7.0](https://github.com/Syfra3/Rotta/compare/v1.6.2...v1.7.0) (2026-07-13)


### Features

* **workflow:** add MCP fallbacks and autonomous TDD checkpoints ([e05917a](https://github.com/Syfra3/Rotta/commit/e05917ab37d8be963381d63a279b12560d1bd18c))

## [1.6.2](https://github.com/Syfra3/Rotta/compare/v1.6.1...v1.6.2) (2026-07-13)


### Bug Fixes

* **installer:** use OpenCode local MCP schema ([faab148](https://github.com/Syfra3/Rotta/commit/faab14899d9559bd953e917b2914fb8a7b8abb68))
* **installer:** use OpenCode local MCP schema ([7728f0c](https://github.com/Syfra3/Rotta/commit/7728f0cb0d110dbfb8b5b84c47cbcd7e359fdca8))

## [1.6.1](https://github.com/Syfra3/Rotta/compare/v1.6.0...v1.6.1) (2026-07-06)


### Bug Fixes

* **workflow:** enforce clean TDD task boundaries ([a3459bc](https://github.com/Syfra3/Rotta/commit/a3459bc5ca84341a8bec2431b012d83a6773fe88))

## [1.6.0](https://github.com/Syfra3/Rotta/compare/v1.5.0...v1.6.0) (2026-07-06)


### Features

* **installer:** adapt command capability reporting ([b1518a0](https://github.com/Syfra3/Rotta/commit/b1518a0f549e5fdbe04e139d052b88119aab4e76))
* **installer:** add all-host install results ([04568f6](https://github.com/Syfra3/Rotta/commit/04568f679e9caefff8fddad92125bf02ba7a4a67))
* **installer:** add codex host target ([089ed5e](https://github.com/Syfra3/Rotta/commit/089ed5ea3f2cc2637a87f6f1096fad8fe5d87f2c))
* **installer:** add Context7 and host compatibility ([bf80f1f](https://github.com/Syfra3/Rotta/commit/bf80f1f1bb37a835c66871d3b0de776f5de035f0))
* **installer:** add context7 mcp integration ([135233b](https://github.com/Syfra3/Rotta/commit/135233b7c17e25f0054e56075c81a91a2a10de73))
* **installer:** add host capability matrix ([dd3e706](https://github.com/Syfra3/Rotta/commit/dd3e706ca22e027baefcd93cdaa1fcdeb2b54cf3))
* **installer:** classify install changed files ([07763fb](https://github.com/Syfra3/Rotta/commit/07763fb44946749ab23a0aa6e4bcaf52658abd63))
* **installer:** configure selected host mcps ([d82c172](https://github.com/Syfra3/Rotta/commit/d82c172f4247cd22972f413b37e24469eeac290f))
* **installer:** disclose adapted host primitives ([73df4e8](https://github.com/Syfra3/Rotta/commit/73df4e8d767fd5cf5383b6dcd2858ebd99e28d3b))
* **installer:** document compact memory pointers ([7de5cc1](https://github.com/Syfra3/Rotta/commit/7de5cc1835ba36101f0e87f7a7441ddee2455da5))
* **installer:** make rerun summaries idempotent ([60585c9](https://github.com/Syfra3/Rotta/commit/60585c9add3171ccc36a914c962611c4213083ce))
* **installer:** preserve context7 when adding codex ([d84f9d0](https://github.com/Syfra3/Rotta/commit/d84f9d07a07d4a2c4c10c9c1f281ef7f52270804))
* **installer:** refuse malformed host config ([74973a3](https://github.com/Syfra3/Rotta/commit/74973a331438cf439af9729e3e8f8740f981d6dd))
* **installer:** report mcp capability degradation ([a746c7f](https://github.com/Syfra3/Rotta/commit/a746c7f9a6b8f83667b208fc9a97a62d1717c2bd))
* **installer:** report mcp health failures ([fae5e78](https://github.com/Syfra3/Rotta/commit/fae5e78858bac6f67a7ba6bc1ffdae4d344d1651))
* **installer:** report partial host recovery ([69514aa](https://github.com/Syfra3/Rotta/commit/69514aa662a2e019be31755c504e967cab297d34))
* **installer:** share canonical host instructions ([773a5be](https://github.com/Syfra3/Rotta/commit/773a5beed24b481fd9f78399958c2fe5144ce45c))
* **installer:** support cross-host workflow continuation ([6d74ad0](https://github.com/Syfra3/Rotta/commit/6d74ad04b8f4abee279e0f8079f24e0faf40b075))


### Bug Fixes

* **installer:** reject unsupported host targets ([c68ad55](https://github.com/Syfra3/Rotta/commit/c68ad556e7f9b16086d9e7a8119d7575fff698b9))

## [1.5.0](https://github.com/Syfra3/Rotta/compare/v1.4.0...v1.5.0) (2026-07-02)


### Features

* add installer backup and clean reinstall ([de6b648](https://github.com/Syfra3/Rotta/commit/de6b648cb886f66f67db7a4bf804c930893c9def))
* add installer recovery flow ([6dfec6e](https://github.com/Syfra3/Rotta/commit/6dfec6ee668d0dbdd902735ac9e958931c74bda0))
* add scoped workflow approval gate ([644e2ab](https://github.com/Syfra3/Rotta/commit/644e2ab32ed660fd38bc01f92795f62e4702ff94))
* archive retired workflow artifacts ([3d0ea2c](https://github.com/Syfra3/Rotta/commit/3d0ea2c91185394d2a2d319bd03a8816b6ac4378))
* classify tracked workflow contracts ([a8863c7](https://github.com/Syfra3/Rotta/commit/a8863c70196f712b9a9cbb53dcaec13bce4e9cde))
* enforce compact vela queries ([2d396d2](https://github.com/Syfra3/Rotta/commit/2d396d274615bad66e123410a7f4e315e43ecee1))
* enforce compact vela queries ([73b0eab](https://github.com/Syfra3/Rotta/commit/73b0eabf064ced7cd2c2e6c372ce520574b55f28))
* exclude local cache artifacts ([5e240cc](https://github.com/Syfra3/Rotta/commit/5e240ccf792187583630297ce81a13557c1ef575))
* implement workflow artifact lifecycle ([b76423f](https://github.com/Syfra3/Rotta/commit/b76423fa33c27a6be84d251182cdf7d3f991de72))
* install vela freshness guards ([f1ee63a](https://github.com/Syfra3/Rotta/commit/f1ee63a4be5bd48f4581789d8c22327a461f6c78))
* **installer:** add optional Vela integration ([5dc1617](https://github.com/Syfra3/Rotta/commit/5dc1617bda223e06f937b62f2d333be88c152740))
* **installer:** add optional Vela integration ([e8ed277](https://github.com/Syfra3/Rotta/commit/e8ed277f71c501efd637688d34b2fc440fb239b8))
* **installer:** add recovery backup restore ([f470a6d](https://github.com/Syfra3/Rotta/commit/f470a6d54db9846800b5b04a2272c25c86b8c4ca))
* keep implemented features active ([3f97141](https://github.com/Syfra3/Rotta/commit/3f97141f70bbf7e4449a85a5fe4d9cb4adfac4d7))
* plan approved repository scenarios ([48bc39e](https://github.com/Syfra3/Rotta/commit/48bc39e05d83f1f35ba4719b700ed193625ce9d5))
* protect namespaced workflow artifacts ([24d5fca](https://github.com/Syfra3/Rotta/commit/24d5fca0f28a401a9d7f4b301359518352803c19))
* protect untracked active contracts ([96adcd2](https://github.com/Syfra3/Rotta/commit/96adcd27de97cf32aea49dea97527bd66add01ea))
* reject sensitive workflow artifacts ([56e6d34](https://github.com/Syfra3/Rotta/commit/56e6d34d5d15c44785d731135909e83512954053))
* report workflow cleanup guidance ([30fa219](https://github.com/Syfra3/Rotta/commit/30fa2190f104e2933a3bf005fa154f02ede505dd))
* serialize pointer-only workflow state ([0d349c0](https://github.com/Syfra3/Rotta/commit/0d349c0f7a3a62f21db1090de8be48c01c098765))
* validate stable scenario traces ([00cc249](https://github.com/Syfra3/Rotta/commit/00cc24907277192286a9fe62b29f716ce643c89f))
* validate stale workflow state pointers ([8b985d2](https://github.com/Syfra3/Rotta/commit/8b985d24191a910b43720f86dd317b891d76c892))


### Bug Fixes

* CI/CD ([6f7f0c2](https://github.com/Syfra3/Rotta/commit/6f7f0c2dbd448e353c927c78852ba7b0b0343058))
* improve opencode vela setup ([3c22815](https://github.com/Syfra3/Rotta/commit/3c2281597bb91f9b29ead37867e97012c9aefc7a))
* improve opencode vela setup ([e4f2aeb](https://github.com/Syfra3/Rotta/commit/e4f2aeba71896a609fdc47789a3a29c66ecbd9f7))
* **installer:** refresh vela before setup and clustering install ([da3bf6d](https://github.com/Syfra3/Rotta/commit/da3bf6d339302fe57c20c6c20972b10b0bbbe803))
* **installer:** upgrade existing Vela before agent setup ([470f949](https://github.com/Syfra3/Rotta/commit/470f949af1d5f47e2c827e2c3e26def66ae989d7))
* load quality gates from TUI config ([791b11e](https://github.com/Syfra3/Rotta/commit/791b11e41347a8a6f842d54d3d7d5636e01715ad))
* prepare Rotta Homebrew releases ([c9c6825](https://github.com/Syfra3/Rotta/commit/c9c68251609ce413ecda9156a8f41253b0cb7f44))
* show vela guard feedback ([1535e9d](https://github.com/Syfra3/Rotta/commit/1535e9d57f4b8a8a279b495e81c56c1ed3b8b811))
* show Vela guard feedback ([2603968](https://github.com/Syfra3/Rotta/commit/26039683ec27648ba122054b027169cb58e26347))


### Documentation

* add README header image ([bb5a4e1](https://github.com/Syfra3/Rotta/commit/bb5a4e1f59198f6438c8d9e7b49b465386449b25))
* add workflow artifact lifecycle plan ([3e9cdc0](https://github.com/Syfra3/Rotta/commit/3e9cdc0132b663cb3da676d727bde2d97173c23e))
* add workflow decision diagram ([68192ea](https://github.com/Syfra3/Rotta/commit/68192ea4faee0735b394cc445f66bb4c86a4e327))
* approve artifact lifecycle bootstrap ([80b2689](https://github.com/Syfra3/Rotta/commit/80b26894e61f34474a7044bb2c9924ef5bd8959f))


### Code Refactoring

* rename project to clean-workflow ([8397516](https://github.com/Syfra3/Rotta/commit/8397516960e4a87fc377f9136fff1a9e91b4aa37))

## [1.4.0](https://github.com/Syfra3/Rotta/compare/rotta-v1.3.0...rotta-v1.4.0) (2026-07-01)


### Features

* add installer backup and clean reinstall ([de6b648](https://github.com/Syfra3/Rotta/commit/de6b648cb886f66f67db7a4bf804c930893c9def))
* add installer recovery flow ([6dfec6e](https://github.com/Syfra3/Rotta/commit/6dfec6ee668d0dbdd902735ac9e958931c74bda0))
* add scoped workflow approval gate ([644e2ab](https://github.com/Syfra3/Rotta/commit/644e2ab32ed660fd38bc01f92795f62e4702ff94))
* archive retired workflow artifacts ([3d0ea2c](https://github.com/Syfra3/Rotta/commit/3d0ea2c91185394d2a2d319bd03a8816b6ac4378))
* classify tracked workflow contracts ([a8863c7](https://github.com/Syfra3/Rotta/commit/a8863c70196f712b9a9cbb53dcaec13bce4e9cde))
* exclude local cache artifacts ([5e240cc](https://github.com/Syfra3/Rotta/commit/5e240ccf792187583630297ce81a13557c1ef575))
* implement workflow artifact lifecycle ([b76423f](https://github.com/Syfra3/Rotta/commit/b76423fa33c27a6be84d251182cdf7d3f991de72))
* install vela freshness guards ([f1ee63a](https://github.com/Syfra3/Rotta/commit/f1ee63a4be5bd48f4581789d8c22327a461f6c78))
* **installer:** add optional Vela integration ([5dc1617](https://github.com/Syfra3/Rotta/commit/5dc1617bda223e06f937b62f2d333be88c152740))
* **installer:** add optional Vela integration ([e8ed277](https://github.com/Syfra3/Rotta/commit/e8ed277f71c501efd637688d34b2fc440fb239b8))
* **installer:** add recovery backup restore ([f470a6d](https://github.com/Syfra3/Rotta/commit/f470a6d54db9846800b5b04a2272c25c86b8c4ca))
* keep implemented features active ([3f97141](https://github.com/Syfra3/Rotta/commit/3f97141f70bbf7e4449a85a5fe4d9cb4adfac4d7))
* plan approved repository scenarios ([48bc39e](https://github.com/Syfra3/Rotta/commit/48bc39e05d83f1f35ba4719b700ed193625ce9d5))
* protect namespaced workflow artifacts ([24d5fca](https://github.com/Syfra3/Rotta/commit/24d5fca0f28a401a9d7f4b301359518352803c19))
* protect untracked active contracts ([96adcd2](https://github.com/Syfra3/Rotta/commit/96adcd27de97cf32aea49dea97527bd66add01ea))
* reject sensitive workflow artifacts ([56e6d34](https://github.com/Syfra3/Rotta/commit/56e6d34d5d15c44785d731135909e83512954053))
* report workflow cleanup guidance ([30fa219](https://github.com/Syfra3/Rotta/commit/30fa2190f104e2933a3bf005fa154f02ede505dd))
* serialize pointer-only workflow state ([0d349c0](https://github.com/Syfra3/Rotta/commit/0d349c0f7a3a62f21db1090de8be48c01c098765))
* validate stable scenario traces ([00cc249](https://github.com/Syfra3/Rotta/commit/00cc24907277192286a9fe62b29f716ce643c89f))
* validate stale workflow state pointers ([8b985d2](https://github.com/Syfra3/Rotta/commit/8b985d24191a910b43720f86dd317b891d76c892))


### Bug Fixes

* CI/CD ([6f7f0c2](https://github.com/Syfra3/Rotta/commit/6f7f0c2dbd448e353c927c78852ba7b0b0343058))
* load quality gates from TUI config ([791b11e](https://github.com/Syfra3/Rotta/commit/791b11e41347a8a6f842d54d3d7d5636e01715ad))
* prepare Rotta Homebrew releases ([c9c6825](https://github.com/Syfra3/Rotta/commit/c9c68251609ce413ecda9156a8f41253b0cb7f44))


### Documentation

* add README header image ([bb5a4e1](https://github.com/Syfra3/Rotta/commit/bb5a4e1f59198f6438c8d9e7b49b465386449b25))
* add workflow artifact lifecycle plan ([3e9cdc0](https://github.com/Syfra3/Rotta/commit/3e9cdc0132b663cb3da676d727bde2d97173c23e))
* add workflow decision diagram ([68192ea](https://github.com/Syfra3/Rotta/commit/68192ea4faee0735b394cc445f66bb4c86a4e327))
* approve artifact lifecycle bootstrap ([80b2689](https://github.com/Syfra3/Rotta/commit/80b26894e61f34474a7044bb2c9924ef5bd8959f))


### Code Refactoring

* rename project to clean-workflow ([8397516](https://github.com/Syfra3/Rotta/commit/8397516960e4a87fc377f9136fff1a9e91b4aa37))

## [1.3.0](https://github.com/Syfra3/clean-workflow/compare/clean-workflow-v1.2.0...clean-workflow-v1.3.0) (2026-06-30)


### Features

* add scoped workflow approval gate ([644e2ab](https://github.com/Syfra3/clean-workflow/commit/644e2ab32ed660fd38bc01f92795f62e4702ff94))
* archive retired workflow artifacts ([3d0ea2c](https://github.com/Syfra3/clean-workflow/commit/3d0ea2c91185394d2a2d319bd03a8816b6ac4378))
* classify tracked workflow contracts ([a8863c7](https://github.com/Syfra3/clean-workflow/commit/a8863c70196f712b9a9cbb53dcaec13bce4e9cde))
* exclude local cache artifacts ([5e240cc](https://github.com/Syfra3/clean-workflow/commit/5e240ccf792187583630297ce81a13557c1ef575))
* implement workflow artifact lifecycle ([b76423f](https://github.com/Syfra3/clean-workflow/commit/b76423fa33c27a6be84d251182cdf7d3f991de72))
* keep implemented features active ([3f97141](https://github.com/Syfra3/clean-workflow/commit/3f97141f70bbf7e4449a85a5fe4d9cb4adfac4d7))
* plan approved repository scenarios ([48bc39e](https://github.com/Syfra3/clean-workflow/commit/48bc39e05d83f1f35ba4719b700ed193625ce9d5))
* protect namespaced workflow artifacts ([24d5fca](https://github.com/Syfra3/clean-workflow/commit/24d5fca0f28a401a9d7f4b301359518352803c19))
* protect untracked active contracts ([96adcd2](https://github.com/Syfra3/clean-workflow/commit/96adcd27de97cf32aea49dea97527bd66add01ea))
* reject sensitive workflow artifacts ([56e6d34](https://github.com/Syfra3/clean-workflow/commit/56e6d34d5d15c44785d731135909e83512954053))
* report workflow cleanup guidance ([30fa219](https://github.com/Syfra3/clean-workflow/commit/30fa2190f104e2933a3bf005fa154f02ede505dd))
* serialize pointer-only workflow state ([0d349c0](https://github.com/Syfra3/clean-workflow/commit/0d349c0f7a3a62f21db1090de8be48c01c098765))
* validate stable scenario traces ([00cc249](https://github.com/Syfra3/clean-workflow/commit/00cc24907277192286a9fe62b29f716ce643c89f))
* validate stale workflow state pointers ([8b985d2](https://github.com/Syfra3/clean-workflow/commit/8b985d24191a910b43720f86dd317b891d76c892))


### Documentation

* add workflow artifact lifecycle plan ([3e9cdc0](https://github.com/Syfra3/clean-workflow/commit/3e9cdc0132b663cb3da676d727bde2d97173c23e))
* approve artifact lifecycle bootstrap ([80b2689](https://github.com/Syfra3/clean-workflow/commit/80b26894e61f34474a7044bb2c9924ef5bd8959f))

## [1.2.0](https://github.com/Syfra3/clean-workflow/compare/clean-workflow-v1.1.0...clean-workflow-v1.2.0) (2026-06-30)


### Features

* **installer:** add recovery backup restore ([f470a6d](https://github.com/Syfra3/clean-workflow/commit/f470a6d54db9846800b5b04a2272c25c86b8c4ca))

## [1.1.0](https://github.com/Syfra3/clean-workflow/compare/clean-workflow-v1.0.0...clean-workflow-v1.1.0) (2026-06-29)


### Features

* **installer:** add optional Vela integration ([5dc1617](https://github.com/Syfra3/clean-workflow/commit/5dc1617bda223e06f937b62f2d333be88c152740))
* **installer:** add optional Vela integration ([e8ed277](https://github.com/Syfra3/clean-workflow/commit/e8ed277f71c501efd637688d34b2fc440fb239b8))

## 1.0.0 (2026-06-26)


### Bug Fixes

* CI/CD ([6f7f0c2](https://github.com/Syfra3/bob-workflow/commit/6f7f0c2dbd448e353c927c78852ba7b0b0343058))
* load quality gates from TUI config ([791b11e](https://github.com/Syfra3/bob-workflow/commit/791b11e41347a8a6f842d54d3d7d5636e01715ad))


### Documentation

* add workflow decision diagram ([68192ea](https://github.com/Syfra3/bob-workflow/commit/68192ea4faee0735b394cc445f66bb4c86a4e327))


### Code Refactoring

* rename project to clean-workflow ([8397516](https://github.com/Syfra3/bob-workflow/commit/8397516960e4a87fc377f9136fff1a9e91b4aa37))
