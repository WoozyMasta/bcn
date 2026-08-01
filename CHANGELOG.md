<!-- markdownlint-disable MD024 -->
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

* DX10 DDS read support for RGBA8/BGRA8 and sRGB BC1–BC3 formats.
* DX10 DDS support for `B8G8R8X8_UNORM` and `B8G8R8X8_UNORM_SRGB`
  as `FormatBGRX8`, with the unused X channel normalized to `255`.
* Signed normalized BC4 and BC5 (`BC4_SNORM`/`BC5_SNORM`) encode and decode,
  with DDS DX10 and KTX v1 I/O.
  AVX2/BMI2 decoding on amd64 with a pure-Go fallback.

## [0.6.0][] - 2026-08-02

### Added

* BC6H/BPTC-HDR encode and decode (unsigned `BC6H_UF16` and signed `BC6H_SF16`),
  with DDS (DX10) and KTX v1 I/O.
  Input/output is `[]uint16` or `[]float32` RGB half-float
  via `EncodeBC6H` / `DecodeBC6H`.
  AVX2 kernels accelerate the nearest-palette search;
  pure-Go fallback everywhere else.
* BC7/BPTC (unorm) encode and decode for all 8 modes,
  with DDS (DX10) and KTX v1 I/O.
  Encoder picks the lowest-error mode per block,
  with `QualityLevel` trading encode time for quality.
  Hot paths use the existing AVX2 path with a pure-Go fallback.
* External `bcdec` compatibility fixtures for BC1–BC7 decoding,
  including unsigned and signed BC6H.
* Canonical BC1, BC2, and BC3 API names;
  existing DXT1, DXT3, and DXT5 names remain compatibility aliases.
* Runnable API examples for image, DDS, and BC6H encoding.

### Fixed

* BC2/BC3 decoding now uses the required four-color palette when `c0 <= c1`.

[0.6.0]: https://github.com/WoozyMasta/bcn/compare/v0.5.0...v0.6.0

## [0.5.0][] - 2026-06-18

### Added

* `EncodeImageInto` and `DecodeImageInto` encode/decode
  into a caller-owned buffer (reallocated only when too small).
* `GenerateMipmapsInto` builds a mip chain reusing
  the per-level NRGBA buffers across calls.
* `ErrBufferTooSmall` for a destination buffer smaller than the encoded size.

### Changed

* `EncodeImageWithOptions` reads an already-tight,
  origin-anchored `*image.NRGBA` in place instead of allocating
  and copying a full `width*height*4` buffer.
* `DecodeImage` / `DecodeImageWithOptions` decode straight into the destination
  `image.NRGBA.Pix` instead of decoding into a temporary buffer and copying.

Both cut about `width*height*4` bytes per call (~4 MiB on 1024^2)
with byte-identical output.

[0.5.0]: https://github.com/WoozyMasta/bcn/compare/v0.4.0...v0.5.0

## [0.4.0][] - 2026-06-18

### Added

* `GenerateMipmapsN` for generating mipmap chains
  with an optional maximum mip level count while keeping `GenerateMipmaps`
  as the compatibility wrapper.

### Changed

* Faster mipmap generation for `*image.NRGBA`,
  including direct byte access, integer averaging,
  and an AVX2 downscale row kernel on `amd64` with pure-Go fallback.

[0.4.0]: https://github.com/WoozyMasta/bcn/compare/v0.3.0...v0.4.0

## [0.3.0][] - 2026-06-17

### Added

* Least-squares (LSQ) endpoint refit for BC1, BC3, BC4 and BC5 encoders,
  with `RefinementOptions.LSQIters` to tune or disable the pass.

### Changed

* `QualityLevelBalanced` and higher presets now include LSQ by default.
* Faster least-squares refit on `amd64` via AVX2 acceleration,
  with pure-Go fallback everywhere else.

[0.3.0]: https://github.com/WoozyMasta/bcn/compare/v0.2.0...v0.3.0

## [0.2.0][] - 2026-06-17

### Added

* SIMD acceleration on `amd64`: AVX2/SSE2 (and BMI2 for DXT3/DXT5/BC4/BC5
  decode) kernels for the hot encode/decode paths, selected at runtime via
  `golang.org/x/sys/cpu`. A pure-Go fallback runs everywhere else and on edge
  blocks, and is byte-exact with the kernels.
* Opt-out for the assembly: `BCN_PUREGO=1` forces the pure-Go path at runtime,
  `-tags purego` excludes the assembly from the build entirely.

### Changed

* Much faster single-thread throughput on AVX2 CPUs vs the previous release:
  decode about 6x to 13x (DXT1 ~6x, DXT3/DXT5 ~7-8x, BC4 ~11x, BC5 ~13x);
  encode best/balanced about 6.5x to 9.5x across all formats;
  encode fast about 2x to 4x.
* The encoder now scores color/alpha error with a
  deterministic integer metric instead of float64.
  Quality is unchanged (PSNR within about 0.01 dB),
  but the encoded bytes can differ slightly from previous releases
  at the same quality level.

[0.2.0]: https://github.com/WoozyMasta/bcn/compare/v0.1.5...v0.2.0

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
