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
	Faces  []Face
	Format Format
	Width  int
	Height int
}

// IsCubemap reports whether the KTX contains six faces.
func (k *KTX) IsCubemap() bool {
	return len(k.Faces) == 6
}

// ReadKTX parses a KTX v1 stream with BCn payload.
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

	mipCount := int(header.NumberOfMipmapLevels)
	if mipCount < 1 {
		mipCount = 1
	}
	faceCount := int(header.NumberOfFaces)
	if faceCount < 1 {
		faceCount = 1
	}

	faces := make([]Face, faceCount)
	for face := 0; face < faceCount; face++ {
		faces[face].Mipmaps = make([][]byte, mipCount)
	}

	for mip := 0; mip < mipCount; mip++ {
		var imageSize uint32
		if err := binary.Read(r, binary.LittleEndian, &imageSize); err != nil {
			return nil, err
		}

		for face := 0; face < faceCount; face++ {
			buf := make([]byte, imageSize)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}

			faces[face].Mipmaps[mip] = buf
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

	return &KTX{Format: format, Width: int(header.PixelWidth), Height: int(header.PixelHeight), Faces: faces}, nil
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
		return ErrUnsupportedFormat
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

	internal, base, err := ktxInternalFormat(k.Format)
	if err != nil {
		return err
	}

	header := KTXHeader{
		Identifier:            KTXIdentifier,
		Endianness:            ktxEndianness,
		GlType:                0,
		GlTypeSize:            1,
		GlFormat:              0,
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

	for mip := 0; mip < mipCount; mip++ {
		imageSize := u32len(len(k.Faces[0].Mipmaps[mip]))
		if err := binary.Write(w, binary.LittleEndian, imageSize); err != nil {
			return err
		}

		for face := 0; face < len(k.Faces); face++ {
			if u32len(len(k.Faces[face].Mipmaps[mip])) != imageSize {
				return ErrMipmapSizeMismatch
			}

			if _, err := w.Write(k.Faces[face].Mipmaps[mip]); err != nil {
				return err
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
	if len(images) != 1 && len(images) != 6 {
		return nil, ErrExpectedOneOrSixImages
	}

	options := normalizeEncodeOptions(opts)
	width := images[0].Bounds().Dx()
	height := images[0].Bounds().Dy()
	for i := 1; i < len(images); i++ {
		// #nosec G602 -- bounds are checked by loop condition.
		if images[i].Bounds().Dx() != width || images[i].Bounds().Dy() != height {
			return nil, ErrFacesDifferentDimensions
		}
	}

	faces := make([]Face, len(images))
	for i, img := range images {
		if options.GenerateMipmaps {
			mips := GenerateMipmaps(img, options.UseSRGB)
			mipData := make([][]byte, len(mips))
			for level := range mips {
				data, _, _, err := EncodeImageWithOptions(mips[level], format, &options)
				if err != nil {
					return nil, err
				}

				mipData[level] = data
			}
			faces[i] = Face{Mipmaps: mipData}
		} else {
			data, _, _, err := EncodeImageWithOptions(img, format, &options)
			if err != nil {
				return nil, err
			}

			faces[i] = Face{Mipmaps: [][]byte{data}}
		}
	}

	return &KTX{Format: format, Width: width, Height: height, Faces: faces}, nil
}

func ktxInternalFormat(format Format) (uint32, uint32, error) {
	switch format {
	case FormatDXT1:
		return KTXGLCompressedRGBAS3TCDXT1, KTXGLRGBA, nil
	case FormatDXT3:
		return KTXGLCompressedRGBAS3TCDXT3, KTXGLRGBA, nil
	case FormatDXT5:
		return KTXGLCompressedRGBAS3TCDXT5, KTXGLRGBA, nil
	case FormatBC4:
		return KTXGLCompressedRedRGTC1, KTXGLRed, nil
	case FormatBC5:
		return KTXGLCompressedRGRGTC2, KTXGLRG, nil
	default:
		return 0, 0, ErrUnsupportedKTXFormat
	}
}

func ktxFormatFromHeader(header *KTXHeader) (Format, error) {
	if header.GlType != 0 || header.GlFormat != 0 {
		return FormatUnknown, ErrUnsupportedKTXCompressed
	}
	switch header.GlInternalFormat {
	case KTXGLCompressedRGBS3TCDXT1, KTXGLCompressedRGBAS3TCDXT1:
		return FormatDXT1, nil
	case KTXGLCompressedRGBAS3TCDXT3:
		return FormatDXT3, nil
	case KTXGLCompressedRGBAS3TCDXT5:
		return FormatDXT5, nil
	case KTXGLCompressedRedRGTC1:
		return FormatBC4, nil
	case KTXGLCompressedRGRGTC2:
		return FormatBC5, nil
	default:
		return FormatUnknown, ErrUnsupportedKTXInternalFormat
	}
}

// padding4 returns the number of bytes needed to align to 4 bytes.
func padding4(size uint32) uint32 {
	return (4 - (size % 4)) % 4
}

func writePadding(w io.Writer, pad uint32) error {
	if pad == 0 {
		return nil
	}

	var zeros [4]byte
	_, err := w.Write(zeros[:pad])

	return err
}
