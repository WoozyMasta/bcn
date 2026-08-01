// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "errors"

// Package-level errors for consumers to match with errors.Is.
var (
	// ErrInvalidDimensions indicates a width/height <= 0.
	ErrInvalidDimensions = errors.New("invalid dimensions")
	// ErrInvalidRGBALength indicates RGBA slice length mismatch.
	ErrInvalidRGBALength = errors.New("invalid rgba length")
	// ErrBufferTooSmall indicates a caller-provided destination buffer is too small.
	ErrBufferTooSmall = errors.New("destination buffer too small")
	// ErrInsufficientData indicates compressed/uncompressed data is too short.
	ErrInsufficientData = errors.New("insufficient data")
	// ErrUnsupportedFormat indicates an unsupported pixel format.
	ErrUnsupportedFormat = errors.New("unsupported format")
	// ErrUnsupportedUncompressedFormat indicates an unsupported uncompressed format.
	ErrUnsupportedUncompressedFormat = errors.New("unsupported uncompressed format")
	// ErrBC6HUsesHDRAPI is returned when a BC6H format is passed to the NRGBA byte API.
	// Use DecodeBC6H / EncodeBC6H instead.
	ErrBC6HUsesHDRAPI = errors.New("BC6H requires the HDR API (DecodeBC6H / EncodeBC6H)")
	// ErrInvalidHDRSliceLength indicates the HDR pixel slice length does not match width*height*3.
	ErrInvalidHDRSliceLength = errors.New("invalid HDR slice length: must be width*height*3")

	// ErrNilDDS indicates a nil DDS container.
	ErrNilDDS = errors.New("nil DDS")
	// ErrNilDDSHeader indicates a nil DDS header.
	ErrNilDDSHeader = errors.New("nil DDS header")
	// ErrInvalidDDSMagic indicates an invalid DDS magic.
	ErrInvalidDDSMagic = errors.New("invalid DDS magic")
	// ErrInvalidDDSHeaderSize indicates a DDS header size mismatch.
	ErrInvalidDDSHeaderSize = errors.New("invalid DDS header size")
	// ErrInvalidDDSPixelFormatSize indicates a DDS pixel format size mismatch.
	ErrInvalidDDSPixelFormatSize = errors.New("invalid DDS pixel format size")
	// ErrUnsupportedDDSPixelFormat indicates an unsupported DDS pixel format.
	ErrUnsupportedDDSPixelFormat = errors.New("unsupported DDS pixel format")
	// ErrUnsupportedDDSFourCC indicates an unsupported DDS FourCC.
	ErrUnsupportedDDSFourCC = errors.New("unsupported DDS FourCC")
	// ErrUnsupportedDDSFormat indicates an unsupported DDS format.
	ErrUnsupportedDDSFormat = errors.New("unsupported DDS format")
	// ErrUnsupportedDX10Format indicates an unsupported DX10 format.
	ErrUnsupportedDX10Format = errors.New("unsupported DX10 format")
	// ErrUnsupportedDDSResourceDimension indicates an unsupported DDS resource dimension.
	ErrUnsupportedDDSResourceDimension = errors.New("unsupported DDS resource dimension")
	// ErrDDSArrayNotSupported indicates DDS array textures are not supported.
	ErrDDSArrayNotSupported = errors.New("DDS array textures not supported")
	// ErrInvalidFaceCount indicates invalid face count (must be 1 or 6).
	ErrInvalidFaceCount = errors.New("invalid face count")
	// ErrNoFaces indicates no faces present.
	ErrNoFaces = errors.New("no faces")
	// ErrNoMipmaps indicates no mipmaps present.
	ErrNoMipmaps = errors.New("no mipmaps")
	// ErrEmptyMipmaps indicates a face has no mipmaps.
	ErrEmptyMipmaps = errors.New("empty mipmaps")
	// ErrExpectedOneOrSixImages indicates the encoder expected 1 or 6 images.
	ErrExpectedOneOrSixImages = errors.New("expected 1 or 6 images")
	// ErrFacesDifferentDimensions indicates cubemap faces mismatch dimensions.
	ErrFacesDifferentDimensions = errors.New("all faces must have same dimensions")
	// ErrMipmapCountMismatch indicates inconsistent mipmap count across faces.
	ErrMipmapCountMismatch = errors.New("mipmap count mismatch")
	// ErrMipmapSizeMismatch indicates inconsistent mipmap size across faces.
	ErrMipmapSizeMismatch = errors.New("mipmap size mismatch")

	// ErrNilKTX indicates a nil KTX container.
	ErrNilKTX = errors.New("nil KTX")
	// ErrNilKTXHeader indicates a nil KTX header.
	ErrNilKTXHeader = errors.New("nil KTX header")
	// ErrInvalidKTXIdentifier indicates an invalid KTX identifier.
	ErrInvalidKTXIdentifier = errors.New("invalid KTX identifier")
	// ErrUnsupportedKTXEndianness indicates unsupported KTX endianness.
	ErrUnsupportedKTXEndianness = errors.New("unsupported KTX endianness")
	// ErrKTXArraysNotSupported indicates KTX arrays are not supported.
	ErrKTXArraysNotSupported = errors.New("KTX arrays not supported")
	// ErrKTX3DNotSupported indicates KTX 3D textures are not supported.
	ErrKTX3DNotSupported = errors.New("KTX 3D textures not supported")
	// ErrUnsupportedKTXFormat indicates an unsupported KTX format.
	ErrUnsupportedKTXFormat = errors.New("unsupported KTX format")
	// ErrUnsupportedKTXInternalFormat indicates an unsupported KTX internal format.
	ErrUnsupportedKTXInternalFormat = errors.New("unsupported KTX internal format")
	// ErrUnsupportedKTXUncompressed indicates an unsupported uncompressed KTX format (only RGBA8/BGRA8 are supported).
	ErrUnsupportedKTXUncompressed = errors.New("unsupported KTX uncompressed format")
)
