package bcn

// KTX constants and header structures.

// KTXIdentifier is the 12-byte KTX v1 file signature.
var KTXIdentifier = [12]byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x31, 0x31, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

const (
	// KTXEndianness is the canonical little-endian marker.
	KTXEndianness = 0x04030201

	// KTXGLUnsignedByte is GL_UNSIGNED_BYTE.
	KTXGLUnsignedByte = 0x1401
	// KTXGLRGB is GL_RGB.
	KTXGLRGB = 0x1907
	// KTXGLRGBA is GL_RGBA.
	KTXGLRGBA = 0x1908
	// KTXGLBGRA is GL_BGRA (extension).
	KTXGLBGRA = 0x80E1
	// KTXGLRed is GL_RED.
	KTXGLRed = 0x1903
	// KTXGLRG is GL_RG.
	KTXGLRG = 0x8227
	// KTXGLRGBA8 is GL_RGBA8 (sized internal format).
	KTXGLRGBA8 = 0x8058

	// KTXGLCompressedRGBS3TCDXT1 is GL_COMPRESSED_RGB_S3TC_DXT1_EXT.
	KTXGLCompressedRGBS3TCDXT1 = 0x83F0
	// KTXGLCompressedRGBAS3TCDXT1 is GL_COMPRESSED_RGBA_S3TC_DXT1_EXT.
	KTXGLCompressedRGBAS3TCDXT1 = 0x83F1
	// KTXGLCompressedRGBAS3TCDXT3 is GL_COMPRESSED_RGBA_S3TC_DXT3_EXT.
	KTXGLCompressedRGBAS3TCDXT3 = 0x83F2
	// KTXGLCompressedRGBAS3TCDXT5 is GL_COMPRESSED_RGBA_S3TC_DXT5_EXT.
	KTXGLCompressedRGBAS3TCDXT5 = 0x83F3
	// KTXGLCompressedRedRGTC1 is GL_COMPRESSED_RED_RGTC1.
	KTXGLCompressedRedRGTC1 = 0x8DBB
	// KTXGLCompressedRGRGTC2 is GL_COMPRESSED_RG_RGTC2.
	KTXGLCompressedRGRGTC2 = 0x8DBD
)

// KTXHeader represents a KTX v1 header.
type KTXHeader struct {
	Identifier            [12]byte
	Endianness            uint32
	GlType                uint32
	GlTypeSize            uint32
	GlFormat              uint32
	GlInternalFormat      uint32
	GlBaseInternalFormat  uint32
	PixelWidth            uint32
	PixelHeight           uint32
	PixelDepth            uint32
	NumberOfArrayElements uint32
	NumberOfFaces         uint32
	NumberOfMipmapLevels  uint32
	BytesOfKeyValueData   uint32
}
