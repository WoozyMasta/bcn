package bcn

type rgba8 struct {
	r, g, b, a uint8
}

func rgbaFromNRGBA(p []byte, i int) rgba8 {
	return rgba8{p[i], p[i+1], p[i+2], p[i+3]}
}

func rgbaToNRGBA(p []byte, i int, c rgba8) {
	p[i] = c.r
	p[i+1] = c.g
	p[i+2] = c.b
	p[i+3] = c.a
}

func rgb565(c rgba8) uint16 {
	r := uint16(c.r) >> 3
	g := uint16(c.g) >> 2
	b := uint16(c.b) >> 3
	return (r << 11) | (g << 5) | b
}

func rgbaFrom565(v uint16) rgba8 {
	r := int((v >> 11) & 0x1F)
	g := int((v >> 5) & 0x3F)
	b := int(v & 0x1F)
	return rgba8{
		r: clampU8((r*255 + 15) / 31),
		g: clampU8((g*255 + 31) / 63),
		b: clampU8((b*255 + 15) / 31),
		a: 255,
	}
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

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
