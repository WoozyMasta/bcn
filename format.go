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
	case FormatRGBA8, FormatBGRA8, FormatBGRX8:
		return 4
	case FormatR8:
		return 1
	case FormatRG8:
		return 2
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
