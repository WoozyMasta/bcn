// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// rgba8 is a compact RGBA pixel used internally in 4x4 block operations.
type rgba8 struct {
	r, g, b, a uint8
}

// rgbaFromNRGBA reads one pixel from a flat NRGBA byte buffer.
func rgbaFromNRGBA(p []byte, i int) rgba8 {
	return rgba8{p[i], p[i+1], p[i+2], p[i+3]}
}

// rgbaToNRGBA writes one pixel into a flat NRGBA byte buffer.
func rgbaToNRGBA(p []byte, i int, c rgba8) {
	p[i] = c.r
	p[i+1] = c.g
	p[i+2] = c.b
	p[i+3] = c.a
}

// rgb565 quantizes an 8-bit RGB pixel into the packed 5:6:5 layout.
func rgb565(c rgba8) uint16 {
	r := uint16(c.r) >> 3
	g := uint16(c.g) >> 2
	b := uint16(c.b) >> 3
	return (r << 11) | (g << 5) | b
}

// lut5to8 expands a 5-bit channel to 8 bits: (v*255 + 15) / 31.
var lut5to8 [32]uint8

// lut6to8 expands a 6-bit channel to 8 bits: (v*255 + 31) / 63.
var lut6to8 [64]uint8

func init() {
	for i := range lut5to8 {
		lut5to8[i] = uint8((i*255 + 15) / 31)
	}
	for i := range lut6to8 {
		lut6to8[i] = uint8((i*255 + 31) / 63)
	}
}

// rgbaFrom565 expands a packed RGB565 value back to 8-bit channels.
func rgbaFrom565(v uint16) rgba8 {
	return rgba8{
		r: lut5to8[(v>>11)&0x1F],
		g: lut6to8[(v>>5)&0x3F],
		b: lut5to8[v&0x1F],
		a: 255,
	}
}

// clampU8 clamps an integer into the uint8 range [0, 255].
func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// insetMinMax shrinks endpoint range slightly to reduce interpolation outliers.
func insetMinMax(minC, maxC rgba8) (rgba8, rgba8) {
	rangeR := int(maxC.r) - int(minC.r)
	rangeG := int(maxC.g) - int(minC.g)
	rangeB := int(maxC.b) - int(minC.b)
	insetR := rangeR >> 4
	insetG := rangeG >> 4
	insetB := rangeB >> 4

	minC.r = clampU8(int(minC.r) + insetR)
	minC.g = clampU8(int(minC.g) + insetG)
	minC.b = clampU8(int(minC.b) + insetB)

	maxC.r = clampU8(int(maxC.r) - insetR)
	maxC.g = clampU8(int(maxC.g) - insetG)
	maxC.b = clampU8(int(maxC.b) - insetB)

	return minC, maxC
}
