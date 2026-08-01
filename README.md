# bcn

Minimal, fast BCn/DXT encoder/decoder in pure-Go-compatible
with AVX2 acceleration and with DDS and KTX v1 I/O.

`bcn` provides block compression (BCn/DXT) encode/decode plus container I/O.
It targets practical texture workflows:
encode from images,
decode to images,
and read/write DDS/KTX with mipmaps and cubemaps.

## Implemented

* BC1/DXT1, BC2/DXT3, BC3/DXT5, BC4, BC5, BC6H/BPTC-HDR, BC7/BPTC encode/decode
* DDS read/write (2D + cubemap, mipmaps, uncompressed RGBA/BGRA)
* KTX v1 read/write (2D + cubemap, mipmaps)
* Mipmap generation with optional sRGB-aware downscale
* Quality levels (1..10) with least-squares endpoint refit
  and refinement overrides (`Refinement`)
* Parallel encoding control via `EncodeOptions.Workers` (0=auto, 1=off)
* Parallel decoding control via `DecodeOptions.Workers` (0=auto, 1=off)

> [!NOTE]  
> For large images or one‑by‑one encoding, use internal parallelism (default).  
> For batch/many small files, parallelize across images in your own code
> and keep `Workers=1` here.  
> `Workers=0` uses `GOMAXPROCS` (Go scheduler's CPU limit).

## Usage

### Encode image to DDS

```go
img, _, _ := image.Decode(in)
opts := &bcn.EncodeOptions{
  QualityLevel: bcn.QualityLevelBalanced,
  GenerateMipmaps: true,
  UseSRGB: true,
}

dds, err := bcn.EncodeDDSWithOptions([]image.Image{img}, bcn.FormatBC3, opts)
if err != nil {
  /* handle */
}
_ = dds.Write(out)
```

### Decode DDS to image

```go
dds, err := bcn.ReadDDS(in)
if err != nil {
  /* handle */
}
img, err := bcn.DecodeImage(dds.Faces[0].Mipmaps[0], dds.Width, dds.Height, dds.Format)
if err != nil {
  /* handle */
}
_ = png.Encode(out, img)
```

### Read DDS header only

```go
hdr, dx10, err := bcn.ReadDDSHeader(in)
if err != nil {
  /* handle */
}
_ = hdr
_ = dx10
```

### Encode to KTX

```go
ktx, err := bcn.EncodeKTXWithOptions([]image.Image{img}, bcn.FormatBC5, &bcn.EncodeOptions{QualityLevel: bcn.QualityLevelFast})
if err != nil {
  /* handle */
}
_ = ktx.Write(out)
```

### Use with standard image.Decode

Import the subpackages to register DDS and KTX with the `image` package;
then `image.Decode` and `image.DecodeConfig` work as usual:

```go
import (
  _ "github.com/woozymasta/bcn/dds"
  _ "github.com/woozymasta/bcn/ktx"
)

// ...
img, _, _ := image.Decode(f)       // decodes first face/mip to NRGBA
cfg, _, _ := image.DecodeConfig(f) // width, height only
```

## Notes

* Only compressed KTX v1 is supported (no arrays/3D).
* DDS DX10 header is read for BC1/3/5 and BC4/5; writing uses legacy FourCC,
  except BC7 which is always written with a DX10 header (`BC7_UNORM`).
* BC4 uses red channel; BC5 uses red/green.
* DDS BGRA is converted to RGBA on decode;
  RGBA/BGRA are supported for uncompressed DDS.
* `Refinement` overrides `QualityLevel` when set.
* Quality levels above 1 polish endpoints
  with an iterated least-squares refit on top of the grid search
  (higher quality, some extra encode cost; decode is unaffected).  
  Disable or tune it via `Refinement.LSQIters`
  (`0` = off, `nil` = quality default, `N` = iterations);
  set `ColorTries: 0` with `LSQIters > 0` for a cheap LSQ-only refine.

## Acceleration

On `amd64` the hot encode/decode paths use AVX2/SSE2 assembly kernels
(in the `internal/simd` package, generated with [avo][])
selected at runtime via `golang.org/x/sys/cpu`;
a portable pure-Go fallback handles every other platform
and any block the kernels do not cover
(e.g. edge blocks when width or height is not a multiple of 4).
The two paths are byte-exact -
validated by exhaustive, randomized and fuzz equivalence tests.

* AVX2 kernels need AVX2
  (decode of BC1 needs AVX2; BC2/BC3/BC4/BC5 decode needs AVX2+BMI2).
  Without them the pure-Go path runs.
* `BCN_PUREGO=1` in the environment forces the pure-Go path at runtime;
  building with `-tags purego` excludes the assembly entirely.
* The avo generator lives in its own build-time-only module
  (`internal/simd/asmgen`), so consumers never pull avo into their module graph;
  the only runtime dependency is `golang.org/x/sys`.
* Regenerate kernels after editing `internal/simd/asmgen` with `make generate`;
  `make generate-check` (part of CI) verifies the committed `.s` is up to date.
* Set `GOAMD64=v2`/`v3` to let the Go compiler also vectorize the fallback
  and container helpers;
  it does not affect the hand-written kernels.

## Performance

Single-thread, Ryzen 9 5950X, Go 1.26, 512x512, throughput over input bytes
(RGBA for LDR formats, RGB float16 for BC6H; higher is better):

| Format    | fast, MB/s | balanced, MB/s | best, MB/s | decode, MB/s |
| --------- | ---------: | -------------: | ---------: | -----------: |
| **BC1**   |       ~830 |            ~35 |        ~12 |         ~920 |
| **BC2**   |       ~635 |            ~33 |        ~12 |        ~1715 |
| **BC3**   |       ~370 |            ~31 |        ~11 |        ~1670 |
| **BC4**   |       ~480 |            ~74 |        ~23 |        ~1530 |
| **BC5**   |       ~255 |            ~38 |        ~12 |        ~2490 |
| **BC6H**  |       ~150 |             ~8 |         ~2 |         ~150 |
| **BC7**   |        ~55 |           ~0.9 |       ~0.5 |          ~66 |

Multi-thread, `Workers=auto` (`GOMAXPROCS=32`), 512x512,
encode throughput over input bytes (higher is better):

| Format    | fast, MB/s | balanced, MB/s | best, MB/s |
| --------- | ---------: | -------------: | ---------: |
| **BC1**   |     ~5,000 |           ~380 |       ~144 |
| **BC3**   |     ~2,620 |           ~370 |       ~130 |
| **BC6H**  |     ~1,350 |            ~90 |        ~30 |
| **BC7**   |       ~540 |            ~13 |         ~7 |

Fast/Balanced/Best correspond to
`QualityLevelFast`, `QualityLevelBalanced`, `QualityLevelBest`.  
For batch/many small files,
parallelize across images in your own code and keep `Workers=1`;
see `EncodeOptions.Workers`.

[avo]: https://github.com/mmcloughlin/avo
