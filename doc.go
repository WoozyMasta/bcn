// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

/*
Package bcn provides BCn/DXT block compression encode/decode and container I/O.

The package focuses on practical texture workflows:
  - Encode/decode BC1/BC1, BC2/BC2, BC3/BC3, BC4, BC5, BC6H/BPTC-HDR, BC7/BPTC
  - Read/write DDS and KTX v1 containers
  - Optional mipmap generation with sRGB-aware downscaling

The core encode/decode APIs operate on NRGBA byte layout (R,G,B,A per pixel).
BC6H is the exception: its API uses []uint16 (or []float32) RGB half-float input,
via EncodeBC6H / DecodeBC6H and their variants.
For best results, ensure inputs are in the expected color space (typically sRGB)
and pick an appropriate QualityLevel in EncodeOptions.

DDS BGRA pixels are converted to RGBA on decode. Uncompressed DDS supports
RGBA and BGRA input when writing.
*/
package bcn
