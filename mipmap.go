// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"image"
	"math"
	"sync"
)

var (
	// gammaOnce initializes conversion lookup tables once per process.
	gammaOnce sync.Once
	// srgbToLinear maps 8-bit sRGB to linear intensity.
	srgbToLinear [256]float32
	// linearToSRGB maps linear intensity to nearest 8-bit sRGB via 12-bit index.
	linearToSRGB [4096]uint8
)

// GenerateMipmaps builds a full mip chain from the input image.
// If useSRGB is true, RGB is averaged in linear space and converted back to sRGB.
func GenerateMipmaps(img image.Image, useSRGB bool) []*image.NRGBA {
	return GenerateMipmapsN(img, 0, useSRGB)
}

// GenerateMipmapsN builds a mip chain from the input image with an optional level limit.
// maxMipmaps <= 0 builds a full chain, maxMipmaps == 1 returns only the base level.
// If useSRGB is true, RGB is averaged in linear space and converted back to sRGB.
func GenerateMipmapsN(img image.Image, maxMipmaps int, useSRGB bool) []*image.NRGBA {
	base := toNRGBA(img)
	w := base.Rect.Dx()
	h := base.Rect.Dy()
	mipCount := fullMipCount(w, h)
	if maxMipmaps > 0 && maxMipmaps < mipCount {
		mipCount = maxMipmaps
	}

	mips := make([]*image.NRGBA, 1, mipCount)
	mips[0] = base
	for len(mips) < mipCount {
		if w > 1 {
			w >>= 1
		}
		if h > 1 {
			h >>= 1
		}
		base = downscaleNRGBAInto(nil, base, w, h, useSRGB)
		mips = append(mips, base)
	}

	return mips
}

// GenerateMipmapsInto builds a mip chain reusing the buffers in dst across calls.
// Level 0 is the input image itself (not copied);
// levels 1..N-1 reuse the matching dst[i] Pix buffer when large enough, otherwise allocate.
// Pass the returned slice back on the next call to reuse buffers across images of varying sizes.
// The chain is identical to GenerateMipmapsN.
func GenerateMipmapsInto(dst []*image.NRGBA, img image.Image, maxMipmaps int, useSRGB bool) []*image.NRGBA {
	base := toNRGBA(img)
	w := base.Rect.Dx()
	h := base.Rect.Dy()
	mipCount := fullMipCount(w, h)
	if maxMipmaps > 0 && maxMipmaps < mipCount {
		mipCount = maxMipmaps
	}

	// Reuse the slice header's backing array when it is large enough.
	// out may alias dst: at each step we read the reuse buffer from dst[i]
	// before append writes index i, so reuse stays valid.
	out := dst[:0]
	if cap(out) < mipCount {
		out = make([]*image.NRGBA, 0, mipCount)
	}
	out = append(out, base)

	for len(out) < mipCount {
		if w > 1 {
			w >>= 1
		}
		if h > 1 {
			h >>= 1
		}

		i := len(out)
		var reuse *image.NRGBA
		if i < len(dst) {
			reuse = dst[i]
		}

		level := downscaleNRGBAInto(reuse, base, w, h, useSRGB)
		out = append(out, level)
		base = level
	}

	return out
}

func fullMipCount(w, h int) int {
	count := 1
	for w > 1 || h > 1 {
		if w > 1 {
			w >>= 1
		}
		if h > 1 {
			h >>= 1
		}
		count++
	}

	return count
}

// toNRGBA converts an image.Image to NRGBA (no premultiply), copying pixels.
func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok && nrgba.Rect.Min.X == 0 && nrgba.Rect.Min.Y == 0 {
		return nrgba
	}

	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if nrgba, ok := img.(*image.NRGBA); ok {
		for y := 0; y < b.Dy(); y++ {
			srcOff := nrgba.PixOffset(b.Min.X, b.Min.Y+y)
			dstOff := y * out.Stride
			copy(out.Pix[dstOff:dstOff+b.Dx()*4], nrgba.Pix[srcOff:srcOff+b.Dx()*4])
		}
		return out
	}

	if rgba, ok := img.(*image.RGBA); ok {
		for y := 0; y < b.Dy(); y++ {
			srcOff := rgba.PixOffset(b.Min.X, b.Min.Y+y)
			dstOff := y * out.Stride
			copy(out.Pix[dstOff:dstOff+b.Dx()*4], rgba.Pix[srcOff:srcOff+b.Dx()*4])
		}
		return out
	}

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

// downscaleNRGBAInto downsamples src by 2x into dst using a box filter,
// reusing dst's Pix buffer when its capacity is large enough
// (otherwise a new image is allocated).
// For non-power-of-two edges the last row/column is replicated.
// dst must not alias src; a nil dst always allocates.
func downscaleNRGBAInto(dst, src *image.NRGBA, dstW, dstH int, useSRGB bool) *image.NRGBA {
	need := dstW * dstH * 4
	if dst != nil && cap(dst.Pix) >= need {
		dst.Pix = dst.Pix[:need]
		dst.Stride = dstW * 4
		dst.Rect = image.Rect(0, 0, dstW, dstH)
	} else {
		dst = image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	}
	srcW := src.Rect.Dx()
	srcH := src.Rect.Dy()
	if useSRGB {
		initGamma()
	}

	for y := range dstH {
		sy0 := y * 2
		sy1 := sy0 + 1
		if sy1 >= srcH {
			sy1 = srcH - 1
		}
		row0 := src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+sy0)
		row1 := src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+sy1)
		dstOff := y * dst.Stride
		x0 := 0
		if !useSRGB && srcW > 1 {
			n := dstW &^ 1
			if downscaleNRGBARow2xASM(dst.Pix[dstOff:], src.Pix[row0:], src.Pix[row1:], n) {
				x0 = n
			}
		}

		for x := x0; x < dstW; x++ {
			sx0 := x * 2
			sx1 := sx0 + 1
			if sx1 >= srcW {
				sx1 = srcW - 1
			}
			i00 := row0 + sx0*4
			i10 := row0 + sx1*4
			i01 := row1 + sx0*4
			i11 := row1 + sx1*4
			o := dstOff + x*4

			if useSRGB {
				dst.Pix[o+0] = linearToSRGB[clampIndex(avg4LinearSRGB(src.Pix[i00+0], src.Pix[i10+0], src.Pix[i01+0], src.Pix[i11+0]))]
				dst.Pix[o+1] = linearToSRGB[clampIndex(avg4LinearSRGB(src.Pix[i00+1], src.Pix[i10+1], src.Pix[i01+1], src.Pix[i11+1]))]
				dst.Pix[o+2] = linearToSRGB[clampIndex(avg4LinearSRGB(src.Pix[i00+2], src.Pix[i10+2], src.Pix[i01+2], src.Pix[i11+2]))]
			} else {
				dst.Pix[o+0] = avg4U8(src.Pix[i00+0], src.Pix[i10+0], src.Pix[i01+0], src.Pix[i11+0])
				dst.Pix[o+1] = avg4U8(src.Pix[i00+1], src.Pix[i10+1], src.Pix[i01+1], src.Pix[i11+1])
				dst.Pix[o+2] = avg4U8(src.Pix[i00+2], src.Pix[i10+2], src.Pix[i01+2], src.Pix[i11+2])
			}

			dst.Pix[o+3] = avg4U8(src.Pix[i00+3], src.Pix[i10+3], src.Pix[i01+3], src.Pix[i11+3])
		}
	}

	return dst
}

func avg4U8(a, b, c, d uint8) uint8 {
	// #nosec G115 -- max ((255*4)+2)>>2 = 255, always fits uint8.
	return uint8((uint32(a) + uint32(b) + uint32(c) + uint32(d) + 2) >> 2)
}

func avg4LinearSRGB(a, b, c, d uint8) float64 {
	return (float64(srgbToLinear[a]) +
		float64(srgbToLinear[b]) +
		float64(srgbToLinear[c]) +
		float64(srgbToLinear[d])) * 0.25
}

// initGamma precomputes sRGB<->linear lookup tables used by mip generation.
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

// clampIndex maps normalized linear value to linearToSRGB lookup table index.
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
