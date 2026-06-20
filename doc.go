// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

/*
Package bcn provides BCn/DXT block compression encode/decode and container I/O.

The package focuses on practical texture workflows:
  - Encode/decode BC1/DXT1, BC2/DXT3, BC3/DXT5, BC4, BC5, BC7/BPTC
  - Read/write DDS and KTX v1 containers
  - Optional mipmap generation with sRGB-aware downscaling

The core encode/decode APIs operate on NRGBA byte layout (R,G,B,A per pixel).
For best results, ensure inputs are in the expected color space (typically sRGB)
and pick an appropriate QualityLevel in EncodeOptions.

DDS BGRA pixels are converted to RGBA on decode. Uncompressed DDS supports
RGBA and BGRA input when writing.
*/
package bcn
