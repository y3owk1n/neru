# Changelog

## [1.11.1](https://github.com/y3owk1n/neru/compare/v1.11.0...v1.11.1) (2025-11-30)


### Bug Fixes

* improve unsafe pointer handling for C callback interop ([#250](https://github.com/y3owk1n/neru/issues/250)) ([50c0929](https://github.com/y3owk1n/neru/commit/50c092934851bfbd568eb933720d37e19601da1c))

## [1.11.0](https://github.com/y3owk1n/neru/compare/v1.10.5...v1.11.0) (2025-11-30)


### Features

* **modes:** add Action and Scroll navigation modes with Mode interface ([#238](https://github.com/y3owk1n/neru/issues/238)) ([8c55ec4](https://github.com/y3owk1n/neru/commit/8c55ec44bf85cb84ee84f4523cfc117af169e168))
* **test:** enhance integration tests with comprehensive coverage ([#242](https://github.com/y3owk1n/neru/issues/242)) ([f44b1ea](https://github.com/y3owk1n/neru/commit/f44b1ea63105a4aa21d2a53eeaae7857d4261060))


### Bug Fixes

* resolve CLI nil pointer regression and add comprehensive tests ([#244](https://github.com/y3owk1n/neru/issues/244)) ([9a611d6](https://github.com/y3owk1n/neru/commit/9a611d6e6952b1b35d355871ffe1f0a33c5c8f4a))


### Performance Improvements

* implement accessibility query optimization ([#245](https://github.com/y3owk1n/neru/issues/245)) ([9ab4930](https://github.com/y3owk1n/neru/commit/9ab493040ade44750b0bd3ffca7d88546709cc68))
* implement grid rendering optimizations ([#247](https://github.com/y3owk1n/neru/issues/247)) ([dda783f](https://github.com/y3owk1n/neru/commit/dda783fdf027ad693c41a7a831a41ca56c4d42f4))
* implement hint filtering performance optimizations ([#246](https://github.com/y3owk1n/neru/issues/246)) ([8076df5](https://github.com/y3owk1n/neru/commit/8076df539597fbf68bb2e543df16300a9c7960fe))
* implement memory management optimizations ([#248](https://github.com/y3owk1n/neru/issues/248)) ([f54dedc](https://github.com/y3owk1n/neru/commit/f54dedcdf8074ab82a8df623ab1df37daab92737))

## [1.10.5](https://github.com/y3owk1n/neru/compare/v1.10.4...v1.10.5) (2025-11-28)


### Bug Fixes

* **config:** fix app-specific hints configuration and add tests ([#223](https://github.com/y3owk1n/neru/issues/223)) ([1ca2c38](https://github.com/y3owk1n/neru/commit/1ca2c38ad9941885dd6556994c38bffafc37a346))
* **overlay:** clear overlay when switching from scroll to hints/grid mode ([#228](https://github.com/y3owk1n/neru/issues/228)) ([20a23ea](https://github.com/y3owk1n/neru/commit/20a23eae7e560c97bf51ecebcbde284952c30092))
* **scroll:** improve responsiveness of scroll mode activation ([#225](https://github.com/y3owk1n/neru/issues/225)) ([7377a74](https://github.com/y3owk1n/neru/commit/7377a74e3c378aa0f21745c3aa146a4e9f9dbf06))


### Performance Improvements

* optimize memory and CPU usage ([#226](https://github.com/y3owk1n/neru/issues/226)) ([0b4560a](https://github.com/y3owk1n/neru/commit/0b4560a372d1604a3adc79245d08336c8cf80a5e))

## [1.10.4](https://github.com/y3owk1n/neru/compare/v1.10.3...v1.10.4) (2025-11-27)


### Bug Fixes

* cleanup for some unused code and functions ([#211](https://github.com/y3owk1n/neru/issues/211)) ([3093084](https://github.com/y3owk1n/neru/commit/30930844548d18093be9d6918ccd1704c4000fe8))
* **grid:** ensure grid overlay style updated in `reload config` ([#208](https://github.com/y3owk1n/neru/issues/208)) ([c08cb2a](https://github.com/y3owk1n/neru/commit/c08cb2a3e66442daee2240f34a0d5f2517f10c3e))
* **hints:** ensure hints using the configured config styles than just defaults ([#209](https://github.com/y3owk1n/neru/issues/209)) ([f7e808c](https://github.com/y3owk1n/neru/commit/f7e808c2a9d0b0ec6e6f714c370cee817740b5e5))
* **logger:** add consoleWriter parameter to Init for output control ([#215](https://github.com/y3owk1n/neru/issues/215)) ([6fbf809](https://github.com/y3owk1n/neru/commit/6fbf8092547c0561306d44cb2ea4f7a9caf8c0f7))
* minor refactoring to ensure functions are not too long ([#212](https://github.com/y3owk1n/neru/issues/212)) ([99ccfbf](https://github.com/y3owk1n/neru/commit/99ccfbfdc1c9195fee522307cfacbd5aac76cfad))


### Performance Improvements

* **overlay:** optimize memory usage in drawing operations ([#206](https://github.com/y3owk1n/neru/issues/206)) ([aac6119](https://github.com/y3owk1n/neru/commit/aac611944eef3347e7f15327d2a86f11451bedde))

## [1.10.3](https://github.com/y3owk1n/neru/compare/v1.10.2...v1.10.3) (2025-11-26)


### Bug Fixes

* add coding standard with stricter golangci config ([#196](https://github.com/y3owk1n/neru/issues/196)) ([8ab3f3b](https://github.com/y3owk1n/neru/commit/8ab3f3bc4577c3ec026d9aa9852abf794195de6c))
* add more test and update some existing tests ([#192](https://github.com/y3owk1n/neru/issues/192)) ([50a59af](https://github.com/y3owk1n/neru/commit/50a59afa0b42e4e62959ef80a0efdfa9ba28a61f))
* clang indent tab and format ([#202](https://github.com/y3owk1n/neru/issues/202)) ([8536bef](https://github.com/y3owk1n/neru/commit/8536bef1c4c869af7282898a956137317ac58a60))
* cleanup some obvious comments and add some comments within complex functions ([#203](https://github.com/y3owk1n/neru/issues/203)) ([58f5164](https://github.com/y3owk1n/neru/commit/58f5164535b0e12a82e83ef68af0a59676282c6f))
* crash in grid bench and move all bench test to its own file ([#191](https://github.com/y3owk1n/neru/issues/191)) ([97263a3](https://github.com/y3owk1n/neru/commit/97263a3ccc497991289ebd8e5ce89980d21be542))
* ensure tests are all with blackbox approach ([#189](https://github.com/y3owk1n/neru/issues/189)) ([33c2223](https://github.com/y3owk1n/neru/commit/33c2223210e9b3a5a4af2e09faf6e1a9f2723f15))
* make most struc private and expose getter and setter ([#197](https://github.com/y3owk1n/neru/issues/197)) ([83bdde4](https://github.com/y3owk1n/neru/commit/83bdde43cabee11a39d50e7075c160498b6a2266))
* no magic numbers and some refactoring to adhere coding style ([#200](https://github.com/y3owk1n/neru/issues/200)) ([04433ec](https://github.com/y3owk1n/neru/commit/04433ec5df62d1f90e6b404e5c87863cc089c1ab))
* remove `Get` keywords from getter ([#198](https://github.com/y3owk1n/neru/issues/198)) ([342b0bd](https://github.com/y3owk1n/neru/commit/342b0bdee2d67b530ec1fc1fa26acd5f7426b523))
* remove unneeded globals ([#194](https://github.com/y3owk1n/neru/issues/194)) ([4418fd3](https://github.com/y3owk1n/neru/commit/4418fd398751ba7837c9eb1355bc2d6f288686f7))
* update flake to support overlays [skip ci] ([#195](https://github.com/y3owk1n/neru/issues/195)) ([cea0a6e](https://github.com/y3owk1n/neru/commit/cea0a6ecac04fc4f942a089d1ebadee4706b677d))

## [1.10.2](https://github.com/y3owk1n/neru/compare/v1.10.1...v1.10.2) (2025-11-25)


### Bug Fixes

* **lifecycle:** ensure `handleScreenParametersChange` will refresh overlays including state and style ([#186](https://github.com/y3owk1n/neru/issues/186)) ([883a13a](https://github.com/y3owk1n/neru/commit/883a13a4294a878251d57a528f34f91e72183e5c))
* **logger:** remove noises that doesn't bring value for debugging ([#188](https://github.com/y3owk1n/neru/issues/188)) ([c119abf](https://github.com/y3owk1n/neru/commit/c119abfb5ff6fca588398cafb40de8429a96a940))

## [1.10.1](https://github.com/y3owk1n/neru/compare/v1.10.0...v1.10.1) (2025-11-24)


### Bug Fixes

* **metrics:** provide toggle to enable metrics collection, off by default ([#183](https://github.com/y3owk1n/neru/issues/183)) ([f7b6532](https://github.com/y3owk1n/neru/commit/f7b6532c5598a5a6c90472d234f537722bc4e7a7))
* more test to improve coverage ([#179](https://github.com/y3owk1n/neru/issues/179)) ([7b4d521](https://github.com/y3owk1n/neru/commit/7b4d52177f68f465e90b6cd25679206dc0a34f98))
* remove all `GetScrollableElements` ([#184](https://github.com/y3owk1n/neru/issues/184)) ([5340972](https://github.com/y3owk1n/neru/commit/53409728e4bb15c068d3eff3ea123ed1c93d33e6))
* standardize error handling and include `err113` lint ([#185](https://github.com/y3owk1n/neru/issues/185)) ([3d1888e](https://github.com/y3owk1n/neru/commit/3d1888e9f2a83f4b1fac759bef48c168e2b3b160))
* tighten golangci lint and refactoring variables ([#181](https://github.com/y3owk1n/neru/issues/181)) ([e570663](https://github.com/y3owk1n/neru/commit/e570663d5d5b31428417ce328c9654094421505a))

## [1.10.0](https://github.com/y3owk1n/neru/compare/v1.9.0...v1.10.0) (2025-11-23)


### Features

* **cli:** add --action flag for hints and grid commands ([#167](https://github.com/y3owk1n/neru/issues/167)) ([d41bc99](https://github.com/y3owk1n/neru/commit/d41bc99db5ff73cb28d9f94fcf89cbb35ed64c73))
* **config:** remove subgrid_enabled option and make subgrid always enabled ([#171](https://github.com/y3owk1n/neru/issues/171)) ([970962e](https://github.com/y3owk1n/neru/commit/970962ef6677fa803381c00fb3f9634bccd54d64))
* implement program wide DI and initial testing suite ([#175](https://github.com/y3owk1n/neru/issues/175)) ([bf29d4e](https://github.com/y3owk1n/neru/commit/bf29d4eb368be696d3b9116766e3b5cd49257fb3))


### Bug Fixes

* add IPC timeout with flags and 5 sec default ([#155](https://github.com/y3owk1n/neru/issues/155)) ([6e5bd24](https://github.com/y3owk1n/neru/commit/6e5bd24bc67f3b76623d83b29a51496543735bd6))
* allow smooth mouse movement ([#159](https://github.com/y3owk1n/neru/issues/159)) ([63cb580](https://github.com/y3owk1n/neru/commit/63cb580124ffa7628b7d26c3e280edc542f749ae))
* **config:** ensure reliable reload and improve startup validation UX ([#169](https://github.com/y3owk1n/neru/issues/169)) ([8127e49](https://github.com/y3owk1n/neru/commit/8127e49ad332d94ba2c1cd1dbe5493b4e29eba1d))
* do not restore cursor if switches to scroll action ([#152](https://github.com/y3owk1n/neru/issues/152)) ([d5ed1d5](https://github.com/y3owk1n/neru/commit/d5ed1d5d4c9eb46dd36c8dd2abbe32203bf9d3f5))
* domain based refactoring & update comments ([#157](https://github.com/y3owk1n/neru/issues/157)) ([b5d6c96](https://github.com/y3owk1n/neru/commit/b5d6c96f19747448f9df732cba67a814e56e57a7))
* enhance codebase robustness and error handling ([#165](https://github.com/y3owk1n/neru/issues/165)) ([b3e6188](https://github.com/y3owk1n/neru/commit/b3e6188f07a04cabf21073e606b1c8d0b171e615))
* ensure clean scroll context when switching mode ([#163](https://github.com/y3owk1n/neru/issues/163)) ([cd69912](https://github.com/y3owk1n/neru/commit/cd69912c18fb8ab01f9ff4d475e9fbb8ca6dcb4f))
* ensure just fmt pointing to new bridge location [skip ci] ([#162](https://github.com/y3owk1n/neru/issues/162)) ([1089e10](https://github.com/y3owk1n/neru/commit/1089e104ab8b460f377b4b4c674b40191e645cba))
* focus on performance optimisation ([#173](https://github.com/y3owk1n/neru/issues/173)) ([74b61f4](https://github.com/y3owk1n/neru/commit/74b61f4d320366c77ef077c7e1cf8051d53e7eaa))
* **grid:** move mouse to cell center when showing subgrid ([#172](https://github.com/y3owk1n/neru/issues/172)) ([bcedcd0](https://github.com/y3owk1n/neru/commit/bcedcd05f7ebaed81387af110dc8659886de1c45))
* **hints:** prevent cursor restoration during hint mode transitions ([#170](https://github.com/y3owk1n/neru/issues/170)) ([83922f0](https://github.com/y3owk1n/neru/commit/83922f08da64a2a17ea30ea54a4e68bf71514c22))
* improve godoc and remove noises for obvious or not needed comments [skip ci] ([#164](https://github.com/y3owk1n/neru/issues/164)) ([3a3c2b2](https://github.com/y3owk1n/neru/commit/3a3c2b255968eb2c432cd228f71f8060318d16e4))
* improve logging and add config dumps ([#154](https://github.com/y3owk1n/neru/issues/154)) ([bc99f53](https://github.com/y3owk1n/neru/commit/bc99f53f0514cbdd03bd7b760e902e30af82ed6e))
* make uuid direct in go.mod ([#176](https://github.com/y3owk1n/neru/issues/176)) ([bb2bbe2](https://github.com/y3owk1n/neru/commit/bb2bbe2e0b2e910257f7528f71514bf6ccfe917c))
* **modes:** reset cursor state when exiting scroll mode via ESC ([#174](https://github.com/y3owk1n/neru/issues/174)) ([12f2124](https://github.com/y3owk1n/neru/commit/12f2124eb91622ac4bcbbeebb4716b882e2d0d12))
* more tests ([#177](https://github.com/y3owk1n/neru/issues/177)) ([a019a53](https://github.com/y3owk1n/neru/commit/a019a53d64fdf9cf4b08b4c064bea1e1633c39e1))
* remove `:` from exec ([#168](https://github.com/y3owk1n/neru/issues/168)) ([385f2ce](https://github.com/y3owk1n/neru/commit/385f2ce5307b64a25e555c4fdca5f20f02b84610))
* restructure the codebase to be more concise ([#161](https://github.com/y3owk1n/neru/issues/161)) ([a089c13](https://github.com/y3owk1n/neru/commit/a089c13f100299e7d53bfe8007fef90770b52b54))
* use faster default smooth cursor for config ([#160](https://github.com/y3owk1n/neru/issues/160)) ([97c8af7](https://github.com/y3owk1n/neru/commit/97c8af711abab4e3c59bc3faed3e1e7624073e9f))

## [1.9.0](https://github.com/y3owk1n/neru/compare/v1.8.0...v1.9.0) (2025-11-16)


### Features

* implement restore cursor after escape mode ([#150](https://github.com/y3owk1n/neru/issues/150)) ([131570f](https://github.com/y3owk1n/neru/commit/131570f93173af22f6532e7d493e04713494c92f))

## [1.8.0](https://github.com/y3owk1n/neru/compare/v1.7.1...v1.8.0) (2025-11-16)


### Features

* add `action` cli that act on current cursor position ([#126](https://github.com/y3owk1n/neru/issues/126)) ([52aaba6](https://github.com/y3owk1n/neru/commit/52aaba6c32cbabb68f294f7f3773d46a2b0ab50d))
* add submode for action that is configurable and allow to &lt;tab&gt; through ([#144](https://github.com/y3owk1n/neru/issues/144)) ([770fc42](https://github.com/y3owk1n/neru/commit/770fc4256ccf24bdc79a83be16957b8cdacda198))
* allow different action colors configuration for grid, same as hints ([#129](https://github.com/y3owk1n/neru/issues/129)) ([61307c8](https://github.com/y3owk1n/neru/commit/61307c8ebb6f5d029fb49d4663fb9ade892bd83e))
* centralize overlay into a single manager and window ([#146](https://github.com/y3owk1n/neru/issues/146)) ([70c4fbe](https://github.com/y3owk1n/neru/commit/70c4fbecba67a794229458f03b023cb3a0a38a3f))
* grid based navigation mode ([#111](https://github.com/y3owk1n/neru/issues/111)) ([441192b](https://github.com/y3owk1n/neru/commit/441192bde26215ec873ef41204e389eaabfeef29))
* **grid:** optimize for square cells across all screen types ([#122](https://github.com/y3owk1n/neru/issues/122)) ([d40d990](https://github.com/y3owk1n/neru/commit/d40d9908e397dee095d5fcde415add9efa382467))
* simplify actions ([#141](https://github.com/y3owk1n/neru/issues/141)) ([49629ff](https://github.com/y3owk1n/neru/commit/49629ff9bfe32bd2183510b767581f7f31176138))


### Bug Fixes

* add missing grid cli for various actions ([#125](https://github.com/y3owk1n/neru/issues/125)) ([c92d09e](https://github.com/y3owk1n/neru/commit/c92d09e941eed56dace5fa26e0449f5b1f9f9251))
* add more loggings throughout the program for better viewabilitiy ([#130](https://github.com/y3owk1n/neru/issues/130)) ([a6ab8d7](https://github.com/y3owk1n/neru/commit/a6ab8d7ea6437d795c57fa7bc2e8ec72e9696238))
* allow hiding unmatched in grid ([#113](https://github.com/y3owk1n/neru/issues/113)) ([59bcca6](https://github.com/y3owk1n/neru/commit/59bcca60a139fe102537c56bf9857d538a6dd511))
* do not clear hintOverlay when cleaning up grid ([#142](https://github.com/y3owk1n/neru/issues/142)) ([d31648f](https://github.com/y3owk1n/neru/commit/d31648f75957d38033cd7c496386c528f070e146))
* ensure backspace can successfully return back to main grid with state ([#136](https://github.com/y3owk1n/neru/issues/136)) ([ae139b1](https://github.com/y3owk1n/neru/commit/ae139b1ec4d996e3c8846ab3626469e73c32a48c))
* ensure different screen sizes and resolution gets the best grid ([#115](https://github.com/y3owk1n/neru/issues/115)) ([6016fce](https://github.com/y3owk1n/neru/commit/6016fce12d2fbba894193a51733f06cfe12419cb))
* ensure grid and hint dont sleep when refresh overlays ([#124](https://github.com/y3owk1n/neru/issues/124)) ([e35c486](https://github.com/y3owk1n/neru/commit/e35c486ff5eb22390be0fcf5bb44850885f4ddfe))
* ensure grid is clicking on absolute point, not relative points ([#118](https://github.com/y3owk1n/neru/issues/118)) ([c08b2a9](https://github.com/y3owk1n/neru/commit/c08b2a95473dd04c045d2f45b47cb2d1342acc5f))
* ensure hints working across multi displays ([#117](https://github.com/y3owk1n/neru/issues/117)) ([29cf0cb](https://github.com/y3owk1n/neru/commit/29cf0cb6dea89d569f4411826bdab8498cd587b4))
* **grid:** only call `ResizeToActiveScreen` when it is needed, not every activation ([#123](https://github.com/y3owk1n/neru/issues/123)) ([856f0b6](https://github.com/y3owk1n/neru/commit/856f0b6a9098dab18a37bc612b9fd124ebdb7ae5))
* improve ci for formatting, linting and vetting go and objc files ([#149](https://github.com/y3owk1n/neru/issues/149)) ([4d09a16](https://github.com/y3owk1n/neru/commit/4d09a167f3a50d331a2c397531c0e1085be2bc12))
* improve linting with more checks ([#147](https://github.com/y3owk1n/neru/issues/147)) ([6091077](https://github.com/y3owk1n/neru/commit/609107742cfe8f7ff40e959542161dd87c9d1e37))
* improve log retention with rotation with more configurable options ([#145](https://github.com/y3owk1n/neru/issues/145)) ([87a85e7](https://github.com/y3owk1n/neru/commit/87a85e73d525f5bf761f765608ce4bd4dee32690))
* improve matched_text highlihts to only highlight the prefix text ([#137](https://github.com/y3owk1n/neru/issues/137)) ([6d6e639](https://github.com/y3owk1n/neru/commit/6d6e639e664a119c01f83fbe01b9294325e44d38))
* improve perf for grid by caching the labels plus more optim ([#139](https://github.com/y3owk1n/neru/issues/139)) ([41fc4ed](https://github.com/y3owk1n/neru/commit/41fc4ed1893a1bb10ff9b87b951bf02de5c04145))
* improve resize callback handling for better UX ([#132](https://github.com/y3owk1n/neru/issues/132)) ([38dbdd9](https://github.com/y3owk1n/neru/commit/38dbdd9d1704b04dd3f3139d42c5db80b93fb608))
* make actions point based instead of element based ([#110](https://github.com/y3owk1n/neru/issues/110)) ([472a390](https://github.com/y3owk1n/neru/commit/472a39011d28c845b16faa6637bb2302076d9df6))
* make grids more predictable than always linearly showing the labels ([#131](https://github.com/y3owk1n/neru/issues/131)) ([808ca14](https://github.com/y3owk1n/neru/commit/808ca14a73d1b9720e11c4dea73c041194e9e7ae))
* more improvements on linting configuration and fixes ([#148](https://github.com/y3owk1n/neru/issues/148)) ([80e0544](https://github.com/y3owk1n/neru/commit/80e054476789909336d2d3db3e8901608a86b8f3))
* more perf for grids and hints ([#140](https://github.com/y3owk1n/neru/issues/140)) ([013025c](https://github.com/y3owk1n/neru/commit/013025c6bbd52ae02c5fad82d6b036127bfb2e2b))
* only use default hotkey when not configured ([#143](https://github.com/y3owk1n/neru/issues/143)) ([f6629cf](https://github.com/y3owk1n/neru/commit/f6629cf16b2af6c373b072a26e83ff8b0ff0cf79))
* react to screen changes and support extended displays for grid ([#116](https://github.com/y3owk1n/neru/issues/116)) ([1fd81c2](https://github.com/y3owk1n/neru/commit/1fd81c27e33d3c16854cf039be516cfe7e1ec02e))
* refactoring and move domain based implementation to internals ([#108](https://github.com/y3owk1n/neru/issues/108)) ([9675949](https://github.com/y3owk1n/neru/commit/96759497600c02b732e4f9273873b93a4c4aac50))
* remove subgrid cell and row config, as we are fixing it to square 3x3 ([#127](https://github.com/y3owk1n/neru/issues/127)) ([56d89d2](https://github.com/y3owk1n/neru/commit/56d89d2de225d8b0bbdfe177c5440b3226923f26))
* restrict keys to only available cells ([#114](https://github.com/y3owk1n/neru/issues/114)) ([d819d72](https://github.com/y3owk1n/neru/commit/d819d7278c4642a0f64af7b74582de44fccb2ad0))
* restructure config and move ax related to hints section instead ([#128](https://github.com/y3owk1n/neru/issues/128)) ([a9947dc](https://github.com/y3owk1n/neru/commit/a9947dcba9c6e8331980c32a4e1e00973539064a))
* separated restore cursor for grid ([#112](https://github.com/y3owk1n/neru/issues/112)) ([f91567f](https://github.com/y3owk1n/neru/commit/f91567f59ee93f12c2a6111f2d08f8cccb23edb4))
* slightly improve memory for overlay.m ([#138](https://github.com/y3owk1n/neru/issues/138)) ([84bd351](https://github.com/y3owk1n/neru/commit/84bd3519fac4883eb1b562d605693c1bdc96252d))
* update default values ([#119](https://github.com/y3owk1n/neru/issues/119)) ([7a946bb](https://github.com/y3owk1n/neru/commit/7a946bbe1cdd3a667f8d2204fed51381b2f95770))

## [1.7.1](https://github.com/y3owk1n/neru/compare/v1.7.0...v1.7.1) (2025-11-09)


### Bug Fixes

* ensure less memory leaks and crashes in objc ([#106](https://github.com/y3owk1n/neru/issues/106)) ([75f521d](https://github.com/y3owk1n/neru/commit/75f521d18f94c6d476239084e069154e4a3c7cd3))

## [1.7.0](https://github.com/y3owk1n/neru/compare/v1.6.0...v1.7.0) (2025-11-09)


### Features

* implements `triple click` and simple `drag and drop` with mouse down and mouse up on a target ([#102](https://github.com/y3owk1n/neru/issues/102)) ([ee89ce1](https://github.com/y3owk1n/neru/commit/ee89ce121d3d77e4024b686c1e7769954f278cd2))
* more robust cli and IPC handling + flexibility in hotkey mappings ([#103](https://github.com/y3owk1n/neru/issues/103)) ([e1eefbe](https://github.com/y3owk1n/neru/commit/e1eefbe31ab2aad9ca9fcaf46a6bc858fa2a8e91))


### Bug Fixes

* add version copy for systray ([#92](https://github.com/y3owk1n/neru/issues/92)) ([2e7242b](https://github.com/y3owk1n/neru/commit/2e7242b178c075190f402f00110d5280f6036769))
* allow &lt;tab&gt; key to toggle between scroll hint and scroll mode ([#87](https://github.com/y3owk1n/neru/issues/87)) ([5fa4c15](https://github.com/y3owk1n/neru/commit/5fa4c1554f222756811e792648afddb5708938a5))
* allow &lt;tab&gt; to switch between hints and hints_action ([#93](https://github.com/y3owk1n/neru/issues/93)) ([44ef64e](https://github.com/y3owk1n/neru/commit/44ef64ef6d75096530c52dfd0ebc2382a3205773))
* allow scroll hints styles to be configurable ([#88](https://github.com/y3owk1n/neru/issues/88)) ([84e087c](https://github.com/y3owk1n/neru/commit/84e087ca27a072df9be6366a14117650f87f1f56))
* auto mouse_up calls after mouse_down ([#104](https://github.com/y3owk1n/neru/issues/104)) ([f789391](https://github.com/y3owk1n/neru/commit/f7893910e3e18fdc4924f378bc47a02e0607186f))
* chill `shouldIncludeElement` checks ([#96](https://github.com/y3owk1n/neru/issues/96)) ([3141ace](https://github.com/y3owk1n/neru/commit/3141ace32f747417b6e0102a40307ced1ab6c051))
* ensure `status` command shows the right message when using default ([#91](https://github.com/y3owk1n/neru/issues/91)) ([4873616](https://github.com/y3owk1n/neru/commit/48736165cf2a99dee70464b51bd32f492e64a32d))
* ensure to init the same style for default scrol hints ([#89](https://github.com/y3owk1n/neru/issues/89)) ([0e0fc86](https://github.com/y3owk1n/neru/commit/0e0fc8695e0f227f4a7d784728a0cd734442643b))
* implements `IsMissionControlActive` to only show hints for mission control when active ([#95](https://github.com/y3owk1n/neru/issues/95)) ([9ac2295](https://github.com/y3owk1n/neru/commit/9ac2295db8757f064d3aa2b8c405913cd0b345d8))
* make scroll configuration easier and put everything in pixel ([#90](https://github.com/y3owk1n/neru/issues/90)) ([401da6b](https://github.com/y3owk1n/neru/commit/401da6b26916814a576df61c89d52bcf179f44fa))
* more refactor for main package ([#99](https://github.com/y3owk1n/neru/issues/99)) ([ed1c635](https://github.com/y3owk1n/neru/commit/ed1c635f9f2d53f7f88a30a976d0cfe601c25925))
* nicer color for matched texts in hint ([#100](https://github.com/y3owk1n/neru/issues/100)) ([ba7e3c9](https://github.com/y3owk1n/neru/commit/ba7e3c944e31956a9a2d2f29ed57d990c40b4bbd))
* refactor `main.go` and split them out, also update build command ([#98](https://github.com/y3owk1n/neru/issues/98)) ([2e98331](https://github.com/y3owk1n/neru/commit/2e983316818e7d5393a3fea32db0c07d1dcc8ec5))
* simpler scroll mechanism ([#85](https://github.com/y3owk1n/neru/issues/85)) ([edd608f](https://github.com/y3owk1n/neru/commit/edd608f60a18c9ac601e471c40aa7bb44ddf69dc))
* vertical action menu without magic heuristic numbers ([#101](https://github.com/y3owk1n/neru/issues/101)) ([9c2405d](https://github.com/y3owk1n/neru/commit/9c2405d6bb784917e726bad72111446a7880399b))
* wrong key for `scroll_step_full` in default config [skip ci] ([#94](https://github.com/y3owk1n/neru/issues/94)) ([c15e203](https://github.com/y3owk1n/neru/commit/c15e203b6ffb909957a8c8c822d7a1ffa65c30da))

## [1.6.0](https://github.com/y3owk1n/neru/compare/v1.5.1...v1.6.0) (2025-11-07)


### Features

* add more actions to systray ([#83](https://github.com/y3owk1n/neru/issues/83)) ([af3e11d](https://github.com/y3owk1n/neru/commit/af3e11d3897372ff0441d81642825ee9d91fc958))
* add more actions to systray menu for Neru ([#76](https://github.com/y3owk1n/neru/issues/76)) ([5c121db](https://github.com/y3owk1n/neru/commit/5c121db03e4b66e4e99e74f32b8375a21a079d84))
* allow to bypass heuristic checks for scrollable and clickable ([#81](https://github.com/y3owk1n/neru/issues/81)) ([60af71b](https://github.com/y3owk1n/neru/commit/60af71b69ecfb9771daac45504911afc2fa29318))
* also allow per-app ignore checks ([2803d0d](https://github.com/y3owk1n/neru/commit/2803d0d0b439386f465e99f48e96786aca2bf14d))
* make scroll areas dynamic with available areas ([#79](https://github.com/y3owk1n/neru/issues/79)) ([bded86e](https://github.com/y3owk1n/neru/commit/bded86e4e2492cc9748a4fb16a0664156c279982))
* revamp hotkey to support custom bash script and IPC calls ([#82](https://github.com/y3owk1n/neru/issues/82)) ([09633de](https://github.com/y3owk1n/neru/commit/09633def7a623668a87a3548a85795b8fdf978e7))
* split out electron, chromium and firefox supports ([#74](https://github.com/y3owk1n/neru/issues/74)) ([6b842f6](https://github.com/y3owk1n/neru/commit/6b842f61410b961206472e835d3044b798348e31))
* use app watcher to enable electron accessibility ([#73](https://github.com/y3owk1n/neru/issues/73)) ([2b9e0e5](https://github.com/y3owk1n/neru/commit/2b9e0e5f7cde5a3c7013c9bda5e0fe715d970c88))


### Bug Fixes

* add conditional parallelize for tree building ([#72](https://github.com/y3owk1n/neru/issues/72)) ([21c06aa](https://github.com/y3owk1n/neru/commit/21c06aa204abcad6e52cd681a2cb54f1f2a7f5b3))
* always force to US keyboard ([#75](https://github.com/y3owk1n/neru/issues/75)) ([a76b29f](https://github.com/y3owk1n/neru/commit/a76b29f8dc0937468dece6341df9bcf8c98e91b4))
* avoid memory leak and proper cleanups ([#80](https://github.com/y3owk1n/neru/issues/80)) ([1845299](https://github.com/y3owk1n/neru/commit/18452998f3fa05e45d46b0eb5b07a9235524a87d))
* centralize app watcher into single instance ([#78](https://github.com/y3owk1n/neru/issues/78)) ([b81a7c0](https://github.com/y3owk1n/neru/commit/b81a7c00f71e50fec7b26923cd2fe045c67bc7f8))
* move timer based watcher for hotkeys registration to app watcher ([#77](https://github.com/y3owk1n/neru/issues/77)) ([ae4e6cd](https://github.com/y3owk1n/neru/commit/ae4e6cd1eadbed5debb9c3ce355f9363b035b28a))
* removing maxDepth checks ([#70](https://github.com/y3owk1n/neru/issues/70)) ([53e0557](https://github.com/y3owk1n/neru/commit/53e0557552052b804427f859dca430e2608bcec2))

## [1.5.1](https://github.com/y3owk1n/neru/compare/v1.5.0...v1.5.1) (2025-11-06)


### Bug Fixes

* add defaults to the config to support launch on bundle start ([#67](https://github.com/y3owk1n/neru/issues/67)) ([30049e0](https://github.com/y3owk1n/neru/commit/30049e031853f65d661db2758437f3b05687b343))
* root cmd to auto launch when in app bundle ([#65](https://github.com/y3owk1n/neru/issues/65)) ([0af1e45](https://github.com/y3owk1n/neru/commit/0af1e45d382c2a0d0701b8268db1e016ae6456fc))

## [1.5.0](https://github.com/y3owk1n/neru/compare/v1.4.0...v1.5.0) (2025-11-06)


### Features

* allow configurable additional targets for menubar by bundle IDs ([#60](https://github.com/y3owk1n/neru/issues/60)) ([bca0da2](https://github.com/y3owk1n/neru/commit/bca0da2b8399931413a0a21c578b9ce449333325))
* implement bundle to work locally ([#62](https://github.com/y3owk1n/neru/issues/62)) ([7f18d45](https://github.com/y3owk1n/neru/commit/7f18d4540416d81b01795145c9bdfb82ba02c915))


### Bug Fixes

* add cache for elements and improve tree walking duration ([#58](https://github.com/y3owk1n/neru/issues/58)) ([22b1fa3](https://github.com/y3owk1n/neru/commit/22b1fa38d94908416f7213392dc65ddc1d589a98))
* allow 3 characters for maxHints ([#63](https://github.com/y3owk1n/neru/issues/63)) ([641c2b9](https://github.com/y3owk1n/neru/commit/641c2b9b127b0fecede934ed8062e0cb1da258bf))
* no need to check isEnabled in go code ([#59](https://github.com/y3owk1n/neru/issues/59)) ([ce753d9](https://github.com/y3owk1n/neru/commit/ce753d99a7bac47e4cc35465939b2ed8c70a6877))
* optimize tree building and element finding with parallelization and simple ttl caching ([#54](https://github.com/y3owk1n/neru/issues/54)) ([008f827](https://github.com/y3owk1n/neru/commit/008f827e003cbcc3ef8f62e41c1140a909fd887f))
* remove `isclickable` and `isscrollable` proxy and call it directly ([#56](https://github.com/y3owk1n/neru/issues/56)) ([e94de63](https://github.com/y3owk1n/neru/commit/e94de63765af66ca361fd2a6ae7ffe20f961f8f5))
* remove configurable maxHints and derive it from `hintChars` ([#61](https://github.com/y3owk1n/neru/issues/61)) ([3745e68](https://github.com/y3owk1n/neru/commit/3745e68107e847025c689945fbafe5df223fa20f))
* replace complex `visibleChildren` logic with `AXVisibleRow` detection ([#57](https://github.com/y3owk1n/neru/issues/57)) ([ac896fb](https://github.com/y3owk1n/neru/commit/ac896fba19c20126f5bcfec30e554bcf06fccb43))

## [1.4.0](https://github.com/y3owk1n/neru/compare/v1.3.0...v1.4.0) (2025-11-04)


### Features

* add notification center hints (configurable) ([#49](https://github.com/y3owk1n/neru/issues/49)) ([5e28692](https://github.com/y3owk1n/neru/commit/5e2869297b83086e6a992a61750f9c65723038f8))
* update branding to `neru` ([#52](https://github.com/y3owk1n/neru/issues/52)) ([7f9922b](https://github.com/y3owk1n/neru/commit/7f9922b0c555814a9bac0ba6499979f1b5981374))


### Bug Fixes

* better heuristic for clickable detection ([#51](https://github.com/y3owk1n/neru/issues/51)) ([f8767af](https://github.com/y3owk1n/neru/commit/f8767af40078a0a4e08705f15e2a51589c681c6b))

## [1.3.0](https://github.com/y3owk1n/neru/compare/v1.2.0...v1.3.0) (2025-11-03)


### Features

* allow configuration for restore cursor position ([#47](https://github.com/y3owk1n/neru/issues/47)) ([d5e5d87](https://github.com/y3owk1n/neru/commit/d5e5d87d46b0ed79407a348d83125900471a65a7))
* new action to move mouse to a position ([#46](https://github.com/y3owk1n/neru/issues/46)) ([9efc9eb](https://github.com/y3owk1n/neru/commit/9efc9eb52d35e6048b1c075824f033dd4fb6356e))


### Bug Fixes

* chromium support fix ([#43](https://github.com/y3owk1n/neru/issues/43)) ([f8c8463](https://github.com/y3owk1n/neru/commit/f8c84635fdf49c1863ff1cd518778649a0f49c63))
* replace all click actions to use actual mouse click rather than accessibility ([#45](https://github.com/y3owk1n/neru/issues/45)) ([7ebfaef](https://github.com/y3owk1n/neru/commit/7ebfaefc6a557805740231626890d030ae1d2c36))

## [1.2.0](https://github.com/y3owk1n/neru/compare/v1.1.1...v1.2.0) (2025-11-03)


### Features

* add occlusion detection with Dock exception ([#34](https://github.com/y3owk1n/neru/issues/34)) ([1d4146f](https://github.com/y3owk1n/neru/commit/1d4146ff44da7c114e4219b9d629ac6971355ada))


### Bug Fixes

* apply heuristic checks for clickable elements ([#38](https://github.com/y3owk1n/neru/issues/38)) ([f6be55d](https://github.com/y3owk1n/neru/commit/f6be55d4105c6c57844e5f02360983fd7c132f79))
* dynamically add menubar and dock roles when enabled ([#37](https://github.com/y3owk1n/neru/issues/37)) ([ab53a03](https://github.com/y3owk1n/neru/commit/ab53a0367724c951f7b27e43f5460b1e31e38809))
* properly check for electron with dynamic maxDepth ([#36](https://github.com/y3owk1n/neru/issues/36)) ([a10bdf7](https://github.com/y3owk1n/neru/commit/a10bdf741387e6b6965ba901093167a7a2b0419d))

## [1.1.1](https://github.com/y3owk1n/neru/compare/v1.1.0...v1.1.1) (2025-11-02)


### Bug Fixes

* ensure dock getting hints and does not crash the program ([#28](https://github.com/y3owk1n/neru/issues/28)) ([b572f58](https://github.com/y3owk1n/neru/commit/b572f58d04a11b47b686da58f99113e5b5c5ea51))
* improve large list traversal with progressive checks ([#30](https://github.com/y3owk1n/neru/issues/30)) ([5c3a0e3](https://github.com/y3owk1n/neru/commit/5c3a0e3aff9a02e5f7d78a7bf3bcc6f5006de7a7))
* move around config keys to places that make more sense ([#31](https://github.com/y3owk1n/neru/issues/31)) ([a929ec6](https://github.com/y3owk1n/neru/commit/a929ec6a52b0d6cee2762a536c5b6486104a8b49))
* standardize rect query and do not consider padding for bounds ([#32](https://github.com/y3owk1n/neru/issues/32)) ([fff37f5](https://github.com/y3owk1n/neru/commit/fff37f512e499d122f0df9c6f405325187f5afb6))

## [1.1.0](https://github.com/y3owk1n/neru/compare/v1.0.3...v1.1.0) (2025-11-02)


### Features

* add app exclusions ([#25](https://github.com/y3owk1n/neru/issues/25)) ([03a1c9d](https://github.com/y3owk1n/neru/commit/03a1c9de70086a220b28cdf8fd3eda8324dc898c))


### Bug Fixes

* auto deregister hotkeys when focused app is excluded ([#26](https://github.com/y3owk1n/neru/issues/26)) ([b5811ef](https://github.com/y3owk1n/neru/commit/b5811ef03354f9e6a1eddf87f29f842700f56113))
* ensure we can activate other modes when in a mode ([#21](https://github.com/y3owk1n/neru/issues/21)) ([5f8e29e](https://github.com/y3owk1n/neru/commit/5f8e29eef9eb63be18891f7434e84a707380102d))
* make action hints style configurable to be visually difference ([#22](https://github.com/y3owk1n/neru/issues/22)) ([8510d03](https://github.com/y3owk1n/neru/commit/8510d030828bd8e2fc7bd6377f8b7717339043f6))
* remove clickable role injection ([#19](https://github.com/y3owk1n/neru/issues/19)) ([707eb8d](https://github.com/y3owk1n/neru/commit/707eb8d184249554b3e02a5c006f51fb70bd57e3))
* remove numeric hint, feels useless ([#24](https://github.com/y3owk1n/neru/issues/24)) ([8db6945](https://github.com/y3owk1n/neru/commit/8db694565e762a8a6b29e17e921f989732255bda))

## [1.0.3](https://github.com/y3owk1n/neru/compare/v1.0.2...v1.0.3) (2025-11-01)


### Bug Fixes

* real electron support (mess) ([#15](https://github.com/y3owk1n/neru/issues/15)) ([6c43bc1](https://github.com/y3owk1n/neru/commit/6c43bc17278f9ba23c3728a3af55b8e7567d5842))

## [1.0.2](https://github.com/y3owk1n/neru/compare/v1.0.1...v1.0.2) (2025-11-01)


### Bug Fixes

* allow to unset global hotkeys ([#10](https://github.com/y3owk1n/neru/issues/10)) ([a0fc0bf](https://github.com/y3owk1n/neru/commit/a0fc0bfec23673e07b9981fdf2b55a2a9c3ce4ab))
* ensure correct config exposes to `status` command ([#8](https://github.com/y3owk1n/neru/issues/8)) ([f2e3f1e](https://github.com/y3owk1n/neru/commit/f2e3f1ead92dc412bddabb0082bd0f851a3eeb4e))
* remove reload config ([#11](https://github.com/y3owk1n/neru/issues/11)) ([a493743](https://github.com/y3owk1n/neru/commit/a4937433777f40aee7ced39653aa168a4f738020))
* remove root run and requires `launch` command to start daemon ([#12](https://github.com/y3owk1n/neru/issues/12)) ([efbf00a](https://github.com/y3owk1n/neru/commit/efbf00a88388cd66618ec8fe79f69ce117728902))

## [1.0.1](https://github.com/y3owk1n/neru/compare/v1.0.0...v1.0.1) (2025-11-01)


### Bug Fixes

* ensure action to run with `cgo` enabled ([#6](https://github.com/y3owk1n/neru/issues/6)) ([0963c66](https://github.com/y3owk1n/neru/commit/0963c66e5ce204048f91c06ffedf20fec572f37f))
* ensure checking from ./bin instead of ./build ([#4](https://github.com/y3owk1n/neru/issues/4)) ([8ee6942](https://github.com/y3owk1n/neru/commit/8ee6942ab960d81a0b3f3409443cbd076162d3e8))

## 1.0.0 (2025-11-01)


### Features

* actually implement middle click ([3783a60](https://github.com/y3owk1n/neru/commit/3783a6008ebfc0aad4718d4bc151f8e2ee58d1cb))
* add ci ([#1](https://github.com/y3owk1n/neru/issues/1)) ([0aab5c0](https://github.com/y3owk1n/neru/commit/0aab5c0e0d23f8c76f8b116e777cdd04fc05f239))
* add hint support for dock and menubar ([7c08d53](https://github.com/y3owk1n/neru/commit/7c08d533304fffa8eafccde51bffd8a31789161e))
* allow additional clickable roles in config ([f893bb4](https://github.com/y3owk1n/neru/commit/f893bb4fe3da2680cf01ea4a369b66e7cfad22f2))
* better hints ([4685a7f](https://github.com/y3owk1n/neru/commit/4685a7f1e219cc73f1673192a915afdaa4142746))
* init project with implementation ([43d255e](https://github.com/y3owk1n/neru/commit/43d255ee3995f4dbaea09ab5c75c5eeb6454a404))
* initial try to support electron ([86db6f7](https://github.com/y3owk1n/neru/commit/86db6f7611b7059fd54827cf87bf8a2e74ed809a))
* more hint modes ([3badbbb](https://github.com/y3owk1n/neru/commit/3badbbb22254ed7a1342ddf1f895eeb21957d1e1))
* nicer action hints ([2a712b0](https://github.com/y3owk1n/neru/commit/2a712b0b71bae4aa7a9d4ef54c799cac0ca0d7df))
* nicer hint with arrow ([e83c5b8](https://github.com/y3owk1n/neru/commit/e83c5b86a72acb9a2008d79fcf6fd9a135170c03))
* only check visible child for ax ([2c1252b](https://github.com/y3owk1n/neru/commit/2c1252b79a7229d81854aaebebe7178f60ab8b27))
* support `-v` and `--version` ([ba29ea1](https://github.com/y3owk1n/neru/commit/ba29ea183b6fbb5ecb48e2d865037378a4afaf35))
* support backspace to go back during hint mode ([8399942](https://github.com/y3owk1n/neru/commit/8399942e996f0d192478de8bc34eb508cfcb4df5))
* support global and per-app ax roles ([677e1c4](https://github.com/y3owk1n/neru/commit/677e1c4e3f4f7bbd63aa2ac9087ed26ba012204c))
* switch to cobra-cli with IPC socket for normal actions ([958989e](https://github.com/y3owk1n/neru/commit/958989e21f325fe30c85d27cd5721d9b6b9df426))


### Bug Fixes

* actually properly support chrome and electron ([5f6ec2d](https://github.com/y3owk1n/neru/commit/5f6ec2d22aa0d3893894507ec6db3be4e64d52b6))
* add fallback and validation for empty hint_characters ([36b8c64](https://github.com/y3owk1n/neru/commit/36b8c645ba5ea29953946201e76d6dda3a864067))
* configurable roles for scroll and hint ([735c98c](https://github.com/y3owk1n/neru/commit/735c98ceffe8a60392467b462ba8b686940f4327))
* ensure config for hint styles are passed through ([8679355](https://github.com/y3owk1n/neru/commit/867935560a17f9d2fbce1978ee581dfda9fddae3))
* ensure event loop runs in main thread ([6826fdb](https://github.com/y3owk1n/neru/commit/6826fdb05e6b9373135562d7f6bdb38e21d35ad7))
* ensure to flush event after clicks ([cb1d5ab](https://github.com/y3owk1n/neru/commit/cb1d5ab5c7cf4cdbf5010c87576755752eb48f09))
* explicit validation for scrollAreaByNumber ([bdbce7f](https://github.com/y3owk1n/neru/commit/bdbce7f4992d728b8ab583ffa840245925b8654b))
* fallback to actual click if press action failed ([e186bb1](https://github.com/y3owk1n/neru/commit/e186bb1dad3d41f81ef0c9352e963ca1be555b03))
* make matched hint color text configurable ([fd0980f](https://github.com/y3owk1n/neru/commit/fd0980f9ddd18de75e5e2fc7e09182731aab683b))
* make sure `ctrl-c` actually kills the program ([a566cf9](https://github.com/y3owk1n/neru/commit/a566cf958db4441399df4a446b45b63b69015245))
* more logs for additional accessibility ([407f992](https://github.com/y3owk1n/neru/commit/407f99273a2c9f4c03455482d2f5aedefe29556f))
* refactor scroll magic numbers to be constants ([521df2a](https://github.com/y3owk1n/neru/commit/521df2acfd3e94ea1007b341773d7bbc8ce86365))
* remove `escape` mapping ([a795e2b](https://github.com/y3owk1n/neru/commit/a795e2bfb2d15fd154578d575761210c35f470db))
* remove action mapping from config ([6974c7d](https://github.com/y3owk1n/neru/commit/6974c7db0ce742e4fafa43ef3e3d9ff002f5d95e))
* remove stupid animations ([97a390c](https://github.com/y3owk1n/neru/commit/97a390c1734f5fa86ee6c9928c5c489bf7ce4247))
* remove unneeded smoothscroll function ([fb77bdc](https://github.com/y3owk1n/neru/commit/fb77bdc4163b6568e9dff8647485d74b3de37d2c))
* remove unused config ([23df329](https://github.com/y3owk1n/neru/commit/23df3293eae53f1f542aa5c26f0abf2a680bd51f))
* some improvement for pre-production ([b0e5a79](https://github.com/y3owk1n/neru/commit/b0e5a79cb3fac447482d50058f165c6492dcaeff))
* sort of working scroll mechanism ([cd4376a](https://github.com/y3owk1n/neru/commit/cd4376a83ffb5f6910d8e68f3fc423d32d098b03))
* support 3 characters hint without clashes ([7120ded](https://github.com/y3owk1n/neru/commit/7120ded044132cfe155665d5cfa70122201dea11))
