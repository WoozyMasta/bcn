// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 mode 5: a single subset with separate color and alpha index sets.
// Color uses 7-bit RGB endpoints (no P-bit) with 2-bit indices;
// alpha uses full 8-bit endpoints with its own 2-bit indices.
// Decoupling the two lets alpha-bearing blocks keep crisp color and crisp alpha independently,
// which the shared index of mode 6 cannot.
// The encoder searches the channel rotations (color <-> alpha) and keeps the best.

// bc7Expand7NoP expands a 7-bit color value to 8 bits (mode 5 has no P-bit).
func bc7Expand7NoP(stored uint8) uint8 {
	v := stored << 1
	return v | v>>7
}

// bc7Quant7NoP rounds an 8-bit channel to the nearest 7-bit value.
func bc7Quant7NoP(target uint8) uint8 {
	seed := int(target) * 127 / 255
	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 1; ds++ {
		s := seed + ds
		if s < 0 || s > 127 {
			continue
		}

		// #nosec G115 -- s is in [0,127].
		d := int(bc7Expand7NoP(uint8(s))) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr, best = d, s
		}
	}

	// #nosec G115 -- best is in [0,127].
	return uint8(best)
}

// bc7QuantColor5 quantizes an RGB endpoint to mode 5's 7-bit precision.
func bc7QuantColor5(c rgba8) rgba8 {
	return rgba8{r: bc7Quant7NoP(c.r), g: bc7Quant7NoP(c.g), b: bc7Quant7NoP(c.b)}
}

// bc7ExpandColor5 expands a quantized RGB endpoint to 8 bits (alpha unused).
func bc7ExpandColor5(q rgba8) rgba8 {
	return rgba8{r: bc7Expand7NoP(q.r), g: bc7Expand7NoP(q.g), b: bc7Expand7NoP(q.b)}
}

// bc7Color5Palette builds the 4 interpolated RGB colors for mode 5.
func bc7Color5Palette(e0, e1 rgba8) [4]rgba8 {
	var pal [4]rgba8
	w := bc7Weight2[:]
	for i := range 4 {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), w, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), w, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), w, i),
		}
	}

	return pal
}

// bc7Color5Nearest returns the nearest color index (RGB) and its error.
func bc7Color5Nearest(px rgba8, pal *[4]rgba8) (int, int) {
	best, bestErr := 0, bc7RGBErr(px, pal[0])
	for k := 1; k < 4; k++ {
		if e := bc7RGBErr(px, pal[k]); e < bestErr {
			best, bestErr = k, e
		}
	}

	return best, bestErr
}

// bc7MaxDistRGB returns the two most distant texels by RGB error.
func bc7MaxDistRGB(block *[16]rgba8) (rgba8, rgba8) {
	bi, bj, bestD := 0, 0, -1
	for i := range 16 {
		for j := i + 1; j < 16; j++ {
			if d := bc7RGBErr(block[i], block[j]); d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}

	return block[bi], block[bj]
}

// bc7Mode5FitColor fits the RGB endpoints:
// a max-distance seed plus iterated least-squares refinement, quantized to 7 bits.
func bc7Mode5FitColor(block *[16]rgba8) (rgba8, rgba8) {
	c0, c1 := bc7MaxDistRGB(block)
	q0, q1 := bc7QuantColor5(c0), bc7QuantColor5(c1)
	pal := bc7Color5Palette(bc7ExpandColor5(q0), bc7ExpandColor5(q1))
	bestErr := bc7Color5Error(block, &pal)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := bc7Mode5ColorLSQ(block, &pal)
		if !ok {
			break
		}

		nq0, nq1 := bc7QuantColor5(nc0), bc7QuantColor5(nc1)
		npal := bc7Color5Palette(bc7ExpandColor5(nq0), bc7ExpandColor5(nq1))
		if err := bc7Color5Error(block, &npal); err < bestErr {
			bestErr, q0, q1, pal = err, nq0, nq1, npal
			if bestErr == 0 {
				break
			}
			continue
		}

		break
	}

	return q0, q1
}

// bc7Color5Error sums the nearest-color RGB error over all texels.
func bc7Color5Error(block *[16]rgba8, pal *[4]rgba8) int {
	total := 0
	for i := range 16 {
		_, e := bc7Color5Nearest(block[i], pal)
		total += e
	}

	return total
}

// bc7Mode5ColorLSQ refits continuous RGB endpoints from the current nearest 2-bit assignment.
// ok is false on a degenerate weight distribution.
func bc7Mode5ColorLSQ(block *[16]rgba8, pal *[4]rgba8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	// TODO(avo): per-texel nearest-index search + accumulation is the hot loop.
	for i := range 16 {
		idx, _ := bc7Color5Nearest(block[i], pal)
		b := int(bc7Weight2[idx])
		a := 64 - b
		saa += a * a
		sbb += b * b
		sab += a * b
		ch := [3]int{int(block[i].r), int(block[i].g), int(block[i].b)}
		for c := range 3 {
			sap[c] += a * ch[c]
			sbp[c] += b * ch[c]
		}
	}

	denom := int64(saa)*int64(sbb) - int64(sab)*int64(sab)
	if denom == 0 {
		return rgba8{}, rgba8{}, false
	}

	var e0, e1 rgba8
	dst0 := [3]*uint8{&e0.r, &e0.g, &e0.b}
	dst1 := [3]*uint8{&e1.r, &e1.g, &e1.b}
	for c := range 3 {
		v0, v1 := lsqSolvePair(64, saa, sbb, sab, sap[c], sbp[c], denom)
		*dst0[c] = clampU8(v0)
		*dst1[c] = clampU8(v1)
	}

	return e0, e1, true
}

// bc7Alpha5Levels builds the 4 interpolated alpha values for endpoints a0, a1.
func bc7Alpha5Levels(a0, a1 uint8) [4]uint8 {
	var lv [4]uint8
	w := bc7Weight2[:]
	for i := range 4 {
		lv[i] = bc7Interpolate(int32(a0), int32(a1), w, i)
	}

	return lv
}

// bc7Alpha5Nearest returns the nearest alpha index and its squared error.
func bc7Alpha5Nearest(a uint8, lv *[4]uint8) (int, int) {
	best, bestErr := 0, 0
	for k := range 4 {
		d := int(a) - int(lv[k])
		e := d * d
		if k == 0 || e < bestErr {
			best, bestErr = k, e
		}
	}

	return best, bestErr
}

// bc7Mode5FitAlpha fits the 8-bit alpha endpoints with iterated least squares.
func bc7Mode5FitAlpha(block *[16]rgba8) (uint8, uint8) {
	a0, a1 := block[0].a, block[0].a
	for i := 1; i < 16; i++ {
		a0 = max(a0, block[i].a)
		a1 = min(a1, block[i].a)
	}
	lv := bc7Alpha5Levels(a0, a1)
	bestErr := bc7Alpha5Error(block, &lv)

	const maxIters = 8
	for range maxIters {
		na0, na1, ok := bc7Mode5AlphaLSQ(block, &lv)
		if !ok {
			break
		}

		nlv := bc7Alpha5Levels(na0, na1)
		if err := bc7Alpha5Error(block, &nlv); err < bestErr {
			bestErr, a0, a1, lv = err, na0, na1, nlv
			if bestErr == 0 {
				break
			}
			continue
		}

		break
	}

	return a0, a1
}

// bc7Alpha5Error sums the nearest-alpha squared error over all texels.
func bc7Alpha5Error(block *[16]rgba8, lv *[4]uint8) int {
	total := 0
	for i := range 16 {
		_, e := bc7Alpha5Nearest(block[i].a, lv)
		total += e
	}

	return total
}

// bc7Mode5AlphaLSQ refits continuous alpha endpoints
// from the current nearest 2-bit assignment.
func bc7Mode5AlphaLSQ(block *[16]rgba8, lv *[4]uint8) (uint8, uint8, bool) {
	var saa, sbb, sab, sap, sbp int
	for i := range 16 {
		idx, _ := bc7Alpha5Nearest(block[i].a, lv)
		b := int(bc7Weight2[idx])
		a := 64 - b
		saa += a * a
		sbb += b * b
		sab += a * b
		sap += a * int(block[i].a)
		sbp += b * int(block[i].a)
	}

	denom := int64(saa)*int64(sbb) - int64(sab)*int64(sab)
	if denom == 0 {
		return 0, 0, false
	}

	v0, v1 := lsqSolvePair(64, saa, sbb, sab, sap, sbp, denom)
	return clampU8(v0), clampU8(v1), true
}

// bc7RotateBlock swaps one color channel with alpha per the BC7 rotation field (0 = identity).
// Modes 4 and 5 fit the rotated block so the separately-indexed "alpha" slot
// can carry whichever channel decorrelates from the rest; the decoder undoes the swap.
// RGBA error is invariant under the swap, so fitting the rotated block measures the true error.
func bc7RotateBlock(block [16]rgba8, rotation int) [16]rgba8 {
	if rotation == 0 {
		return block
	}
	var out [16]rgba8
	for i, c := range block {
		switch rotation {
		case 1:
			out[i] = rgba8{r: c.a, g: c.g, b: c.b, a: c.r}
		case 2:
			out[i] = rgba8{r: c.r, g: c.a, b: c.b, a: c.g}
		case 3:
			out[i] = rgba8{r: c.r, g: c.g, b: c.a, a: c.b}
		}
	}
	return out
}

// encodeBC7Mode5 encodes a block as BC7 mode 5,
// trying the given number of channel rotations (1 = rotation 0 only, 4 = all)
// and keeping the best.
func encodeBC7Mode5(block [16]rgba8, rotations int) ([16]byte, int) {
	var bestBytes [16]byte
	bestErr := -1
	for r := range rotations {
		// #nosec G115 -- r is in [0,3].
		if b, e := bc7Mode5Rotated(bc7RotateBlock(block, r), uint8(r)); bestErr < 0 || e < bestErr {
			bestErr, bestBytes = e, b
		}
	}
	return bestBytes, bestErr
}

// bc7Mode5Rotated encodes one already-rotated block as mode 5 with the given
// rotation field, returning the packed block and its (rotation-invariant) error.
func bc7Mode5Rotated(block [16]rgba8, rotation uint8) ([16]byte, int) {
	cq0, cq1 := bc7Mode5FitColor(&block)
	a0, a1 := bc7Mode5FitAlpha(&block)

	cpal := bc7Color5Palette(bc7ExpandColor5(cq0), bc7ExpandColor5(cq1))
	alv := bc7Alpha5Levels(a0, a1)

	var cidx, aidx [16]uint8
	for i := range 16 {
		ci, _ := bc7Color5Nearest(block[i], &cpal)
		ai, _ := bc7Alpha5Nearest(block[i].a, &alv)
		// #nosec G115 -- indices are in [0,3].
		cidx[i], aidx[i] = uint8(ci), uint8(ai)
	}

	// Anchor (texel 0) of each index set must have its MSB clear
	// (2-bit ->// 1-bit);
	// if not, swap that endpoint pair and re-assign.
	if cidx[0]&0x2 != 0 {
		cq0, cq1 = cq1, cq0
		cpal = bc7Color5Palette(bc7ExpandColor5(cq0), bc7ExpandColor5(cq1))
		for i := range 16 {
			ci, _ := bc7Color5Nearest(block[i], &cpal)
			// #nosec G115 -- index is in [0,3].
			cidx[i] = uint8(ci)
		}
	}

	if aidx[0]&0x2 != 0 {
		a0, a1 = a1, a0
		alv = bc7Alpha5Levels(a0, a1)
		for i := range 16 {
			ai, _ := bc7Alpha5Nearest(block[i].a, &alv)
			// #nosec G115 -- index is in [0,3].
			aidx[i] = uint8(ai)
		}
	}

	total := 0
	for i := range 16 {
		rec := rgba8{r: cpal[cidx[i]].r, g: cpal[cidx[i]].g, b: cpal[cidx[i]].b, a: alv[aidx[i]]}
		total += bc7SSE(block[i], rec)
	}

	return bc7PackMode5(rotation, cq0, cq1, a0, a1, &cidx, &aidx), total
}

// bc7PackMode5 serializes a mode 5 block: mode bits, rotation,
// 7-bit RGB endpoints (channel-major),
// 8-bit alpha endpoints,
// then the color and alpha index sets (texel 0 anchored to one fewer bit in each set).
func bc7PackMode5(rotation uint8, cq0, cq1 rgba8, a0, a1 uint8, cidx, aidx *[16]uint8) [16]byte {
	var w bptcWriter
	w.put(1<<5, 6)             // mode 5
	w.put(uint32(rotation), 2) // rotation

	w.put(uint32(cq0.r), 7)
	w.put(uint32(cq1.r), 7)
	w.put(uint32(cq0.g), 7)
	w.put(uint32(cq1.g), 7)
	w.put(uint32(cq0.b), 7)
	w.put(uint32(cq1.b), 7)
	w.put(uint32(a0), 8)
	w.put(uint32(a1), 8)

	for i := range 16 {
		bits := 2
		if i == 0 {
			bits = 1
		}
		w.put(uint32(cidx[i]), bits)
	}
	for i := range 16 {
		bits := 2
		if i == 0 {
			bits = 1
		}
		w.put(uint32(aidx[i]), bits)
	}

	return w.bytes()
}
