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
	// ddsMagic is the file signature "DDS ".
	ddsMagic = DDSMagic

	// Cubemap caps (caps2). Only DDSCaps2Cubemap is public.
	ddsCaps2CubemapPosX = 0x400
	ddsCaps2CubemapNegX = 0x800
	ddsCaps2CubemapPosY = 0x1000
	ddsCaps2CubemapNegY = 0x2000
	ddsCaps2CubemapPosZ = 0x4000
	ddsCaps2CubemapNegZ = 0x8000
)

// Face contains all mip levels for a single face.
type Face struct {
	Mipmaps [][]byte // Mipmaps of the face.
}

// DDS represents a DDS texture with BCn payload.
//
// Faces is 1 for 2D textures or 6 for cubemaps.
type DDS struct {
	Faces  []Face // Faces of the texture.
	Format Format // Format of the texture.
	Width  int    // Width of the texture.
	Height int    // Height of the texture.
}

// IsCubemap reports whether the DDS contains six faces.
func (d *DDS) IsCubemap() bool {
	return len(d.Faces) == 6
}

// ReadDDS parses a DDS stream with BCn payload.
// Cubemaps and mipmaps are supported; arrays are not.
func ReadDDS(r io.Reader) (*DDS, error) {
	var magic uint32
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
		return nil, err
	}

	if magic != ddsMagic {
		return nil, ErrInvalidDDSMagic
	}

	var header DDSHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Size != DDSHeaderSize {
		return nil, ErrInvalidDDSHeaderSize
	}

	format, dx10, err := ddsFormatFromHeader(r, &header)
	if err != nil {
		return nil, err
	}

	if dx10 != nil {
		if dx10.ResourceDimension != 3 {
			return nil, ErrUnsupportedDDSResourceDimension
		}

		if dx10.ArraySize != 1 {
			return nil, ErrDDSArrayNotSupported
		}
	}

	mipCount := max(int(header.MipMapCount), 1)
	width := int(header.Width)
	height := int(header.Height)
	faceCount := 1
	if header.Caps2&DDSCaps2Cubemap != 0 {
		faceCount = 6
	}

	faces := make([]Face, faceCount)
	for face := 0; face < faceCount; face++ {
		w := width
		h := height
		mips := make([][]byte, mipCount)

		for i := range mipCount {
			size := mipSize(format, w, h)
			buf := make([]byte, size)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}

			mips[i] = buf
			if w > 1 {
				w >>= 1
			}
			if h > 1 {
				h >>= 1
			}
		}
		faces[face] = Face{Mipmaps: mips}
	}

	return &DDS{Format: format, Width: width, Height: height, Faces: faces}, nil
}

// Write serializes the DDS to a stream.
// The caller must populate Faces and Mipmaps consistently.
func (d *DDS) Write(w io.Writer) error {
	if d == nil {
		return ErrNilDDS
	}

	if d.Width <= 0 || d.Height <= 0 {
		return ErrInvalidDimensions
	}

	if !d.Format.isCompressed() {
		switch d.Format {
		case FormatRGBA8, FormatBGRA8:
		default:
			return ErrUnsupportedFormat
		}
	}

	if len(d.Faces) == 0 {
		return ErrNoFaces
	}

	if len(d.Faces) != 1 && len(d.Faces) != 6 {
		return ErrInvalidFaceCount
	}

	for i := range d.Faces {
		if len(d.Faces[i].Mipmaps) == 0 {
			return ErrEmptyMipmaps
		}
	}

	hdr := DDSHeader{}
	hdr.Size = DDSHeaderSize
	hdr.Flags = DDSFlagCaps | DDSFlagHeight | DDSFlagWidth | DDSFlagPixelFormat
	hdr.Width = u32(d.Width)
	hdr.Height = u32(d.Height)
	hdr.Depth = 1
	hdr.MipMapCount = u32len(len(d.Faces[0].Mipmaps))
	hdr.Caps = DDSCapsTexture

	if len(d.Faces[0].Mipmaps) > 1 {
		hdr.Flags |= DDSFlagMipmapCount
		hdr.Caps |= DDSCapsComplex | DDSCapsMipmap
	}

	if len(d.Faces) == 6 {
		hdr.Caps |= DDSCapsComplex
		hdr.Caps2 |= DDSCaps2Cubemap | ddsCaps2CubemapPosX | ddsCaps2CubemapNegX |
			ddsCaps2CubemapPosY | ddsCaps2CubemapNegY | ddsCaps2CubemapPosZ | ddsCaps2CubemapNegZ
	}

	hdr.PixelFormat.Size = DDSPixelFormatSize
	switch d.Format {
	case FormatDXT1:
		hdr.Flags |= DDSFlagLinearSize
		hdr.PixelFormat.Flags = DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '1')
	case FormatDXT3:
		hdr.Flags |= DDSFlagLinearSize
		hdr.PixelFormat.Flags = DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '3')
	case FormatDXT5:
		hdr.Flags |= DDSFlagLinearSize
		hdr.PixelFormat.Flags = DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '5')
	case FormatBC4:
		hdr.Flags |= DDSFlagLinearSize
		hdr.PixelFormat.Flags = DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('A', 'T', 'I', '1')
	case FormatBC5:
		hdr.Flags |= DDSFlagLinearSize
		hdr.PixelFormat.Flags = DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('A', 'T', 'I', '2')
	case FormatRGBA8:
		hdr.Flags |= DDSFlagPitch
		hdr.PixelFormat.Flags = DDSPFRGB | DDSPFAlphaPixels
		hdr.PixelFormat.RGBBitCount = 32
		hdr.PixelFormat.RBitMask = 0x000000ff
		hdr.PixelFormat.GBitMask = 0x0000ff00
		hdr.PixelFormat.BBitMask = 0x00ff0000
		hdr.PixelFormat.ABitMask = 0xff000000
		hdr.PitchOrLinearSize = u32(d.Width * 4)
	case FormatBGRA8:
		hdr.Flags |= DDSFlagPitch
		hdr.PixelFormat.Flags = DDSPFRGB | DDSPFAlphaPixels
		hdr.PixelFormat.RGBBitCount = 32
		hdr.PixelFormat.RBitMask = 0x00ff0000
		hdr.PixelFormat.GBitMask = 0x0000ff00
		hdr.PixelFormat.BBitMask = 0x000000ff
		hdr.PixelFormat.ABitMask = 0xff000000
		hdr.PitchOrLinearSize = u32(d.Width * 4)
	default:
		return ErrUnsupportedDDSFormat
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(ddsMagic)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}

	for _, face := range d.Faces {
		for _, mip := range face.Mipmaps {
			if _, err := w.Write(mip); err != nil {
				return err
			}
		}
	}

	return nil
}

// ddsFormatFromHeader resolves BCn/uncompressed format from classic or DX10 DDS headers.
func ddsFormatFromHeader(r io.Reader, header *DDSHeader) (Format, *DDSHeaderDX10, error) {
	pf := header.PixelFormat
	if pf.Flags&DDSPFFourCC == 0 {
		if pf.Flags&DDSPFRGB != 0 && pf.RGBBitCount == 32 && (pf.Flags&DDSPFAlphaPixels) != 0 {
			if pf.RBitMask == 0x000000ff && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x00ff0000 && pf.ABitMask == 0xff000000 {
				return FormatRGBA8, nil, nil
			}
			if pf.RBitMask == 0x00ff0000 && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x000000ff && pf.ABitMask == 0xff000000 {
				return FormatBGRA8, nil, nil
			}
		}
		return FormatUnknown, nil, ErrUnsupportedDDSPixelFormat
	}

	if pf.FourCC == makeFourCC('D', 'X', '1', '0') {
		var dx10 DDSHeaderDX10
		if err := binary.Read(r, binary.LittleEndian, &dx10); err != nil {
			return FormatUnknown, nil, err
		}

		switch dx10.DXGIFormat {
		case 71:
			return FormatDXT1, &dx10, nil
		case 74:
			return FormatDXT3, &dx10, nil
		case 77:
			return FormatDXT5, &dx10, nil
		case 80:
			return FormatBC4, &dx10, nil
		case 83:
			return FormatBC5, &dx10, nil
		default:
			return FormatUnknown, &dx10, ErrUnsupportedDX10Format
		}
	}

	switch pf.FourCC {
	case makeFourCC('D', 'X', 'T', '1'):
		return FormatDXT1, nil, nil
	case makeFourCC('D', 'X', 'T', '3'):
		return FormatDXT3, nil, nil
	case makeFourCC('D', 'X', 'T', '5'):
		return FormatDXT5, nil, nil
	case makeFourCC('A', 'T', 'I', '1'):
		return FormatBC4, nil, nil
	case makeFourCC('A', 'T', 'I', '2'):
		return FormatBC5, nil, nil
	case makeFourCC('B', 'C', '4', 'U'):
		return FormatBC4, nil, nil
	case makeFourCC('B', 'C', '5', 'U'):
		return FormatBC5, nil, nil
	default:
		return FormatUnknown, nil, ErrUnsupportedDDSFourCC
	}
}

// makeFourCC packs four ASCII bytes into DDS little-endian FOURCC form.
func makeFourCC(a, b, c, d byte) uint32 {
	return uint32(a) | uint32(b)<<8 | uint32(c)<<16 | uint32(d)<<24
}

// mipSize computes byte size for one mip level for compressed or raw pixel formats.
func mipSize(format Format, width, height int) int {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	blockSize := format.blockSize()
	if format.isCompressed() {
		bx := (width + 3) / 4
		by := (height + 3) / 4
		return bx * by * blockSize
	}

	return width * height * blockSize
}

// DecodeDDS decodes the first face/mip level of a DDS into an image.
// This is a convenience wrapper around ReadDDS + DecodeImageWithOptions with nil options.
func DecodeDDS(r io.Reader) (*DDS, *image.NRGBA, error) {
	return DecodeDDSWithOptions(r, nil)
}

// DecodeDDSWithOptions decodes the first face/mip level of a DDS into an image with options.
// This is a convenience wrapper around ReadDDS + DecodeImageWithOptions.
func DecodeDDSWithOptions(r io.Reader, opts *DecodeOptions) (*DDS, *image.NRGBA, error) {
	d, err := ReadDDS(r)
	if err != nil {
		return nil, nil, err
	}

	if len(d.Faces) == 0 || len(d.Faces[0].Mipmaps) == 0 {
		return d, nil, ErrNoMipmaps
	}

	img, err := DecodeImageWithOptions(d.Faces[0].Mipmaps[0], d.Width, d.Height, d.Format, opts)
	if err != nil {
		return d, nil, err
	}

	return d, img, nil
}

// EncodeDDS encodes an image into a DDS with a single mip level.
func EncodeDDS(img image.Image, format Format) (*DDS, error) {
	return EncodeDDSWithOptions([]image.Image{img}, format, nil)
}

// EncodeDDSWithOptions encodes 1 image (2D) or 6 images (cubemap) into a DDS.
// Mipmaps are generated when EncodeOptions.GenerateMipmaps is true.
func EncodeDDSWithOptions(images []image.Image, format Format, opts *EncodeOptions) (*DDS, error) {
	faces, width, height, err := encodeFacesWithOptions(images, format, opts)
	if err != nil {
		return nil, err
	}

	return &DDS{Format: format, Width: width, Height: height, Faces: faces}, nil
}
