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
	rgba, width, height := imageToTightRGBA(img)
	data, err := encodeBlocksWithOptions(rgba, width, height, format, opts)

	return data, width, height, err
}

// EncodeImageInto encodes img into dst, a caller-owned buffer reused across calls,
// and returns the encoded slice plus the image dimensions.
// dst is reallocated only when its capacity is too small;
// pass the returned slice back on the next call to reuse it.
// The output is identical to EncodeImageWithOptions.
func EncodeImageInto(dst []byte, img image.Image, format Format, opts *EncodeOptions) ([]byte, int, int, error) {
	rgba, width, height := imageToTightRGBA(img)

	n, err := encodedBlocksSize(rgba, width, height, format)
	if err != nil {
		return nil, 0, 0, err
	}
	if cap(dst) < n {
		dst = make([]byte, n)
	} else {
		dst = dst[:n]
	}

	if err := encodeBlocksInto(dst, rgba, width, height, format, opts); err != nil {
		return nil, 0, 0, err
	}

	return dst, width, height, nil
}

// imageToTightRGBA returns the image pixels as a tight width*height*4 NRGBA buffer.
// An already-tight, origin-anchored *image.NRGBA is read in place (no copy);
// otherwise a fresh buffer is allocated and filled row by row.
func imageToTightRGBA(img image.Image) (rgba []byte, width, height int) {
	nrgba := toNRGBA(img)
	b := nrgba.Bounds()
	width = b.Dx()
	height = b.Dy()

	if nrgba.Rect.Min.X == 0 && nrgba.Rect.Min.Y == 0 && nrgba.Stride == width*4 && len(nrgba.Pix) >= width*height*4 {
		return nrgba.Pix[: width*height*4 : width*height*4], width, height
	}

	rgba = make([]byte, width*height*4)
	for y := range height {
		src := nrgba.Pix[nrgba.PixOffset(b.Min.X, b.Min.Y+y):]
		copy(rgba[y*width*4:(y+1)*width*4], src[:width*4])
	}

	return rgba, width, height
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

// DecodeImageInto decodes BCn blocks into a reusable destination image and returns it.
// When dst is non-nil and its Pix capacity is large enough,
// the existing buffer is reused (no allocation); otherwise a new image is allocated.
// Pass the returned image back on the next call
// to reuse its buffer across decodes of varying sizes.
func DecodeImageInto(dst *image.NRGBA, data []byte, width, height int, format Format, opts *DecodeOptions) (*image.NRGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	need := width * height * 4
	var pix []byte
	if dst != nil && cap(dst.Pix) >= need {
		pix = dst.Pix[:need]
	} else {
		pix = make([]byte, need)
	}

	img := &image.NRGBA{Pix: pix, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
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
