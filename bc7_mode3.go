// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 mode 3: two subsets, opaque RGB, 7-bit endpoints plus a per-endpoint P-bit
// (8-bit precision, lossless endpoints) and 2-bit indices over the 64 two-subset partitions.
// It complements mode 1 (6-bit endpoints, 3-bit indices)
// by trading index resolution for full endpoint precision,
// winning on two-region blocks whose subsets are nearly flat.

// bc7ExpandMode3 reconstructs an 8-bit RGB endpoint (alpha forced to 255).
// With 7 bits plus the P-bit the precision is 8, so the value passes through.
func bc7ExpandMode3(q rgba8, pbit uint8) rgba8 {
	return rgba8{r: q.r<<1 | pbit, g: q.g<<1 | pbit, b: q.b<<1 | pbit, a: 255}
}

// bc7QuantMode3 quantizes one RGB endpoint to 7 bits per channel,
// choosing the per-endpoint P-bit that minimizes the round-trip error.
func bc7QuantMode3(c rgba8) (rgba8, uint8) {
	var best rgba8
	var bestPBit uint8
	bestErr := -1

	for pb := range 2 {
		q := rgba8{r: bc7Store7(c.r, pb), g: bc7Store7(c.g, pb), b: bc7Store7(c.b, pb)}
		// #nosec G115 -- pb is 0 or 1.
		pbit := uint8(pb)
		if e := bc7RGBErr(bc7ExpandMode3(q, pbit), c); bestErr < 0 || e < bestErr {
			bestErr, best, bestPBit = e, q, pbit
		}
	}

	return best, bestPBit
}

// bc7Mode3Palette builds the 4 interpolated RGB colors for an endpoint pair.
func bc7Mode3Palette(e0, e1 rgba8) [4]rgba8 {
	var pal [4]rgba8
	w := bc7Weight2[:]

	for i := range 4 {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), w, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), w, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), w, i),
			a: 255,
		}
	}

	return pal
}

// bc7Mode3SubsetError sums the nearest-entry RGB error over the pixels of one subset.
func bc7Mode3SubsetError(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) int {
	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < 4; k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr = e
			}
		}

		total += bestErr
	}

	return total
}

// bc7Mode3SubsetLSQ refits continuous RGB endpoints for one subset
// from its current nearest 2-bit assignment.
//
//nolint:dupl // per-mode BC7 subset fits are intentionally kept separate.
func bc7Mode3SubsetLSQ(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	// TODO(avo): per-texel nearest-index search + accumulation is the hot loop.
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < 4; k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

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

// bc7Mode3FitSubset fits one subset's RGB endpoints
// (max-distance seed plus iterated least-squares refinement)
// at mode 3 precision.
func bc7Mode3FitSubset(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8, uint8, uint8) {
	c0, c1 := bc73SubsetMaxDist(block, part, subset)
	q0, pb0 := bc7QuantMode3(c0)
	q1, pb1 := bc7QuantMode3(c1)
	pal := bc7Mode3Palette(bc7ExpandMode3(q0, pb0), bc7ExpandMode3(q1, pb1))
	bestErr := bc7Mode3SubsetError(block, part, subset, &pal)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := bc7Mode3SubsetLSQ(block, part, subset, &pal)
		if !ok {
			break
		}

		nq0, npb0 := bc7QuantMode3(nc0)
		nq1, npb1 := bc7QuantMode3(nc1)
		npal := bc7Mode3Palette(bc7ExpandMode3(nq0, npb0), bc7ExpandMode3(nq1, npb1))
		if err := bc7Mode3SubsetError(block, part, subset, &npal); err < bestErr {
			bestErr, q0, q1, pb0, pb1, pal = err, nq0, nq1, npb0, npb1, npal
			if bestErr == 0 {
				break
			}

			continue
		}

		break
	}

	return q0, q1, pb0, pb1
}

// bc7Mode3Assign assigns every texel its nearest 2-bit index within its subset.
func bc7Mode3Assign(block *[16]rgba8, part *[16]uint8, pal *[2][4]rgba8) [16]uint8 {
	var idx [16]uint8
	for i := range 16 {
		s := part[i] & 0x03
		best := 0
		bestErr := bc7RGBErr(block[i], pal[s][0])
		for k := 1; k < 4; k++ {
			if e := bc7RGBErr(block[i], pal[s][k]); e < bestErr {
				bestErr, best = e, k
			}
		}
		// #nosec G115 -- best is in [0,3].
		idx[i] = uint8(best)
	}

	return idx
}

// encodeBC7Mode3 encodes a fully opaque block as BC7 mode 3,
// trying the top maxPartitions ranked partitions.
func encodeBC7Mode3(block [16]rgba8, maxPartitions int) ([16]byte, int, bool) {
	order := bc7Rank2Subset(&block)
	tries := min(maxPartitions, 64)

	var bestBytes [16]byte
	bestErr := 1 << 30
	found := false
	for t := range tries {
		b, err := bc7Mode3TryPartition(&block, order[t])
		if err < bestErr {
			bestErr, bestBytes, found = err, b, true
			if bestErr == 0 {
				break
			}
		}
	}
	return bestBytes, bestErr, found
}

// bc7Mode3TryPartition fits both subsets of one partition,
// resolves the anchor constraints, and returns the packed block with its total error.
//
//nolint:dupl // per-mode BC7 partition encoders are intentionally kept separate.
func bc7Mode3TryPartition(block *[16]rgba8, p int) ([16]byte, int) {
	part := &bc7PartitionSets[0][p]

	var q [4]rgba8
	var pbit [4]uint8
	var pal [2][4]rgba8
	for s := range 2 {
		// #nosec G115 -- s is 0 or 1.
		e0, e1, pb0, pb1 := bc7Mode3FitSubset(block, part, uint8(s))
		q[s*2], q[s*2+1] = e0, e1
		pbit[s*2], pbit[s*2+1] = pb0, pb1
		pal[s] = bc7Mode3Palette(bc7ExpandMode3(e0, pb0), bc7ExpandMode3(e1, pb1))
	}

	idx := bc7Mode3Assign(block, part, &pal)

	anchors := [2]int{0, bc7Mode1Anchor1(p)}
	for s := range 2 {
		if idx[anchors[s]]&0x02 != 0 {
			q[s*2], q[s*2+1] = q[s*2+1], q[s*2]
			pbit[s*2], pbit[s*2+1] = pbit[s*2+1], pbit[s*2]
			pal[s] = bc7Mode3Palette(bc7ExpandMode3(q[s*2], pbit[s*2]), bc7ExpandMode3(q[s*2+1], pbit[s*2+1]))
			idx = bc7Mode3Assign(block, part, &pal)
		}
	}

	total := 0
	for i := range 16 {
		total += bc7SSE(block[i], pal[part[i]&0x03][idx[i]])
	}

	return bc7PackMode3(&q, &pbit, &idx, p), total
}

// bc7PackMode3 serializes a mode 3 block: mode bits, partition id,
// RGB endpoints (7 bits each, channel-major over four endpoints),
// four per-endpoint P-bits, and 2-bit indices (anchored texels use one fewer bit).
func bc7PackMode3(q *[4]rgba8, pbit *[4]uint8, idx *[16]uint8, p int) [16]byte {
	var w bptcWriter
	w.put(1<<3, 4) // mode 3
	// #nosec G115 -- p is in [0,63].
	w.put(uint32(p), 6)

	for _, ch := range [3]func(rgba8) uint8{
		func(c rgba8) uint8 { return c.r },
		func(c rgba8) uint8 { return c.g },
		func(c rgba8) uint8 { return c.b },
	} {
		for e := range 4 {
			w.put(uint32(ch(q[e])), 7)
		}
	}

	for e := range 4 {
		w.put(uint32(pbit[e]), 1)
	}

	part := &bc7PartitionSets[0][p]
	for i := range 16 {
		bits := 2
		if part[i]&0x80 != 0 {
			bits = 1
		}
		w.put(uint32(idx[i]), bits)
	}

	return w.bytes()
}
