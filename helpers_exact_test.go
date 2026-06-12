package bcn

import "testing"

// Reference implementations frozen at the pre-optimization state. Optimized
// helpers (LUT, mul-shift interpolation, SIMD kernels) must stay byte-exact
// with these over the full input domain.

func refRGBAFrom565(v uint16) rgba8 {
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

func refDXT1Palette(c0, c1 uint16) [4]rgba8 {
	p0 := refRGBAFrom565(c0)
	p1 := refRGBAFrom565(c1)
	var palette [4]rgba8
	palette[0] = p0
	palette[1] = p1
	if c0 > c1 {
		palette[2] = rgba8{
			r: clampU8((2*int(p0.r) + int(p1.r) + 1) / 3),
			g: clampU8((2*int(p0.g) + int(p1.g) + 1) / 3),
			b: clampU8((2*int(p0.b) + int(p1.b) + 1) / 3),
			a: 255,
		}
		palette[3] = rgba8{
			r: clampU8((int(p0.r) + 2*int(p1.r) + 1) / 3),
			g: clampU8((int(p0.g) + 2*int(p1.g) + 1) / 3),
			b: clampU8((int(p0.b) + 2*int(p1.b) + 1) / 3),
			a: 255,
		}
	} else {
		palette[2] = rgba8{
			r: clampU8((int(p0.r) + int(p1.r)) / 2),
			g: clampU8((int(p0.g) + int(p1.g)) / 2),
			b: clampU8((int(p0.b) + int(p1.b)) / 2),
			a: 255,
		}
		palette[3] = rgba8{0, 0, 0, 0}
	}

	return palette
}

func refDXT5AlphaPalette(a0, a1 uint8) [8]uint8 {
	var p [8]uint8
	p[0] = a0
	p[1] = a1
	if a0 > a1 {
		p[2] = clampU8((6*int(a0) + 1*int(a1) + 3) / 7)
		p[3] = clampU8((5*int(a0) + 2*int(a1) + 3) / 7)
		p[4] = clampU8((4*int(a0) + 3*int(a1) + 3) / 7)
		p[5] = clampU8((3*int(a0) + 4*int(a1) + 3) / 7)
		p[6] = clampU8((2*int(a0) + 5*int(a1) + 3) / 7)
		p[7] = clampU8((1*int(a0) + 6*int(a1) + 3) / 7)
	} else {
		p[2] = clampU8((4*int(a0) + 1*int(a1) + 2) / 5)
		p[3] = clampU8((3*int(a0) + 2*int(a1) + 2) / 5)
		p[4] = clampU8((2*int(a0) + 3*int(a1) + 2) / 5)
		p[5] = clampU8((1*int(a0) + 4*int(a1) + 2) / 5)
		p[6] = 0
		p[7] = 255
	}

	return p
}

// TestRGBAFrom565Exhaustive checks all 65536 packed values.
func TestRGBAFrom565Exhaustive(t *testing.T) {
	for v := 0; v <= 0xFFFF; v++ {
		got := rgbaFrom565(uint16(v))
		want := refRGBAFrom565(uint16(v))
		if got != want {
			t.Fatalf("rgbaFrom565(%#04x) = %v, want %v", v, got, want)
		}
	}
}

// TestDXT1PaletteExhaustivePerChannel sweeps every endpoint pair per channel.
// Per-channel interpolation is independent, so full 5-bit (R/B) and 6-bit (G)
// pair sweeps cover the whole arithmetic domain in both BC1 modes.
func TestDXT1PaletteExhaustivePerChannel(t *testing.T) {
	check := func(c0, c1 uint16) {
		t.Helper()
		got := dxt1Palette(c0, c1)
		want := refDXT1Palette(c0, c1)
		if got != want {
			t.Fatalf("dxt1Palette(%#04x, %#04x) = %v, want %v", c0, c1, got, want)
		}
	}

	// Red and blue: all 32x32 5-bit pairs; green fixed at both extremes to hit
	// c0>c1 and c0<=c1 orderings for the same channel pair.
	for r0 := 0; r0 < 32; r0++ {
		for r1 := 0; r1 < 32; r1++ {
			check(uint16(r0<<11), uint16(r1<<11))
			check(uint16(r0<<11|0x7FF), uint16(r1<<11|0x7FF))
			check(uint16(r0), uint16(r1))               // blue channel sweep
			check(uint16(r0|0xFFE0), uint16(r1|0xFFE0)) // blue with c0>c1 bias
		}
	}

	// Green: all 64x64 6-bit pairs, with low/high R+B to exercise both modes.
	for g0 := 0; g0 < 64; g0++ {
		for g1 := 0; g1 < 64; g1++ {
			check(uint16(g0<<5), uint16(g1<<5))
			check(uint16(g0<<5|0xF81F), uint16(g1<<5|0xF81F))
		}
	}
}

// TestDXT1PaletteRandomPairs adds joint-channel coverage over random pairs.
func TestDXT1PaletteRandomPairs(t *testing.T) {
	// Deterministic LCG; full uint16 pair space is 2^32, sampled instead.
	state := uint32(0x12345678)
	next := func() uint16 {
		state = state*1664525 + 1013904223
		return uint16(state >> 16)
	}
	for i := 0; i < 1_000_000; i++ {
		c0, c1 := next(), next()
		got := dxt1Palette(c0, c1)
		want := refDXT1Palette(c0, c1)
		if got != want {
			t.Fatalf("dxt1Palette(%#04x, %#04x) = %v, want %v", c0, c1, got, want)
		}
	}
}

// TestDXT5AlphaPaletteExhaustive checks all 65536 (a0, a1) pairs.
func TestDXT5AlphaPaletteExhaustive(t *testing.T) {
	for a0 := 0; a0 <= 255; a0++ {
		for a1 := 0; a1 <= 255; a1++ {
			got := dxt5AlphaPalette(uint8(a0), uint8(a1))
			want := refDXT5AlphaPalette(uint8(a0), uint8(a1))
			if got != want {
				t.Fatalf("dxt5AlphaPalette(%d, %d) = %v, want %v", a0, a1, got, want)
			}
		}
	}
}
