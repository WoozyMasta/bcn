// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 (BPTC unorm) encoding.
//
// Each block is encoded by every applicable mode and the lowest-error result wins.
// Opaque blocks use modes 6, 1, 3 and the three-subset modes 0/2;
// alpha blocks use modes 6, 5, 4 and 7. Each mode lives in its own bc7_mode*.go file.
//
// Endpoint fitting reuses the least-squares solver from lsq.go:
// BC7's non-uniform interpolation weights (bc7Weight4) are just per-texel beta values,
// so the closed-form endpoint solve carries over with d = 64.

// EncodeBC7 encodes an RGBA image (NRGBA layout) into BC7 blocks.
func EncodeBC7(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC7, nil)
}

// EncodeBC7WithOptions encodes with explicit options. QualityLevel controls the
// endpoint refinement budget.
func EncodeBC7WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC7, opts)
}

// bc7SSE returns the squared RGBA error between two pixels.
func bc7SSE(p, q rgba8) int {
	dr := int(p.r) - int(q.r)
	dg := int(p.g) - int(q.g)
	db := int(p.b) - int(q.b)
	da := int(p.a) - int(q.a)

	return dr*dr + dg*dg + db*db + da*da
}

// bc7ExpandMode6 reconstructs the 8-bit endpoint from its 7-bit value and the shared P-bit
// (mode 6 has 8-bit precision, so no MSB replication is needed).
func bc7ExpandMode6(q rgba8, pbit uint8) rgba8 {
	return rgba8{
		r: q.r<<1 | pbit,
		g: q.g<<1 | pbit,
		b: q.b<<1 | pbit,
		a: q.a<<1 | pbit,
	}
}

// bc7QuantizeMode6 rounds an 8-bit endpoint to mode 6's 7-bit value plus a single shared P-bit,
// choosing the P-bit that minimizes the RGBA round-trip error
// (the P-bit is the common LSB of all four channels).
func bc7QuantizeMode6(c rgba8) (rgba8, uint8) {
	var best rgba8
	var bestPBit uint8
	bestErr := -1

	for pb := range 2 {
		q := rgba8{
			r: bc7Store7(c.r, pb),
			g: bc7Store7(c.g, pb),
			b: bc7Store7(c.b, pb),
			a: bc7Store7(c.a, pb),
		}

		// #nosec G115 -- pb is 0 or 1.
		dec := bc7ExpandMode6(q, uint8(pb))
		if e := bc7SSE(c, dec); bestErr < 0 || e < bestErr {
			bestErr = e
			best = q
			// #nosec G115 -- pb is 0 or 1.
			bestPBit = uint8(pb)
		}
	}

	return best, bestPBit
}

// bc7Store7 rounds an 8-bit channel to the nearest 7-bit value reachable
// with the given P-bit: value ~= store<<1 | pbit.
func bc7Store7(v uint8, pbit int) uint8 {
	s := min(max((int(v)-pbit+1)>>1, 0), 127)

	// #nosec G115 -- s is clamped to [0,127].
	return uint8(s)
}

// bc7Mode6Palette builds the 16 interpolated RGBA colors
// for a mode 6 endpoint pair (expanded to 8 bits).
func bc7Mode6Palette(e0, e1 rgba8) [16]rgba8 {
	var pal [16]rgba8
	w := bc7Weight4[:]
	for i := range 16 {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), w, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), w, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), w, i),
			a: bc7Interpolate(int32(e0.a), int32(e1.a), w, i),
		}
	}

	return pal
}

// bc7Mode6Indices assigns each texel its nearest palette index
// and returns the indices together with the total block error.
func bc7Mode6Indices(block [16]rgba8, pal *[16]rgba8) ([16]uint8, int) {
	var idx [16]uint8
	total := 0

	for i, px := range block {
		best := 0
		bestErr := bc7SSE(px, pal[0])
		for k := 1; k < 16; k++ {
			if e := bc7SSE(px, pal[k]); e < bestErr {
				bestErr = e
				best = k
			}
		}

		// #nosec G115 -- best is in [0,15].
		idx[i] = uint8(best)
		total += bestErr
	}

	return idx, total
}

// bc7Mode6LSQ refits continuous 8-bit endpoints for a fixed index assignment
// using the least-squares solver, with beta taken from the BC7 weight table.
// ok is false on a degenerate distribution (all texels share one weight).
func bc7Mode6LSQ(block [16]rgba8, idx *[16]uint8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [4]int

	// TODO(avo): per-texel weight lookup + accumulation is the hot loop.
	for i, px := range block {
		b := int(bc7Weight4[idx[i]])
		a := 64 - b
		saa += a * a
		sbb += b * b
		sab += a * b
		ch := [4]int{int(px.r), int(px.g), int(px.b), int(px.a)}
		for c := range 4 {
			sap[c] += a * ch[c]
			sbp[c] += b * ch[c]
		}
	}

	denom := int64(saa)*int64(sbb) - int64(sab)*int64(sab)
	if denom == 0 {
		return rgba8{}, rgba8{}, false
	}

	var e0, e1 rgba8
	dst0 := [4]*uint8{&e0.r, &e0.g, &e0.b, &e0.a}
	dst1 := [4]*uint8{&e1.r, &e1.g, &e1.b, &e1.a}
	for c := range 4 {
		v0, v1 := lsqSolvePair(64, saa, sbb, sab, sap[c], sbp[c], denom)
		*dst0[c] = clampU8(v0)
		*dst1[c] = clampU8(v1)
	}

	return e0, e1, true
}

// bc7MaxDistPair returns the two most distant texels (by RGBA SSE)
// as an endpoint seed for the least-squares refinement.
func bc7MaxDistPair(block [16]rgba8) (rgba8, rgba8) {
	bi, bj, bestD := 0, 0, -1
	for i := range 16 {
		for j := i + 1; j < 16; j++ {
			if d := bc7SSE(block[i], block[j]); d > bestD {
				bestD = d
				bi, bj = i, j
			}
		}
	}

	return block[bi], block[bj]
}

// encodeBlockBC7 encodes one 4x4 block, trying the applicable modes
// and keeping the one with the lowest reconstruction error. Mode 6 always applies.
// Opaque blocks also try mode 1 (2-subset RGB) and, at higher quality,
// the 3-subset modes 0 and 2;
// alpha blocks try mode 5 (separate color/alpha) and mode 7 (2-subset RGBA).
// The extra modes run only when the quality level enables them.
func encodeBlockBC7(block [16]rgba8, opts EncodeOptions) [16]byte {
	best, bestErr := encodeBC7Mode6(block)
	consider := func(b [16]byte, err int, ok bool) {
		if ok && err < bestErr {
			best, bestErr = b, err
		}
	}

	n := qualitySettingsForOpts(opts).bc7Partitions
	if n == 0 {
		return best
	}

	if bc7BlockHasAlpha(block) {
		b5, e5 := encodeBC7Mode5(block)
		consider(b5, e5, true)
		b4, e4 := encodeBC7Mode4(block)
		consider(b4, e4, true)
		consider(encodeBC7Mode7(block, n))
	} else {
		consider(encodeBC7Mode1(block, n))
		consider(encodeBC7Mode3(block, n))
		if n >= 8 { // 3-subset search is reserved for the higher quality levels
			consider(encodeBC7Mode02(bc7Mode0Params, block, n))
			consider(encodeBC7Mode02(bc7Mode2Params, block, n))
		}
	}

	return best
}

// bc7BlockHasAlpha reports whether any texel is not fully opaque.
func bc7BlockHasAlpha(block [16]rgba8) bool {
	for _, px := range block {
		if px.a != 255 {
			return true
		}
	}
	return false
}

// encodeBC7Mode6 encodes a block as BC7 mode 6
// and returns the packed block together with its total reconstruction error.
func encodeBC7Mode6(block [16]rgba8) ([16]byte, int) {
	c0, c1 := bc7MaxDistPair(block)
	q0, p0 := bc7QuantizeMode6(c0)
	q1, p1 := bc7QuantizeMode6(c1)

	pal := bc7Mode6Palette(bc7ExpandMode6(q0, p0), bc7ExpandMode6(q1, p1))
	idx, bestErr := bc7Mode6Indices(block, &pal)

	// Iterate assign -> least-squares refit -> quantize, keeping the best.
	const maxIters = 16
	for range maxIters {
		nc0, nc1, ok := bc7Mode6LSQ(block, &idx)
		if !ok {
			break
		}
		nq0, np0 := bc7QuantizeMode6(nc0)
		nq1, np1 := bc7QuantizeMode6(nc1)

		npal := bc7Mode6Palette(bc7ExpandMode6(nq0, np0), bc7ExpandMode6(nq1, np1))
		nIdx, err := bc7Mode6Indices(block, &npal)
		if err >= bestErr {
			break
		}

		bestErr = err
		q0, p0, q1, p1 = nq0, np0, nq1, np1
		idx = nIdx
		if bestErr == 0 {
			break
		}
	}

	// The anchor index (texel 0) must have its MSB clear; if not,
	// swap the endpoints (which inverts every index) and re-assign.
	if idx[0]&0x8 != 0 {
		q0, q1 = q1, q0
		p0, p1 = p1, p0
		pal = bc7Mode6Palette(bc7ExpandMode6(q0, p0), bc7ExpandMode6(q1, p1))
		idx, bestErr = bc7Mode6Indices(block, &pal)
	}

	return bc7PackMode6(q0, p0, q1, p1, &idx), bestErr
}

// bc7PackMode6 serializes a mode 6 block: the mode bit,
// RGBA endpoints (7 bits each, channel-major),
// the two P-bits, and the indices (texel 0 anchored to 3 bits, the rest 4 bits).
// The layout mirrors the reference decoder.
func bc7PackMode6(q0 rgba8, p0 uint8, q1 rgba8, p1 uint8, idx *[16]uint8) [16]byte {
	var w bptcWriter
	w.put(1<<6, 7) // mode 6: six zero bits then the set bit
	w.put(uint32(q0.r), 7)
	w.put(uint32(q1.r), 7)
	w.put(uint32(q0.g), 7)
	w.put(uint32(q1.g), 7)
	w.put(uint32(q0.b), 7)
	w.put(uint32(q1.b), 7)
	w.put(uint32(q0.a), 7)
	w.put(uint32(q1.a), 7)
	w.put(uint32(p0), 1)
	w.put(uint32(p1), 1)
	w.put(uint32(idx[0]), 3)
	for i := 1; i < 16; i++ {
		w.put(uint32(idx[i]), 4)
	}
	return w.bytes()
}
