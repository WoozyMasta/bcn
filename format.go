// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// Format identifies a BCn compression format.
//
// The format controls block size and how channels are interpreted:
// - BC1: RGB (optionally 1-bit alpha via 3-color mode)
// - BC2: RGBA with explicit 4-bit alpha
// - BC3: RGBA with interpolated alpha
// - BC4: single channel (stored in red, replicated on decode)
// - BC5: two channels (stored in red/green, blue=0 on decode)
type Format int

const (
	// FormatUnknown is a sentinel for unsupported/unknown formats.
	FormatUnknown Format = iota
	// FormatBC1 is BC1 (formerly DXT1; 8 bytes per 4x4 block).
	FormatBC1
	// FormatBC2 is BC2 (formerly DXT3; 16 bytes per 4x4 block).
	FormatBC2
	// FormatBC3 is BC3 (formerly DXT5; 16 bytes per 4x4 block).
	FormatBC3
	// FormatBC4 is BC4/ATI1 (8 bytes per 4x4 block, single channel).
	FormatBC4
	// FormatBC5 is BC5/ATI2 (16 bytes per 4x4 block, two channels).
	FormatBC5
	// FormatRGBA8 is uncompressed RGBA (4 bytes per pixel).
	FormatRGBA8
	// FormatBGRA8 is uncompressed BGRA (4 bytes per pixel).
	FormatBGRA8
	// FormatBC7 is BC7/BPTC unorm RGBA (16 bytes per 4x4 block).
	FormatBC7
	// FormatBC6HU is BC6H unsigned float RGB HDR (16 bytes per 4x4 block).
	// Use DecodeBC6H / EncodeBC6H; the NRGBA byte API returns ErrBC6HUsesHDRAPI.
	FormatBC6HU
	// FormatBC6HS is BC6H signed float RGB HDR (16 bytes per 4x4 block).
	// Use DecodeBC6H / EncodeBC6H; the NRGBA byte API returns ErrBC6HUsesHDRAPI.
	FormatBC6HS
	// FormatBC4S is BC4 signed normalized (8 bytes per 4x4 block, single channel).
	// NRGBA input/output maps 0..255 to -1..1; the decoded value is replicated to RGB.
	FormatBC4S
	// FormatBC5S is BC5 signed normalized (16 bytes per 4x4 block, two channels).
	// NRGBA input/output maps R/G from 0..255 to -1..1; decoded B=0 and A=255.
	FormatBC5S
	// FormatBGRX8 is uncompressed BGR with an unused byte (4 bytes per pixel).
	// Encoding and decoding always set the unused byte/alpha to 255.
	FormatBGRX8
	// FormatR8 is uncompressed single-channel UNORM (1 byte per pixel).
	// Decoding replicates R to RGB and sets A to 255.
	FormatR8
	// FormatRG8 is uncompressed two-channel UNORM (2 bytes per pixel).
	// Decoding writes R and G, with B=0 and A=255.
	FormatRG8
	// FormatRGB10A2 is packed 10-bit RGB plus 2-bit alpha UNORM (4 bytes per pixel).
	FormatRGB10A2
	// FormatR8S is uncompressed single-channel SNORM (1 byte per pixel).
	// NRGBA input/output maps 0..255 to -1..1; decoding replicates R to RGB.
	FormatR8S
	// FormatRG8S is uncompressed two-channel SNORM (2 bytes per pixel).
	// NRGBA input/output maps R/G from 0..255 to -1..1; decoded B=0 and A=255.
	FormatRG8S
	// FormatA8 is uncompressed alpha-only UNORM (1 byte per pixel).
	// Decoding writes RGB=0 and preserves alpha.
	FormatA8
	// FormatRGB565 is packed 5-bit R, 6-bit G, 5-bit B UNORM (2 bytes per pixel).
	FormatRGB565
	// FormatRGBA5551 is packed 5-bit RGB plus 1-bit alpha UNORM (2 bytes per pixel).
	// The little-endian word layout is A1:R5:G5:B5.
	FormatRGBA5551
	// FormatRGBA4444 is packed 4-bit RGBA UNORM (2 bytes per pixel).
	// The little-endian word layout is A4:R4:G4:B4.
	FormatRGBA4444
	// FormatRGB8 is uncompressed RGB UNORM (3 bytes per pixel).
	FormatRGB8
	// FormatBGR8 is uncompressed BGR UNORM (3 bytes per pixel).
	FormatBGR8
)

const (
	// FormatDXT1 is a compatibility alias for FormatBC1.
	FormatDXT1 = FormatBC1
	// FormatDXT3 is a compatibility alias for FormatBC2.
	FormatDXT3 = FormatBC2
	// FormatDXT5 is a compatibility alias for FormatBC3.
	FormatDXT5 = FormatBC3
)

func (f Format) String() string {
	switch f {
	case FormatBC1:
		return "BC1"
	case FormatBC2:
		return "BC2"
	case FormatBC3:
		return "BC3"
	case FormatBC4:
		return "BC4"
	case FormatBC5:
		return "BC5"
	case FormatBC7:
		return "BC7"
	case FormatBC6HU:
		return "BC6HU"
	case FormatBC6HS:
		return "BC6HS"
	case FormatBC4S:
		return "BC4S"
	case FormatBC5S:
		return "BC5S"
	case FormatRGBA8:
		return "RGBA8"
	case FormatBGRA8:
		return "BGRA8"
	case FormatBGRX8:
		return "BGRX8"
	case FormatR8:
		return "R8"
	case FormatRG8:
		return "RG8"
	case FormatRGB10A2:
		return "RGB10A2"
	case FormatR8S:
		return "R8S"
	case FormatRG8S:
		return "RG8S"
	case FormatA8:
		return "A8"
	case FormatRGB565:
		return "RGB565"
	case FormatRGBA5551:
		return "RGBA5551"
	case FormatRGBA4444:
		return "RGBA4444"
	case FormatRGB8:
		return "RGB8"
	case FormatBGR8:
		return "BGR8"
	default:
		return "Unknown"
	}
}

// blockSize returns bytes per block (compressed) or per pixel (uncompressed).
func (f Format) blockSize() int {
	switch f {
	case FormatBC1:
		return 8
	case FormatBC2, FormatBC3:
		return 16
	case FormatBC4, FormatBC4S:
		return 8
	case FormatBC5, FormatBC5S:
		return 16
	case FormatBC7, FormatBC6HU, FormatBC6HS:
		return 16
	case FormatRGBA8, FormatBGRA8, FormatBGRX8, FormatRGB10A2:
		return 4
	case FormatR8, FormatR8S, FormatA8:
		return 1
	case FormatRG8, FormatRG8S:
		return 2
	case FormatRGB565, FormatRGBA5551, FormatRGBA4444:
		return 2
	case FormatRGB8, FormatBGR8:
		return 3
	default:
		return 0
	}
}

// isCompressed reports whether the format uses BCn block compression.
func (f Format) isCompressed() bool {
	switch f {
	case FormatBC1, FormatBC2, FormatBC3, FormatBC4, FormatBC5, FormatBC4S, FormatBC5S, FormatBC7, FormatBC6HU, FormatBC6HS:
		return true
	default:
		return false
	}
}
