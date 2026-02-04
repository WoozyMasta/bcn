package bcn

// DDS constants and header structures.

const (
	// DDSMagic is the file signature "DDS ".
	DDSMagic = 0x20534444

	// DDSHeaderSize is the size of DDS_HEADER.
	DDSHeaderSize = 124
	// DDSPixelFormatSize is the size of DDS_PIXELFORMAT.
	DDSPixelFormatSize = 32

	// DDS header flags

	// DDSFlagCaps marks caps field as valid.
	DDSFlagCaps = 0x1
	// DDSFlagHeight marks height field as valid.
	DDSFlagHeight = 0x2
	// DDSFlagWidth marks width field as valid.
	DDSFlagWidth = 0x4
	// DDSFlagPitch marks pitch field as valid.
	DDSFlagPitch = 0x8
	// DDSFlagPixelFormat marks pixel format as valid.
	DDSFlagPixelFormat = 0x1000
	// DDSFlagMipmapCount marks mipmap count as valid.
	DDSFlagMipmapCount = 0x20000
	// DDSFlagLinearSize marks linear size as valid.
	DDSFlagLinearSize = 0x80000
	// DDSFlagDepth marks depth as valid.
	DDSFlagDepth = 0x800000

	// DDS pixel format flags

	// DDSPFAlphaPixels indicates alpha data is present.
	DDSPFAlphaPixels = 0x1
	// DDSPFAlpha indicates alpha-only data.
	DDSPFAlpha = 0x2
	// DDSPFFourCC indicates a FourCC format.
	DDSPFFourCC = 0x4
	// DDSPFRGB indicates uncompressed RGB data.
	DDSPFRGB = 0x40
	// DDSPFYUV indicates uncompressed YUV data.
	DDSPFYUV = 0x200
	// DDSPFLuminance indicates uncompressed luminance data.
	DDSPFLuminance = 0x20000

	// DDS caps flags

	// DDSCapsComplex indicates more than one surface (mips/cubemap/volume).
	DDSCapsComplex = 0x8
	// DDSCapsTexture indicates a texture.
	DDSCapsTexture = 0x1000
	// DDSCapsMipmap indicates mipmaps are present.
	DDSCapsMipmap = 0x400000

	// DDS caps2 flags

	// DDSCaps2Cubemap indicates cubemap faces are present.
	DDSCaps2Cubemap = 0x200

	// DDSFourCCDX10 is the DX10 FourCC ("DX10").
	DDSFourCCDX10 = 0x30315844
)

// DDSPixelFormat represents DDS_PIXELFORMAT.
type DDSPixelFormat struct {
	Size        uint32 // Size of the structure.
	Flags       uint32 // Flags.
	FourCC      uint32 // FourCC code.
	RGBBitCount uint32 // RGB bit count.
	RBitMask    uint32 // R bit mask.
	GBitMask    uint32 // G bit mask.
	BBitMask    uint32 // B bit mask.
	ABitMask    uint32 // A bit mask.
}

// DDSHeader represents DDS_HEADER.
type DDSHeader struct {
	Size              uint32         // Size of the structure.
	Flags             uint32         // Flags.
	Height            uint32         // Height of the texture.
	Width             uint32         // Width of the texture.
	PitchOrLinearSize uint32         // Pitch or linear size.
	Depth             uint32         // Depth of the texture.
	MipMapCount       uint32         // Number of mipmaps.
	Reserved1         [11]uint32     // Reserved1.
	PixelFormat       DDSPixelFormat // Pixel format.
	Caps              uint32         // Caps.
	Caps2             uint32         // Caps2.
	Caps3             uint32         // Caps3.
	Caps4             uint32         // Caps4.
	Reserved2         uint32         // Reserved2.
}

// DDSHeaderDX10 represents DDS_HEADER_DXT10 (DXGI_FORMAT_UNKNOWN).
type DDSHeaderDX10 struct {
	DXGIFormat        uint32 // DXGI format (DXGI_FORMAT).
	ResourceDimension uint32 // Resource dimension (D3D10_RESOURCE_DIMENSION).
	MiscFlag          uint32 // Misc flag (D3D10_MISC_FLAGS).
	ArraySize         uint32 // Array size (D3D10_ARRAY_SIZE).
	MiscFlags2        uint32 // Misc flags2 (D3D10_MISC_FLAGS2).
}

// CreateDDSHeaderRGBA8 creates a DDS header for RGBA8 (byte order R,G,B,A).
func CreateDDSHeaderRGBA8(width, height, mipMapCount uint32) *DDSHeader {
	flags := uint32(DDSFlagCaps | DDSFlagHeight | DDSFlagWidth | DDSFlagPixelFormat | DDSFlagPitch)
	if mipMapCount > 0 {
		flags |= DDSFlagMipmapCount
	}

	caps := uint32(DDSCapsTexture)
	if mipMapCount > 0 {
		caps |= DDSCapsComplex | DDSCapsMipmap
	}

	reserved1 := [11]uint32{
		0,
		0x31464e45, // "ENF1"
		0, 0, 0, 0, 0, 0, 0, 0, 0,
	}

	return &DDSHeader{
		Size:              DDSHeaderSize,
		Flags:             flags,
		Height:            height,
		Width:             width,
		PitchOrLinearSize: width * 4,
		Depth:             0,
		MipMapCount:       mipMapCount,
		Reserved1:         reserved1,
		PixelFormat: DDSPixelFormat{
			Size:        DDSPixelFormatSize,
			Flags:       DDSPFAlphaPixels | DDSPFRGB,
			FourCC:      0,
			RGBBitCount: 32,
			RBitMask:    0x00ff0000,
			GBitMask:    0x0000ff00,
			BBitMask:    0x000000ff,
			ABitMask:    0xff000000,
		},
		Caps:      caps,
		Caps2:     0,
		Caps3:     0,
		Caps4:     0,
		Reserved2: 0,
	}
}
