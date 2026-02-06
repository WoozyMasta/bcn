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

## Unreleased

### Added

* Quality tuning with `QualityLevel` (1..10) and `RefinementOptions` overrides
* Parallel encoding control via `EncodeOptions.Workers`
  with worker pool/thresholds
* Parallel decoding control via `DecodeOptions.Workers`
  and `Decode*WithOptions`
* Encode/decode benchmarks and Makefile targets
  (`bench-encode`, `bench-decode`, baseline/compare)

### Changed

* DXT1/DXT5 encoding significantly faster with fewer allocations
  (refinement, PCA, alpha paths)
* Fast paths for block extract/store and other hot loops
* Encoding/decoding can now run in parallel by default
  (uses `GOMAXPROCS` for large images);
  set `Workers=1` to force serial behavior

### Removed (Breaking)

* `Quality` enum and `EncodeOptions.Quality`
  (use `QualityLevel` or `QualityLevelFast/Balanced/Best`)

## [0.1.2][] - 2026-02-05

### Added

* configurable RGB weights with
  new `RGBWeights` type and option in `EncodeOptions`
* more roundtrip tests

[0.1.2]: https://github.com/WoozyMasta/bcn/compare/v0.1.1...v0.1.2
[Unreleased]: https://github.com/WoozyMasta/bcn/compare/v0.1.2...HEAD

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
