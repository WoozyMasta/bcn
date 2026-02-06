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
	rgba := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		src := nrgba.Pix[nrgba.PixOffset(b.Min.X, b.Min.Y+y):]
		copy(rgba[y*width*4:(y+1)*width*4], src[:width*4])
	}

	data, err := encodeBlocksWithOptions(rgba, width, height, format, opts)

	return data, width, height, err
}

// DecodeImage decodes BCn blocks into a new image.NRGBA.
func DecodeImage(data []byte, width, height int, format Format) (*image.NRGBA, error) {
	rgba, err := decodeBlocks(data, width, height, format)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, rgba)

	return img, nil
}

// DecodeImageWithOptions decodes BCn blocks into a new image.NRGBA with options.
func DecodeImageWithOptions(data []byte, width, height int, format Format, opts *DecodeOptions) (*image.NRGBA, error) {
	rgba, err := decodeBlocksWithOptions(data, width, height, format, opts)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, rgba)

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

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*img.Stride + x*4
			img.Pix[i+0] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}

	return img
}
