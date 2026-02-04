# bcn

Minimal, fast BCn/DXT encoder/decoder in Go with DDS and KTX v1 I/O.

`bcn` provides block compression (BCn/DXT) encode/decode plus container I/O.
It targets practical texture workflows:
encode from images,
decode to images,
and read/write DDS/KTX with mipmaps and cubemaps.

## Implemented

* BC1/DXT1, BC2/DXT3, BC3/DXT5, BC4, BC5 encode/decode
* DDS read/write (2D + cubemap, mipmaps, uncompressed RGBA/BGRA)
* KTX v1 read/write (2D + cubemap, mipmaps)
* Mipmap generation with optional sRGB-aware downscale
* Quality presets: `fast`, `balanced`, `best`

## Usage

### Encode image to DDS

```go
img, _, _ := image.Decode(in)
opts := &bcn.EncodeOptions{
  Quality: bcn.QualityBalanced,
  GenerateMipmaps: true,
  UseSRGB: true,
}

dds, err := bcn.EncodeDDSWithOptions([]image.Image{img}, bcn.FormatDXT5, opts)
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
ktx, err := bcn.EncodeKTXWithOptions([]image.Image{img}, bcn.FormatBC5, &bcn.EncodeOptions{Quality: bcn.QualityFast})
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
* DDS DX10 header is read for BC1/3/5 and BC4/5; writing uses legacy FourCC.
* BC4 uses red channel; BC5 uses red/green.
* DDS BGRA is converted to RGBA on decode; RGBA/BGRA are supported for uncompressed DDS.
