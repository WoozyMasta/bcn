// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"image"
	"image/color"
)

// EncodeImage encodes an image.Image into BCn blocks using default options.
//
// The input is sampled as NRGBA (8-bit per channel).
func EncodeImage(img image.Image, format Format) ([]byte, int, int, error) {
	return EncodeImageWithOptions(img, format, nil)
}

// EncodeImageWithOptions encodes an image.Image into BCn blocks with options.
//
// This is the main entry point for quality level and mipmap behavior.
func EncodeImageWithOptions(img image.Image, format Format, opts *EncodeOptions) ([]byte, int, int, error) {
	nrgba := toNRGBA(img)
	b := nrgba.Bounds()
	width := b.Dx()
	height := b.Dy()

	// encodeBlocksWithOptions only reads rgba and requires a tight width*height*4 buffer.
	// When nrgba is already tight and origin-anchored
	// (the common case: toNRGBA returns a fresh tight NRGBA, or the caller passed one),
	// read Pix in place instead of allocating and copying a full-image buffer.
	var rgba []byte
	if nrgba.Rect.Min.X == 0 && nrgba.Rect.Min.Y == 0 && nrgba.Stride == width*4 && len(nrgba.Pix) >= width*height*4 {
		rgba = nrgba.Pix[: width*height*4 : width*height*4]
	} else {
		rgba = make([]byte, width*height*4)
		for y := range height {
			src := nrgba.Pix[nrgba.PixOffset(b.Min.X, b.Min.Y+y):]
			copy(rgba[y*width*4:(y+1)*width*4], src[:width*4])
		}
	}

	data, err := encodeBlocksWithOptions(rgba, width, height, format, opts)

	return data, width, height, err
}

// DecodeImage decodes BCn blocks into a new image.NRGBA.
func DecodeImage(data []byte, width, height int, format Format) (*image.NRGBA, error) {
	return DecodeImageWithOptions(data, width, height, format, nil)
}

// DecodeImageWithOptions decodes BCn blocks into a new image.NRGBA with options.
func DecodeImageWithOptions(data []byte, width, height int, format Format, opts *DecodeOptions) (*image.NRGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	// Decode straight into the destination Pix to skip a second width*height*4 allocation and copy.
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if err := decodeBlocksInto(img.Pix, data, width, height, format, opts); err != nil {
		return nil, err
	}

	return img, nil
}

// AsNRGBA converts a slice of RGBA bytes into an image.NRGBA without copying.
// The caller must ensure the slice length is width*height*4.
func AsNRGBA(rgba []byte, width, height int) *image.NRGBA {
	return &image.NRGBA{
		Pix:    rgba,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
}

// SolidImage returns a solid-color NRGBA image for tests and examples.
func SolidImage(width, height int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			i := y*img.Stride + x*4
			img.Pix[i+0] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}

	return img
}
