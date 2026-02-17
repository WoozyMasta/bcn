// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"image"
	"image/color"
	"math"
	"sync"
)

// GenerateMipmaps builds a full mip chain from the input image.
// If useSRGB is true, RGB is averaged in linear space and converted back to sRGB.
func GenerateMipmaps(img image.Image, useSRGB bool) []*image.NRGBA {
	base := toNRGBA(img)
	mips := []*image.NRGBA{base}
	w := base.Rect.Dx()
	h := base.Rect.Dy()
	for w > 1 || h > 1 {
		if w > 1 {
			w >>= 1
		}
		if h > 1 {
			h >>= 1
		}
		base = downscaleNRGBA(base, w, h, useSRGB)
		mips = append(mips, base)
	}

	return mips
}

// toNRGBA converts an image.Image to NRGBA (no premultiply), copying pixels.
func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}

	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	idx := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			out.Pix[idx+0] = clampU8(int(r >> 8))
			out.Pix[idx+1] = clampU8(int(g >> 8))
			out.Pix[idx+2] = clampU8(int(b >> 8))
			out.Pix[idx+3] = clampU8(int(a >> 8))
			idx += 4
		}
	}

	return out
}

// downscaleNRGBA downsamples by 2x using a box filter.
// For non-power-of-two edges, the last row/column is replicated.
func downscaleNRGBA(src *image.NRGBA, dstW, dstH int, useSRGB bool) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	initGamma()

	for y := range dstH {
		for x := range dstW {
			sx := x * 2
			sy := y * 2
			samples := [4]color.NRGBA{
				atNRGBA(src, sx, sy),
				atNRGBA(src, sx+1, sy),
				atNRGBA(src, sx, sy+1),
				atNRGBA(src, sx+1, sy+1),
			}

			var r, g, b float64
			var a int
			for i := range 4 {
				if useSRGB {
					r += float64(srgbToLinear[samples[i].R])
					g += float64(srgbToLinear[samples[i].G])
					b += float64(srgbToLinear[samples[i].B])
				} else {
					r += float64(samples[i].R)
					g += float64(samples[i].G)
					b += float64(samples[i].B)
				}
				a += int(samples[i].A)
			}

			if useSRGB {
				r /= 4
				g /= 4
				b /= 4
				dst.Pix[(y*dst.Stride)+x*4+0] = linearToSRGB[clampIndex(r)]
				dst.Pix[(y*dst.Stride)+x*4+1] = linearToSRGB[clampIndex(g)]
				dst.Pix[(y*dst.Stride)+x*4+2] = linearToSRGB[clampIndex(b)]
			} else {
				dst.Pix[(y*dst.Stride)+x*4+0] = uint8(math.Round(r / 4))
				dst.Pix[(y*dst.Stride)+x*4+1] = uint8(math.Round(g / 4))
				dst.Pix[(y*dst.Stride)+x*4+2] = uint8(math.Round(b / 4))
			}

			dst.Pix[(y*dst.Stride)+x*4+3] = clampU8((a + 2) / 4)
		}
	}

	return dst
}

func atNRGBA(img *image.NRGBA, x, y int) color.NRGBA {
	if x >= img.Rect.Dx() {
		x = img.Rect.Dx() - 1
	}
	if y >= img.Rect.Dy() {
		y = img.Rect.Dy() - 1
	}
	off := y*img.Stride + x*4

	return color.NRGBA{
		R: img.Pix[off+0],
		G: img.Pix[off+1],
		B: img.Pix[off+2],
		A: img.Pix[off+3],
	}
}

var (
	gammaOnce    sync.Once
	srgbToLinear [256]float32
	linearToSRGB [4096]uint8
)

func initGamma() {
	gammaOnce.Do(func() {
		for i := range 256 {
			v := float64(i) / 255.0
			if v <= 0.04045 {
				srgbToLinear[i] = float32(v / 12.92)
			} else {
				srgbToLinear[i] = float32(math.Pow((v+0.055)/1.055, 2.4))
			}
		}

		for i := range len(linearToSRGB) {
			v := float64(i) / float64(len(linearToSRGB)-1)
			var s float64
			if v <= 0.0031308 {
				s = v * 12.92
			} else {
				s = 1.055*math.Pow(v, 1.0/2.4) - 0.055
			}

			if s < 0 {
				s = 0
			} else if s > 1 {
				s = 1
			}

			linearToSRGB[i] = uint8(math.Round(s * 255))
		}
	})
}

func clampIndex(v float64) int {
	if v <= 0 {
		return 0
	}

	maxIdx := float64(len(linearToSRGB) - 1)
	idx := int(math.Round(v * maxIdx))
	if idx < 0 {
		return 0
	}

	if idx > int(maxIdx) {
		return int(maxIdx)
	}

	return idx
}
