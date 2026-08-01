// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"encoding/binary"
	"image"
	"io"
)

const (
	// ktxEndianness is the canonical little-endian marker.
	ktxEndianness = KTXEndianness
)

// KTX represents a KTX v1 texture with BCn payload.
//
// Faces is 1 for 2D textures or 6 for cubemaps.
type KTX struct {
	Faces  []Face // Faces of the texture.
	Format Format // Format of the texture.
	Width  int    // Width of the texture.
	Height int    // Height of the texture.
}

// IsCubemap reports whether the KTX contains six faces.
func (k *KTX) IsCubemap() bool {
	return len(k.Faces) == 6
}

// ReadKTX parses a KTX v1 stream with BCn or supported uncompressed payload.
// Arrays and 3D textures are rejected.
func ReadKTX(r io.Reader) (*KTX, error) {
	var header KTXHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Identifier != KTXIdentifier {
		return nil, ErrInvalidKTXIdentifier
	}

	if header.Endianness != ktxEndianness {
		return nil, ErrUnsupportedKTXEndianness
	}

	if header.NumberOfArrayElements > 0 {
		return nil, ErrKTXArraysNotSupported
	}

	if header.PixelDepth != 0 && header.PixelDepth != 1 {
		return nil, ErrKTX3DNotSupported
	}

	format, err := ktxFormatFromHeader(&header)
	if err != nil {
		return nil, err
	}

	if header.BytesOfKeyValueData > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(header.BytesOfKeyValueData)); err != nil {
			return nil, err
		}
	}

	mipCount := max(int(header.NumberOfMipmapLevels), 1)
	faceCount := max(int(header.NumberOfFaces), 1)
	width := int(header.PixelWidth)
	height := int(header.PixelHeight)
	uncompressed := !format.isCompressed()

	faces := make([]Face, faceCount)
	for face := range faceCount {
		faces[face].Mipmaps = make([][]byte, mipCount)
	}

	for mip := range mipCount {
		var imageSize uint32
		if err := binary.Read(r, binary.LittleEndian, &imageSize); err != nil {
			return nil, err
		}

		mipW := width
		if mip > 0 {
			for i := 0; i < mip && mipW > 1; i++ {
				mipW >>= 1
			}
		}
		mipH := height
		if mip > 0 {
			for i := 0; i < mip && mipH > 1; i++ {
				mipH >>= 1
			}
		}

		if uncompressed && imageSize == 0 {
			imageSize = ktxUncompressedMipSize(mipW, mipH, format.blockSize())
		}

		for face := range faceCount {
			buf := make([]byte, imageSize)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}

			if uncompressed {
				faces[face].Mipmaps[mip] = ktxUncompressedToTight(buf, mipW, mipH, format.blockSize())
			} else {
				faces[face].Mipmaps[mip] = buf
			}

			if faceCount == 6 {
				pad := padding4(imageSize)
				if _, err := io.CopyN(io.Discard, r, int64(pad)); err != nil {
					return nil, err
				}
			}
		}

		pad := padding4(imageSize)
		if _, err := io.CopyN(io.Discard, r, int64(pad)); err != nil {
			return nil, err
		}
	}

	return &KTX{Format: format, Width: width, Height: height, Faces: faces}, nil
}

// DecodeKTX decodes the first face/mip level of a KTX into an image.
// This is a convenience wrapper around ReadKTX + DecodeImageWithOptions with nil options.
func DecodeKTX(r io.Reader) (*KTX, *image.NRGBA, error) {
	return DecodeKTXWithOptions(r, nil)
}

// DecodeKTXWithOptions decodes the first face/mip level of a KTX into an image with options.
// This is a convenience wrapper around ReadKTX + DecodeImageWithOptions.
func DecodeKTXWithOptions(r io.Reader, opts *DecodeOptions) (*KTX, *image.NRGBA, error) {
	k, err := ReadKTX(r)
	if err != nil {
		return nil, nil, err
	}

	if len(k.Faces) == 0 || len(k.Faces[0].Mipmaps) == 0 {
		return k, nil, ErrNoMipmaps
	}

	img, err := DecodeImageWithOptions(k.Faces[0].Mipmaps[0], k.Width, k.Height, k.Format, opts)
	if err != nil {
		return k, nil, err
	}

	return k, img, nil
}

// Write serializes the KTX to a stream.
// The caller must populate Faces and Mipmaps consistently.
func (k *KTX) Write(w io.Writer) error {
	if k == nil {
		return ErrNilKTX
	}

	if k.Width <= 0 || k.Height <= 0 {
		return ErrInvalidDimensions
	}

	if !k.Format.isCompressed() {
		switch k.Format {
		case FormatRGBA8, FormatBGRA8, FormatR8, FormatRG8:
		default:
			return ErrUnsupportedFormat
		}
	}

	if len(k.Faces) == 0 {
		return ErrNoFaces
	}

	if len(k.Faces) != 1 && len(k.Faces) != 6 {
		return ErrInvalidFaceCount
	}

	mipCount := len(k.Faces[0].Mipmaps)
	if mipCount == 0 {
		return ErrNoMipmaps
	}

	for i := range k.Faces {
		if len(k.Faces[i].Mipmaps) != mipCount {
			return ErrMipmapCountMismatch
		}
	}

	glType, glTypeSize, glFormat, internal, base, err := ktxHeaderFormats(k.Format)
	if err != nil {
		return err
	}

	header := KTXHeader{
		Identifier:            KTXIdentifier,
		Endianness:            ktxEndianness,
		GlType:                glType,
		GlTypeSize:            glTypeSize,
		GlFormat:              glFormat,
		GlInternalFormat:      internal,
		GlBaseInternalFormat:  base,
		PixelWidth:            u32(k.Width),
		PixelHeight:           u32(k.Height),
		PixelDepth:            0,
		NumberOfArrayElements: 0,
		NumberOfFaces:         u32len(len(k.Faces)),
		NumberOfMipmapLevels:  u32len(mipCount),
		BytesOfKeyValueData:   0,
	}

	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return err
	}

	for mip := range mipCount {
		mipData := k.Faces[0].Mipmaps[mip]
		imageSize := u32len(len(mipData))
		if k.Format.isCompressed() {
			if err := binary.Write(w, binary.LittleEndian, imageSize); err != nil {
				return err
			}
		} else {
			mipW := k.Width
			for i := 0; i < mip && mipW > 1; i++ {
				mipW >>= 1
			}
			mipH := k.Height
			for i := 0; i < mip && mipH > 1; i++ {
				mipH >>= 1
			}

			imageSize = ktxUncompressedMipSize(mipW, mipH, k.Format.blockSize())
			if err := binary.Write(w, binary.LittleEndian, imageSize); err != nil {
				return err
			}
		}

		// Encode each face/mipmap
		for face := 0; face < len(k.Faces); face++ {
			faceMip := k.Faces[face].Mipmaps[mip]
			if k.Format.isCompressed() {
				if u32len(len(faceMip)) != imageSize {
					return ErrMipmapSizeMismatch
				}
				if _, err := w.Write(faceMip); err != nil {
					return err
				}
			} else {
				mipW := k.Width
				for i := 0; i < mip && mipW > 1; i++ {
					mipW >>= 1
				}
				mipH := k.Height
				for i := 0; i < mip && mipH > 1; i++ {
					mipH >>= 1
				}

				if err := ktxWriteUncompressedMip(w, faceMip, mipW, mipH, k.Format.blockSize()); err != nil {
					return err
				}
			}

			if len(k.Faces) == 6 {
				pad := padding4(imageSize)
				if err := writePadding(w, pad); err != nil {
					return err
				}
			}
		}

		pad := padding4(imageSize)
		if err := writePadding(w, pad); err != nil {
			return err
		}
	}

	return nil
}

// EncodeKTX encodes an image into a KTX with a single mip level.
func EncodeKTX(img image.Image, format Format) (*KTX, error) {
	return EncodeKTXWithOptions([]image.Image{img}, format, nil)
}

// EncodeKTXWithOptions encodes 1 image (2D) or 6 images (cubemap) into a KTX.
// Mipmaps are generated when EncodeOptions.GenerateMipmaps is true.
func EncodeKTXWithOptions(images []image.Image, format Format, opts *EncodeOptions) (*KTX, error) {
	faces, width, height, err := encodeFacesWithOptions(images, format, opts)
	if err != nil {
		return nil, err
	}

	return &KTX{Format: format, Width: width, Height: height, Faces: faces}, nil
}

// ktxHeaderFormats returns GlType, GlTypeSize, GlFormat, GlInternalFormat, GlBaseInternalFormat for the KTX header.
func ktxHeaderFormats(format Format) (glType, glTypeSize, glFormat, glInternalFormat, glBaseInternalFormat uint32, err error) {
	switch format {
	case FormatBC1:
		return 0, 1, 0, KTXGLCompressedRGBAS3TCBC1, KTXGLRGBA, nil
	case FormatBC2:
		return 0, 1, 0, KTXGLCompressedRGBAS3TCBC2, KTXGLRGBA, nil
	case FormatBC3:
		return 0, 1, 0, KTXGLCompressedRGBAS3TCBC3, KTXGLRGBA, nil
	case FormatBC4:
		return 0, 1, 0, KTXGLCompressedRedRGTC1, KTXGLRed, nil
	case FormatBC4S:
		return 0, 1, 0, KTXGLCompressedSignedRedRGTC1, KTXGLRed, nil
	case FormatBC5:
		return 0, 1, 0, KTXGLCompressedRGRGTC2, KTXGLRG, nil
	case FormatBC5S:
		return 0, 1, 0, KTXGLCompressedSignedRGRGTC2, KTXGLRG, nil
	case FormatBC7:
		return 0, 1, 0, KTXGLCompressedRGBABPTCUnorm, KTXGLRGBA, nil
	case FormatBC6HU:
		return 0, 1, 0, KTXGLCompressedRGBBPTCUnsignedFloat, KTXGLRGB, nil
	case FormatBC6HS:
		return 0, 1, 0, KTXGLCompressedRGBBPTCSignedFloat, KTXGLRGB, nil
	case FormatRGBA8:
		return KTXGLUnsignedByte, 1, KTXGLRGBA, KTXGLRGBA8, KTXGLRGBA, nil
	case FormatBGRA8:
		return KTXGLUnsignedByte, 1, KTXGLBGRA, KTXGLRGBA8, KTXGLRGBA, nil
	case FormatR8:
		return KTXGLUnsignedByte, 1, KTXGLRed, KTXGLR8, KTXGLRed, nil
	case FormatRG8:
		return KTXGLUnsignedByte, 1, KTXGLRG, KTXGLRG8, KTXGLRG, nil
	default:
		return 0, 0, 0, 0, 0, ErrUnsupportedKTXFormat
	}
}

// ktxFormatFromHeader maps KTX header GL fields to internal BCn/uncompressed format.
func ktxFormatFromHeader(header *KTXHeader) (Format, error) {
	if header.GlType != 0 || header.GlFormat != 0 {
		if header.GlType == KTXGLUnsignedByte && header.GlTypeSize == 1 {
			switch header.GlFormat {
			case KTXGLRGBA:
				return FormatRGBA8, nil
			case KTXGLBGRA:
				return FormatBGRA8, nil
			case KTXGLRed:
				return FormatR8, nil
			case KTXGLRG:
				return FormatRG8, nil
			default:
				return FormatUnknown, ErrUnsupportedKTXUncompressed
			}
		}

		return FormatUnknown, ErrUnsupportedKTXUncompressed
	}

	switch header.GlInternalFormat {
	case KTXGLCompressedRGBS3TCBC1, KTXGLCompressedRGBAS3TCBC1:
		return FormatBC1, nil
	case KTXGLCompressedRGBAS3TCBC2:
		return FormatBC2, nil
	case KTXGLCompressedRGBAS3TCBC3:
		return FormatBC3, nil
	case KTXGLCompressedRedRGTC1:
		return FormatBC4, nil
	case KTXGLCompressedSignedRedRGTC1:
		return FormatBC4S, nil
	case KTXGLCompressedRGRGTC2:
		return FormatBC5, nil
	case KTXGLCompressedSignedRGRGTC2:
		return FormatBC5S, nil
	case KTXGLCompressedRGBABPTCUnorm, KTXGLCompressedSRGBAlphaBPTCUnorm:
		return FormatBC7, nil
	case KTXGLCompressedRGBBPTCUnsignedFloat:
		return FormatBC6HU, nil
	case KTXGLCompressedRGBBPTCSignedFloat:
		return FormatBC6HS, nil
	default:
		return FormatUnknown, ErrUnsupportedKTXInternalFormat
	}
}

// ktxUncompressedMipSize returns an uncompressed KTX mip size including row padding.
func ktxUncompressedMipSize(width, height, bytesPerPixel int) uint32 {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	rowStride := (width*bytesPerPixel + 3) & ^3
	return u32len(rowStride * height)
}

// ktxUncompressedToTight converts bottom-up, row-padded KTX pixels to tight top-down data.
func ktxUncompressedToTight(buf []byte, width, height, bytesPerPixel int) []byte {
	rowStride := (width*bytesPerPixel + 3) & ^3
	tight := make([]byte, width*height*bytesPerPixel)

	for y := height - 1; y >= 0; y-- {
		src := buf[(height-1-y)*rowStride:]
		dst := tight[y*width*bytesPerPixel:]
		copy(dst[:width*bytesPerPixel], src[:width*bytesPerPixel])
	}

	return tight
}

// ktxWriteUncompressedMip writes tight pixels as bottom-up, row-padded KTX data.
func ktxWriteUncompressedMip(w io.Writer, tight []byte, width, height, bytesPerPixel int) error {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	rowStride := (width*bytesPerPixel + 3) & ^3
	for y := height - 1; y >= 0; y-- {
		row := tight[y*width*bytesPerPixel:]
		if _, err := w.Write(row[:width*bytesPerPixel]); err != nil {
			return err
		}

		if pad := rowStride - width*bytesPerPixel; pad > 0 {
			var zeros [4]byte
			if _, err := w.Write(zeros[:pad]); err != nil {
				return err
			}
		}
	}

	return nil
}

// padding4 returns the number of bytes needed to align to 4 bytes.
func padding4(size uint32) uint32 {
	return (4 - (size % 4)) % 4
}

// writePadding emits up to 3 zero bytes to maintain 4-byte KTX alignment.
func writePadding(w io.Writer, pad uint32) error {
	if pad == 0 {
		return nil
	}

	var zeros [4]byte
	_, err := w.Write(zeros[:pad])

	return err
}
