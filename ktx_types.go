// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// KTX constants and header structures.

// KTXIdentifier is the 12-byte KTX v1 file signature.
var KTXIdentifier = [12]byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x31, 0x31, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

const (
	// KTXEndianness is the canonical little-endian marker.
	KTXEndianness = 0x04030201

	// KTXGLUnsignedByte is GL_UNSIGNED_BYTE.
	KTXGLUnsignedByte = 0x1401
	// KTXGLByte is GL_BYTE.
	KTXGLByte = 0x1400
	// KTXGLUnsignedInt2101010Rev is GL_UNSIGNED_INT_2_10_10_10_REV.
	KTXGLUnsignedInt2101010Rev = 0x8368
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
	// KTXGLRGB10A2 is GL_RGB10_A2 (sized internal format).
	KTXGLRGB10A2 = 0x8059
	// KTXGLR8 is GL_R8 (sized internal format).
	KTXGLR8 = 0x8229
	// KTXGLRG8 is GL_RG8 (sized internal format).
	KTXGLRG8 = 0x822B
	// KTXGLR8SNORM is GL_R8_SNORM (sized internal format).
	KTXGLR8SNORM = 0x8F94
	// KTXGLRG8SNORM is GL_RG8_SNORM (sized internal format).
	KTXGLRG8SNORM = 0x8F95

	// KTXGLCompressedRGBS3TCBC1 is GL_COMPRESSED_RGB_S3TC_DXT1_EXT.
	KTXGLCompressedRGBS3TCBC1 = 0x83F0
	// KTXGLCompressedRGBAS3TCBC1 is GL_COMPRESSED_RGBA_S3TC_DXT1_EXT.
	KTXGLCompressedRGBAS3TCBC1 = 0x83F1
	// KTXGLCompressedRGBAS3TCBC2 is GL_COMPRESSED_RGBA_S3TC_DXT3_EXT.
	KTXGLCompressedRGBAS3TCBC2 = 0x83F2
	// KTXGLCompressedRGBAS3TCBC3 is GL_COMPRESSED_RGBA_S3TC_DXT5_EXT.
	KTXGLCompressedRGBAS3TCBC3 = 0x83F3
	// KTXGLCompressedRedRGTC1 is GL_COMPRESSED_RED_RGTC1.
	KTXGLCompressedRedRGTC1 = 0x8DBB
	// KTXGLCompressedSignedRedRGTC1 is GL_COMPRESSED_SIGNED_RED_RGTC1.
	KTXGLCompressedSignedRedRGTC1 = 0x8DBC
	// KTXGLCompressedRGRGTC2 is GL_COMPRESSED_RG_RGTC2.
	KTXGLCompressedRGRGTC2 = 0x8DBD
	// KTXGLCompressedSignedRGRGTC2 is GL_COMPRESSED_SIGNED_RG_RGTC2.
	KTXGLCompressedSignedRGRGTC2 = 0x8DBE
	// KTXGLCompressedRGBABPTCUnorm is GL_COMPRESSED_RGBA_BPTC_UNORM (BC7).
	KTXGLCompressedRGBABPTCUnorm = 0x8E8C
	// KTXGLCompressedSRGBAlphaBPTCUnorm is GL_COMPRESSED_SRGB_ALPHA_BPTC_UNORM (BC7 sRGB).
	KTXGLCompressedSRGBAlphaBPTCUnorm = 0x8E8D
	// KTXGLCompressedRGBBPTCUnsignedFloat is GL_COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT (BC6H UF16).
	KTXGLCompressedRGBBPTCUnsignedFloat = 0x8E8E
	// KTXGLCompressedRGBBPTCSignedFloat is GL_COMPRESSED_RGB_BPTC_SIGNED_FLOAT (BC6H SF16).
	KTXGLCompressedRGBBPTCSignedFloat = 0x8E8F
)

const (
	// KTXGLCompressedRGBS3TCDXT1 is a compatibility alias for KTXGLCompressedRGBS3TCBC1.
	KTXGLCompressedRGBS3TCDXT1 = KTXGLCompressedRGBS3TCBC1
	// KTXGLCompressedRGBAS3TCDXT1 is a compatibility alias for KTXGLCompressedRGBAS3TCBC1.
	KTXGLCompressedRGBAS3TCDXT1 = KTXGLCompressedRGBAS3TCBC1
	// KTXGLCompressedRGBAS3TCDXT3 is a compatibility alias for KTXGLCompressedRGBAS3TCBC2.
	KTXGLCompressedRGBAS3TCDXT3 = KTXGLCompressedRGBAS3TCBC2
	// KTXGLCompressedRGBAS3TCDXT5 is a compatibility alias for KTXGLCompressedRGBAS3TCBC3.
	KTXGLCompressedRGBAS3TCDXT5 = KTXGLCompressedRGBAS3TCBC3
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
