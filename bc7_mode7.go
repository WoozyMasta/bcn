// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 mode 7: two subsets, RGBA, 5-bit endpoints plus a per-endpoint P-bit (6-bit precision),
// and 2-bit indices over the 64 two-subset partitions.
// It covers alpha-bearing blocks that also span two color/alpha regions,
// which the single subset of mode 5 cannot fit.
// Structure mirrors mode 1 (partition search + per-subset fit)
// but carries alpha and uses a P-bit per endpoint.

// bc7Expand6 expands a 6-bit raw value (5 bits plus a P-bit) to 8 bits,
// matching the decoder's unquantize (left-align the MSB, replicate downward).
func bc7Expand6(raw uint8) uint8 {
	v := raw << 2
	return v | v>>6
}

// bc7Store5 rounds an 8-bit channel to the nearest 5-bit value
// reachable with the given P-bit (precision 6).
func bc7Store5(target, pbit uint8) uint8 {
	seed := (int(target)*63/255 - int(pbit)) >> 1
	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 2; ds++ {
		s := seed + ds
		if s < 0 || s > 31 {
			continue
		}

		// #nosec G115 -- s is in [0,31], pbit is 0/1.
		raw := uint8(s<<1) | pbit
		d := int(bc7Expand6(raw)) - int(target)
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

// bc7ExpandMode7 expands a quantized RGBA endpoint to 8 bits per channel.
func bc7ExpandMode7(q rgba8, pbit uint8) rgba8 {
	return rgba8{
		r: bc7Expand6(q.r<<1 | pbit),
		g: bc7Expand6(q.g<<1 | pbit),
		b: bc7Expand6(q.b<<1 | pbit),
		a: bc7Expand6(q.a<<1 | pbit),
	}
}

// bc7QuantMode7 quantizes one RGBA endpoint to 5 bits per channel,
// choosing the per-endpoint P-bit (shared across the four channels)
// that minimizes the round-trip error.
func bc7QuantMode7(c rgba8) (rgba8, uint8) {
	var best rgba8
	var bestPBit uint8
	bestErr := -1
	for pb := range 2 {
		// #nosec G115 -- pb is 0 or 1.
		pbit := uint8(pb)
		q := rgba8{
			r: bc7Store5(c.r, pbit),
			g: bc7Store5(c.g, pbit),
			b: bc7Store5(c.b, pbit),
			a: bc7Store5(c.a, pbit),
		}
		if e := bc7SSE(c, bc7ExpandMode7(q, pbit)); bestErr < 0 || e < bestErr {
			bestErr, best, bestPBit = e, q, pbit
		}
	}

	return best, bestPBit
}

// bc7Mode7Palette builds the 4 interpolated RGBA colors for an endpoint pair.
func bc7Mode7Palette(e0, e1 rgba8) [4]rgba8 {
	var pal [4]rgba8
	w := bc7Weight2[:]
	for i := range 4 {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), w, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), w, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), w, i),
			a: bc7Interpolate(int32(e0.a), int32(e1.a), w, i),
		}
	}

	return pal
}

// bc7Mode7SubsetMaxDist returns the two most distant RGBA pixels of a subset.
func bc7Mode7SubsetMaxDist(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8) {
	var first rgba8
	haveFirst := false
	bi, bj, bestD := -1, -1, -1
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		if !haveFirst {
			first, haveFirst = block[i], true
		}
		for j := i + 1; j < 16; j++ {
			if part[j]&0x03 != subset {
				continue
			}
			if d := bc7SSE(block[i], block[j]); d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}

	if bi < 0 {
		return first, first
	}

	return block[bi], block[bj]
}

// bc7Mode7SubsetError sums the nearest-entry RGBA error over a subset.
func bc7Mode7SubsetError(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) int {
	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		bestErr := bc7SSE(block[i], pal[0])
		for k := 1; k < 4; k++ {
			if e := bc7SSE(block[i], pal[k]); e < bestErr {
				bestErr = e
			}
		}
		total += bestErr
	}

	return total
}

// bc7Mode7SubsetLSQ refits continuous RGBA endpoints
// for a subset from its current nearest 2-bit assignment.
func bc7Mode7SubsetLSQ(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [4]int

	// TODO(avo): per-texel nearest-index search + accumulation is the hot loop.
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7SSE(block[i], pal[0])
		for k := 1; k < 4; k++ {
			if e := bc7SSE(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

		b := int(bc7Weight2[idx])
		a := 64 - b
		saa += a * a
		sbb += b * b
		sab += a * b
		ch := [4]int{int(block[i].r), int(block[i].g), int(block[i].b), int(block[i].a)}
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

// bc7Mode7FitSubset fits one subset's RGBA endpoints with a max-distance seed
// and iterated least-squares refinement, quantized to mode 7 precision.
func bc7Mode7FitSubset(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8, uint8, uint8) {
	c0, c1 := bc7Mode7SubsetMaxDist(block, part, subset)
	q0, pb0 := bc7QuantMode7(c0)
	q1, pb1 := bc7QuantMode7(c1)
	pal := bc7Mode7Palette(bc7ExpandMode7(q0, pb0), bc7ExpandMode7(q1, pb1))
	bestErr := bc7Mode7SubsetError(block, part, subset, &pal)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := bc7Mode7SubsetLSQ(block, part, subset, &pal)
		if !ok {
			break
		}

		nq0, npb0 := bc7QuantMode7(nc0)
		nq1, npb1 := bc7QuantMode7(nc1)
		npal := bc7Mode7Palette(bc7ExpandMode7(nq0, npb0), bc7ExpandMode7(nq1, npb1))
		if err := bc7Mode7SubsetError(block, part, subset, &npal); err < bestErr {
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

// bc7Mode7Assign assigns every texel its nearest 2-bit index within its subset.
func bc7Mode7Assign(block *[16]rgba8, part *[16]uint8, pal *[2][4]rgba8) [16]uint8 {
	var idx [16]uint8
	for i := range 16 {
		s := part[i] & 0x03
		best := 0
		bestErr := bc7SSE(block[i], pal[s][0])
		for k := 1; k < 4; k++ {
			if e := bc7SSE(block[i], pal[s][k]); e < bestErr {
				bestErr, best = e, k
			}
		}
		// #nosec G115 -- best is in [0,3].
		idx[i] = uint8(best)
	}

	return idx
}

// encodeBC7Mode7 encodes a block as BC7 mode 7,
// trying the top maxPartitions ranked partitions
// and keeping the lowest-error result.
func encodeBC7Mode7(block [16]rgba8, maxPartitions int) ([16]byte, int, bool) {
	order := bc7Rank2Subset(&block)
	tries := min(maxPartitions, 64)

	var bestBytes [16]byte
	bestErr := 1 << 30
	found := false
	for t := range tries {
		b, err := bc7Mode7TryPartition(&block, order[t])
		if err < bestErr {
			bestErr, bestBytes, found = err, b, true
			if bestErr == 0 {
				break
			}
		}
	}

	return bestBytes, bestErr, found
}

// bc7Mode7TryPartition fits both subsets of one partition,
// resolves the anchor constraints,
// and returns the packed block with its total error.
//
//nolint:dupl // per-mode BC7 partition encoders are intentionally kept separate.
func bc7Mode7TryPartition(block *[16]rgba8, p int) ([16]byte, int) {
	part := &bc7PartitionSets[0][p]

	var q [4]rgba8
	var pbit [4]uint8
	var pal [2][4]rgba8
	for s := range 2 {
		// #nosec G115 -- s is 0 or 1.
		e0, e1, pb0, pb1 := bc7Mode7FitSubset(block, part, uint8(s))
		q[s*2], q[s*2+1] = e0, e1
		pbit[s*2], pbit[s*2+1] = pb0, pb1
		pal[s] = bc7Mode7Palette(bc7ExpandMode7(e0, pb0), bc7ExpandMode7(e1, pb1))
	}

	idx := bc7Mode7Assign(block, part, &pal)

	// Each subset's anchor index must have its MSB clear (2-bit -> 1-bit).
	anchors := [2]int{0, bc7Mode1Anchor1(p)}
	for s := range 2 {
		if idx[anchors[s]]&0x02 != 0 {
			q[s*2], q[s*2+1] = q[s*2+1], q[s*2]
			pbit[s*2], pbit[s*2+1] = pbit[s*2+1], pbit[s*2]
			pal[s] = bc7Mode7Palette(bc7ExpandMode7(q[s*2], pbit[s*2]), bc7ExpandMode7(q[s*2+1], pbit[s*2+1]))
			idx = bc7Mode7Assign(block, part, &pal)
		}
	}

	total := 0
	for i := range 16 {
		total += bc7SSE(block[i], pal[part[i]&0x03][idx[i]])
	}

	return bc7PackMode7(&q, &pbit, &idx, p), total
}

// bc7PackMode7 serializes a mode 7 block: mode bits, partition id,
// RGBA endpoints (5 bits each, channel-major over the four endpoints),
// four per-endpoint P-bits, and 2-bit indices (anchored texels use one fewer bit).
func bc7PackMode7(q *[4]rgba8, pbit *[4]uint8, idx *[16]uint8, p int) [16]byte {
	var w bptcWriter
	w.put(1<<7, 8) // mode 7
	// #nosec G115 -- p is in [0,63].
	w.put(uint32(p), 6)

	for _, ch := range [4]func(rgba8) uint8{
		func(c rgba8) uint8 { return c.r },
		func(c rgba8) uint8 { return c.g },
		func(c rgba8) uint8 { return c.b },
		func(c rgba8) uint8 { return c.a },
	} {
		for e := range 4 {
			w.put(uint32(ch(q[e])), 5)
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
