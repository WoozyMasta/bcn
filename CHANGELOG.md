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

## [0.1.5][] - 2026-02-17

### Changed

* Internal refactor of DDS/KTX encoding pipeline
  (shared face/mipmap encoding path).
* Codebase cleanup for stricter lint rules
  (modernized loops, safer indexing patterns).
* Build/CI maintenance: updated linter/tooling setup and benchmark targets.
* Added SPDX file headers across package sources.
* Improved code documentation with more detailed comments.

[0.1.5]: https://github.com/WoozyMasta/bcn/compare/v0.1.4...v0.1.5

## [0.1.4][] - 2026-02-10

### Added

* `DecodeDDSWithOptions` and `DecodeKTXWithOptions` to decode first
  face/mip with `DecodeOptions` (e.g. `Workers`)
* KTX uncompressed support read/write RGBA8 and BGRA8 (parity with DDS)
* KTX GL constants for uncompressed format
  `KTXGLUnsignedByte`, `KTXGLBGRA`, `KTXGLRGBA8`
* `ktxHeaderFormats` and helpers for KTX uncompressed layout
  (row stride, bottom-up <-> tight top-down)
* Round-trip test for uncompressed KTX (`TestKTXUncompressedRoundTrip`)

### Changed

* `DecodeDDS` / `DecodeKTX` now delegate to
  `DecodeDDSWithOptions` / `DecodeKTXWithOptions` with nil options
* KTX `ErrUnsupportedKTXCompressed` renamed to `ErrUnsupportedKTXUncompressed`
  (used for unsupported uncompressed formats)
* KTX write accepts `FormatRGBA8` and `FormatBGRA8`;
  header uses `ktxHeaderFormats` for both compressed and uncompressed

[0.1.4]: https://github.com/WoozyMasta/bcn/compare/v0.1.3...v0.1.4

## [0.1.3][] - 2026-02-06

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

[0.1.3]: https://github.com/WoozyMasta/bcn/compare/v0.1.2...v0.1.3

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
