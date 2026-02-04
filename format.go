package bcn

// Format identifies a BCn/DXT compression format.
//
// The format controls block size and how channels are interpreted:
// - DXT1: RGB (optionally 1-bit alpha via 3-color mode)
// - DXT3: RGBA with explicit 4-bit alpha
// - DXT5: RGBA with interpolated alpha
// - BC4: single channel (stored in red, replicated on decode)
// - BC5: two channels (stored in red/green, blue=0 on decode)
type Format int

const (
	// FormatUnknown is a sentinel for unsupported/unknown formats.
	FormatUnknown Format = iota
	// FormatDXT1 is BC1/DXT1 (8 bytes per 4x4 block).
	FormatDXT1
	// FormatDXT3 is BC2/DXT3 (16 bytes per 4x4 block).
	FormatDXT3
	// FormatDXT5 is BC3/DXT5 (16 bytes per 4x4 block).
	FormatDXT5
	// FormatBC4 is BC4/ATI1 (8 bytes per 4x4 block, single channel).
	FormatBC4
	// FormatBC5 is BC5/ATI2 (16 bytes per 4x4 block, two channels).
	FormatBC5
	// FormatRGBA8 is uncompressed RGBA (4 bytes per pixel).
	FormatRGBA8
	// FormatBGRA8 is uncompressed BGRA (4 bytes per pixel).
	FormatBGRA8
)

func (f Format) String() string {
	switch f {
	case FormatDXT1:
		return "DXT1"
	case FormatDXT3:
		return "DXT3"
	case FormatDXT5:
		return "DXT5"
	case FormatBC4:
		return "BC4"
	case FormatBC5:
		return "BC5"
	case FormatRGBA8:
		return "RGBA8"
	case FormatBGRA8:
		return "BGRA8"
	default:
		return "Unknown"
	}
}

func (f Format) blockSize() int {
	switch f {
	case FormatDXT1:
		return 8
	case FormatDXT3, FormatDXT5:
		return 16
	case FormatBC4:
		return 8
	case FormatBC5:
		return 16
	case FormatRGBA8, FormatBGRA8:
		return 4
	default:
		return 0
	}
}

func (f Format) isCompressed() bool {
	switch f {
	case FormatDXT1, FormatDXT3, FormatDXT5, FormatBC4, FormatBC5:
		return true
	default:
		return false
	}
}
