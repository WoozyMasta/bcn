# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.2][] - 2026-02-05

### Added

* configurable RGB weights with
  new `RGBWeights` type and option in `EncodeOptions`
* more roundtrip tests

[0.1.2]: https://github.com/WoozyMasta/bcn/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2026-02-04

### Added

* `DecodeKTX` decode first face/mip of KTX to `*image.NRGBA`
* Subpackages `bcn/dds` and `bcn/ktx` registers DDS/KTX with
  `image.RegisterFormat` for use with `image.Decode` / `image.DecodeConfig`

[0.1.1]: https://github.com/WoozyMasta/bcn/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-02-04

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/bcn/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
