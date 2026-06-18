// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 mode 4: a single subset with separate color and alpha index sets like mode 5,
// but with a 1-bit index-selection that hands the wider 3-bit index
// to either color or alpha (the other gets 2 bits).
// Endpoints are lower precision (5-bit RGB, 6-bit alpha),
// so mode 4 wins when one channel needs more interpolation levels
// than mode 5's fixed 2-bit pair can provide.
// Rotation is fixed at 0; both index-selection settings are tried.

// bc7Quant5NoP rounds an 8-bit channel to the nearest 5-bit value (precision 5).
func bc7Quant5NoP(target uint8) uint8 {
	seed := int(target) * 31 / 255
	best, bestErr := 0, 1<<30

	for ds := -1; ds <= 1; ds++ {
		s := seed + ds
		if s < 0 || s > 31 {
			continue
		}

		// #nosec G115 -- s is in [0,31].
		d := int(bc7Expand5(uint8(s))) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr, best = d, s
		}
	}

	// #nosec G115 -- best is in [0,31].
	return uint8(best)
}

// bc7Quant6NoP rounds an 8-bit channel to the nearest 6-bit value (precision 6).
func bc7Quant6NoP(target uint8) uint8 {
	seed := int(target) * 63 / 255
	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 1; ds++ {
		s := seed + ds
		if s < 0 || s > 63 {
			continue
		}

		// #nosec G115 -- s is in [0,63].
		d := int(bc7Expand6(uint8(s))) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr, best = d, s
		}
	}

	// #nosec G115 -- best is in [0,63].
	return uint8(best)
}

// bc7Mode4ColorPalette builds n interpolated RGB colors (5-bit endpoints).
func bc7Mode4ColorPalette(e0, e1 rgba8, weights []int32, n int) [8]rgba8 {
	var pal [8]rgba8
	for i := range n {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), weights, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), weights, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), weights, i),
		}
	}

	return pal
}

// bc7Mode4ColorNearest returns the nearest color index (RGB) and its error.
func bc7Mode4ColorNearest(px rgba8, pal *[8]rgba8, n int) (int, int) {
	best, bestErr := 0, bc7RGBErr(px, pal[0])
	for k := 1; k < n; k++ {
		if e := bc7RGBErr(px, pal[k]); e < bestErr {
			best, bestErr = k, e
		}
	}

	return best, bestErr
}

// bc7Mode4ColorLSQ refits continuous 5-bit RGB endpoints from the assignment.
func bc7Mode4ColorLSQ(block *[16]rgba8, pal *[8]rgba8, weights []int32, n int) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int
	for i := range 16 {
		idx, _ := bc7Mode4ColorNearest(block[i], pal, n)
		b := int(weights[idx])
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

// bc7Mode4ExpandColor expands a quantized 5-bit RGB endpoint to 8 bits.
func bc7Mode4ExpandColor(q rgba8) rgba8 {
	return rgba8{r: bc7Expand5(q.r), g: bc7Expand5(q.g), b: bc7Expand5(q.b)}
}

// bc7Mode4FitColor fits 5-bit RGB endpoints for the given index resolution.
func bc7Mode4FitColor(block *[16]rgba8, weights []int32, n int) (rgba8, rgba8) {
	c0, c1 := bc7MaxDistRGB(block)
	q0 := rgba8{r: bc7Quant5NoP(c0.r), g: bc7Quant5NoP(c0.g), b: bc7Quant5NoP(c0.b)}
	q1 := rgba8{r: bc7Quant5NoP(c1.r), g: bc7Quant5NoP(c1.g), b: bc7Quant5NoP(c1.b)}
	pal := bc7Mode4ColorPalette(bc7Mode4ExpandColor(q0), bc7Mode4ExpandColor(q1), weights, n)
	bestErr := bc7Mode4ColorErr(block, &pal, n)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := bc7Mode4ColorLSQ(block, &pal, weights, n)
		if !ok {
			break
		}

		nq0 := rgba8{r: bc7Quant5NoP(nc0.r), g: bc7Quant5NoP(nc0.g), b: bc7Quant5NoP(nc0.b)}
		nq1 := rgba8{r: bc7Quant5NoP(nc1.r), g: bc7Quant5NoP(nc1.g), b: bc7Quant5NoP(nc1.b)}
		npal := bc7Mode4ColorPalette(bc7Mode4ExpandColor(nq0), bc7Mode4ExpandColor(nq1), weights, n)
		if err := bc7Mode4ColorErr(block, &npal, n); err < bestErr {
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

// bc7Mode4ColorErr sums the nearest-color RGB error over all texels.
func bc7Mode4ColorErr(block *[16]rgba8, pal *[8]rgba8, n int) int {
	total := 0
	for i := range 16 {
		_, e := bc7Mode4ColorNearest(block[i], pal, n)
		total += e
	}

	return total
}

// bc7Mode4AlphaLevels builds n interpolated alpha values (6-bit endpoints).
func bc7Mode4AlphaLevels(a0, a1 uint8, weights []int32, n int) [8]uint8 {
	var lv [8]uint8
	for i := range n {
		lv[i] = bc7Interpolate(int32(a0), int32(a1), weights, i)
	}

	return lv
}

// bc7Mode4AlphaNearest returns the nearest alpha index and its squared error.
func bc7Mode4AlphaNearest(a uint8, lv *[8]uint8, n int) (int, int) {
	best, bestErr := 0, 0
	for k := range n {
		d := int(a) - int(lv[k])
		e := d * d
		if k == 0 || e < bestErr {
			best, bestErr = k, e
		}
	}

	return best, bestErr
}

// bc7Mode4FitAlpha fits 6-bit alpha endpoints for the given index resolution.
func bc7Mode4FitAlpha(block *[16]rgba8, weights []int32, n int) (uint8, uint8) {
	a0, a1 := block[0].a, block[0].a
	for i := 1; i < 16; i++ {
		a0 = max(a0, block[i].a)
		a1 = min(a1, block[i].a)
	}

	a0, a1 = bc7Quant6NoP(a0), bc7Quant6NoP(a1)
	lv := bc7Mode4AlphaLevels(bc7Expand6(a0), bc7Expand6(a1), weights, n)
	bestErr := bc7Mode4AlphaErr(block, &lv, n)

	const maxIters = 8
	for range maxIters {
		na0, na1, ok := bc7Mode4AlphaLSQ(block, &lv, weights, n)
		if !ok {
			break
		}

		nq0, nq1 := bc7Quant6NoP(na0), bc7Quant6NoP(na1)
		nlv := bc7Mode4AlphaLevels(bc7Expand6(nq0), bc7Expand6(nq1), weights, n)
		if err := bc7Mode4AlphaErr(block, &nlv, n); err < bestErr {
			bestErr, a0, a1, lv = err, nq0, nq1, nlv
			if bestErr == 0 {
				break
			}
			continue
		}

		break
	}

	return a0, a1
}

// bc7Mode4AlphaErr sums the nearest-alpha squared error over all texels.
func bc7Mode4AlphaErr(block *[16]rgba8, lv *[8]uint8, n int) int {
	total := 0
	for i := range 16 {
		_, e := bc7Mode4AlphaNearest(block[i].a, lv, n)
		total += e
	}

	return total
}

// bc7Mode4AlphaLSQ refits continuous alpha endpoints
// (returned as 8-bit values before re-quantization) from the current assignment.
func bc7Mode4AlphaLSQ(block *[16]rgba8, lv *[8]uint8, weights []int32, n int) (uint8, uint8, bool) {
	var saa, sbb, sab, sap, sbp int
	for i := range 16 {
		idx, _ := bc7Mode4AlphaNearest(block[i].a, lv, n)
		b := int(weights[idx])
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

// encodeBC7Mode4 encodes a block as BC7 mode 4 (rotation 0),
// trying both index-selection settings
// (3-bit index given to color or to alpha).
func encodeBC7Mode4(block [16]rgba8) ([16]byte, int) {
	var bestBytes [16]byte
	bestErr := -1

	for idxMode := range 2 {
		colorN, alphaN := 4, 8 // idxMode 0: color 2-bit, alpha 3-bit
		colorW, alphaW := bc7Weight2[:], bc7Weight3[:]
		if idxMode == 1 {
			colorN, alphaN = 8, 4
			colorW, alphaW = bc7Weight3[:], bc7Weight2[:]
		}

		cq0, cq1 := bc7Mode4FitColor(&block, colorW, colorN)
		a0, a1 := bc7Mode4FitAlpha(&block, alphaW, alphaN)

		cpal := bc7Mode4ColorPalette(bc7Mode4ExpandColor(cq0), bc7Mode4ExpandColor(cq1), colorW, colorN)
		alv := bc7Mode4AlphaLevels(bc7Expand6(a0), bc7Expand6(a1), alphaW, alphaN)

		var cidx, aidx [16]uint8
		for i := range 16 {
			ci, _ := bc7Mode4ColorNearest(block[i], &cpal, colorN)
			ai, _ := bc7Mode4AlphaNearest(block[i].a, &alv, alphaN)
			// #nosec G115 -- indices are < 8.
			cidx[i], aidx[i] = uint8(ci), uint8(ai)
		}

		// Anchor (texel 0) MSB must be clear for each index set.
		// #nosec G115 -- colorN/alphaN are 4 or 8.
		cMSB, aMSB := uint8(colorN/2), uint8(alphaN/2)
		if cidx[0]&cMSB != 0 {
			cq0, cq1 = cq1, cq0
			cpal = bc7Mode4ColorPalette(bc7Mode4ExpandColor(cq0), bc7Mode4ExpandColor(cq1), colorW, colorN)
			for i := range 16 {
				ci, _ := bc7Mode4ColorNearest(block[i], &cpal, colorN)
				// #nosec G115 -- index < 8.
				cidx[i] = uint8(ci)
			}
		}
		if aidx[0]&aMSB != 0 {
			a0, a1 = a1, a0
			alv = bc7Mode4AlphaLevels(bc7Expand6(a0), bc7Expand6(a1), alphaW, alphaN)
			for i := range 16 {
				ai, _ := bc7Mode4AlphaNearest(block[i].a, &alv, alphaN)
				// #nosec G115 -- index < 8.
				aidx[i] = uint8(ai)
			}
		}

		total := 0
		for i := range 16 {
			rec := rgba8{r: cpal[cidx[i]].r, g: cpal[cidx[i]].g, b: cpal[cidx[i]].b, a: alv[aidx[i]]}
			total += bc7SSE(block[i], rec)
		}

		if bestErr < 0 || total < bestErr {
			bestErr = total
			// #nosec G115 -- idxMode is 0 or 1.
			bestBytes = bc7PackMode4(uint8(idxMode), cq0, cq1, a0, a1, &cidx, &aidx, colorN)
		}
	}

	return bestBytes, bestErr
}

// bc7PackMode4 serializes a mode 4 block.
// The 2-bit index set is always written first (primary)
// and the 3-bit set second (secondary);
// idxMode selects which of color/alpha is which.
// colorN tells whether color holds the 2- or 3-bit set.
func bc7PackMode4(idxMode uint8, cq0, cq1 rgba8, a0, a1 uint8, cidx, aidx *[16]uint8, colorN int) [16]byte {
	var w bptcWriter
	w.put(1<<4, 5) // mode 4
	w.put(0, 2)    // rotation 0
	w.put(uint32(idxMode), 1)

	w.put(uint32(cq0.r), 5)
	w.put(uint32(cq1.r), 5)
	w.put(uint32(cq0.g), 5)
	w.put(uint32(cq1.g), 5)
	w.put(uint32(cq0.b), 5)
	w.put(uint32(cq1.b), 5)
	w.put(uint32(a0), 6)
	w.put(uint32(a1), 6)

	// Primary set is the 2-bit one; secondary is the 3-bit one.
	primary, secondary := cidx, aidx
	if colorN == 8 { // color holds the 3-bit (secondary) set
		primary, secondary = aidx, cidx
	}

	for i := range 16 {
		bits := 2
		if i == 0 {
			bits = 1
		}
		w.put(uint32(primary[i]), bits)
	}

	for i := range 16 {
		bits := 3
		if i == 0 {
			bits = 2
		}
		w.put(uint32(secondary[i]), bits)
	}

	return w.bytes()
}
