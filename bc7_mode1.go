// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sort"

// BC7 mode 1: two subsets, RGB only (alpha is implicitly 255),
// 6-bit endpoints with one shared P-bit per subset,
// and 3-bit indices over 64 partition shapes.
// It is the main quality win for opaque blocks that span two color regions,
// which a single subset (mode 6) cannot fit.
// Candidate partitions are ranked by a cheap within-subset variance
// and only the top few are fully fitted.

// bc7Expand7 expands a 6-bit stored value plus the shared P-bit to 8 bits,
// matching the decoder's 7-bit (incl. P-bit) unquantize.
func bc7Expand7(stored, pbit uint8) uint8 {
	v := stored<<1 | pbit // 7 significant bits
	v <<= 1               // shift the MSB to bit 7 (8 - 7)
	return v | v>>7       // replicate the MSB into the freed LSB
}

// bc7Quant7 rounds an 8-bit channel
// to the nearest 6-bit value reachable with the given P-bit.
func bc7Quant7(target, pbit uint8) uint8 {
	seed := (int(target)*127/255 - int(pbit)) >> 1
	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 2; ds++ {
		s := seed + ds
		if s < 0 || s > 63 {
			continue
		}

		// #nosec G115 -- s is in [0,63].
		d := int(bc7Expand7(uint8(s), pbit)) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr = d
			best = s
		}
	}

	// #nosec G115 -- best is in [0,63].
	return uint8(best)
}

// bc7QuantSubset1 quantizes a subset's two RGB endpoints with a single shared P-bit,
// chosen to minimize the combined round-trip error.
func bc7QuantSubset1(c0, c1 rgba8) (rgba8, rgba8, uint8) {
	var best0, best1 rgba8
	var bestPBit uint8
	bestErr := -1

	for pb := range 2 {
		// #nosec G115 -- pb is 0 or 1.
		pbit := uint8(pb)
		q0 := rgba8{r: bc7Quant7(c0.r, pbit), g: bc7Quant7(c0.g, pbit), b: bc7Quant7(c0.b, pbit)}
		q1 := rgba8{r: bc7Quant7(c1.r, pbit), g: bc7Quant7(c1.g, pbit), b: bc7Quant7(c1.b, pbit)}
		e := bc7RGBErr(bc7ExpandRGB7(q0, pbit), c0) + bc7RGBErr(bc7ExpandRGB7(q1, pbit), c1)
		if bestErr < 0 || e < bestErr {
			bestErr, best0, best1, bestPBit = e, q0, q1, pbit
		}
	}

	return best0, best1, bestPBit
}

// bc7ExpandRGB7 expands a quantized RGB endpoint to 8 bits (alpha is 255).
func bc7ExpandRGB7(q rgba8, pbit uint8) rgba8 {
	return rgba8{
		r: bc7Expand7(q.r, pbit),
		g: bc7Expand7(q.g, pbit),
		b: bc7Expand7(q.b, pbit),
		a: 255,
	}
}

// bc7RGBErr returns the squared RGB error between two pixels.
func bc7RGBErr(p, q rgba8) int {
	dr := int(p.r) - int(q.r)
	dg := int(p.g) - int(q.g)
	db := int(p.b) - int(q.b)
	return dr*dr + dg*dg + db*db
}

// bc7Mode1Palette builds the 8 interpolated RGB colors for a subset's endpoints.
func bc7Mode1Palette(e0, e1 rgba8) [8]rgba8 {
	var pal [8]rgba8
	w := bc7Weight3[:]
	for i := range 8 {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), w, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), w, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), w, i),
			a: 255,
		}
	}

	return pal
}

// bc7Mode1FitSubset fits the RGB endpoints of one subset:
// a max-distance seed followed by iterated least-squares refinement,
// quantized to mode 1 precision.
func bc7Mode1FitSubset(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8, uint8) {
	c0, c1 := bc7SubsetMaxDist(block, part, subset)
	q0, q1, pbit := bc7QuantSubset1(c0, c1)
	pal := bc7Mode1Palette(bc7ExpandRGB7(q0, pbit), bc7ExpandRGB7(q1, pbit))
	bestErr := bc7SubsetError(block, part, subset, &pal)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := bc7SubsetLSQ(block, part, subset, &pal)
		if !ok {
			break
		}

		nq0, nq1, npbit := bc7QuantSubset1(nc0, nc1)
		npal := bc7Mode1Palette(bc7ExpandRGB7(nq0, npbit), bc7ExpandRGB7(nq1, npbit))
		if err := bc7SubsetError(block, part, subset, &npal); err < bestErr {
			bestErr, q0, q1, pbit, pal = err, nq0, nq1, npbit, npal
			if bestErr == 0 {
				break
			}
			continue
		}

		break
	}

	return q0, q1, pbit
}

// bc7SubsetMaxDist returns the two most distant RGB pixels of a subset.
func bc7SubsetMaxDist(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8) {
	var first rgba8
	haveFirst := false
	bi, bj, bestD := -1, -1, -1
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		if !haveFirst {
			first = block[i]
			haveFirst = true
		}
		for j := i + 1; j < 16; j++ {
			if part[j]&0x03 != subset {
				continue
			}
			if d := bc7RGBErr(block[i], block[j]); d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}

	if bi < 0 {
		return first, first // single-pixel subset
	}

	return block[bi], block[bj]
}

// bc7SubsetError assigns each subset pixel its nearest palette entry
// and returns the total RGB error.
func bc7SubsetError(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[8]rgba8) int {
	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < 8; k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr = e
			}
		}
		total += bestErr
	}

	return total
}

// bc7SubsetLSQ refits continuous RGB endpoints for a subset from its current
// nearest-index assignment. ok is false on a degenerate weight distribution.
func bc7SubsetLSQ(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[8]rgba8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	// TODO(avo): per-texel nearest-index search + accumulation is the hot loop.
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < 8; k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

		b := int(bc7Weight3[idx])
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

// bc7Mode1Anchor1 returns the texel index of subset 1's anchor
// for a partition (the entry tagged 0x80 with subset id 1).
func bc7Mode1Anchor1(p int) int {
	part := &bc7PartitionSets[0][p]
	for i := range 16 {
		if part[i] == 0x80|1 {
			return i
		}
	}

	return 0
}

// bc7Mode1Assign assigns every texel its nearest index within its subset.
func bc7Mode1Assign(block *[16]rgba8, part *[16]uint8, pal *[2][8]rgba8) [16]uint8 {
	var idx [16]uint8
	for i := range 16 {
		s := part[i] & 0x03
		best := 0
		bestErr := bc7RGBErr(block[i], pal[s][0])
		for k := 1; k < 8; k++ {
			if e := bc7RGBErr(block[i], pal[s][k]); e < bestErr {
				bestErr, best = e, k
			}
		}
		// #nosec G115 -- best is in [0,7].
		idx[i] = uint8(best)
	}

	return idx
}

// bc7Rank2Subset orders the 64 two-subset partitions best-first by within-subset
// variance (lower means cleaner separation), ties broken by partition index.
func bc7Rank2Subset(block *[16]rgba8) [64]int {
	var score [64]int
	for p := range 64 {
		part := &bc7PartitionSets[0][p]
		var sum [2][4]int
		var cnt [2]int
		for i := range 16 {
			s := part[i] & 0x03
			sum[s][0] += int(block[i].r)
			sum[s][1] += int(block[i].g)
			sum[s][2] += int(block[i].b)
			sum[s][3] += int(block[i].a)
			cnt[s]++
		}
		var mean [2][4]int
		for s := range 2 {
			if cnt[s] > 0 {
				for c := range 4 {
					mean[s][c] = sum[s][c] / cnt[s]
				}
			}
		}
		total := 0
		for i := range 16 {
			s := part[i] & 0x03
			px := [4]int{int(block[i].r), int(block[i].g), int(block[i].b), int(block[i].a)}
			for c := range 4 {
				d := px[c] - mean[s][c]
				total += d * d
			}
		}
		score[p] = total
	}

	// Pack (score, index) into one key so a plain int sort orders by score and
	// then by partition index, deterministically and without a closure alloc.
	var keys [64]int
	for p := range 64 {
		keys[p] = score[p]<<6 | p
	}
	sort.Ints(keys[:])

	var order [64]int
	for i := range 64 {
		order[i] = keys[i] & 0x3F
	}

	return order
}

// encodeBC7Mode1 encodes a fully opaque block as BC7 mode 1, trying the top
// maxPartitions ranked partitions and keeping the lowest-error result.
func encodeBC7Mode1(block [16]rgba8, maxPartitions int) ([16]byte, int, bool) {
	order := bc7Rank2Subset(&block)
	tries := min(maxPartitions, 64)

	var bestBytes [16]byte
	bestErr := 1 << 30
	found := false
	for t := range tries {
		b, err := bc7Mode1TryPartition(&block, order[t])
		if err < bestErr {
			bestErr, bestBytes, found = err, b, true
			if bestErr == 0 {
				break
			}
		}
	}

	return bestBytes, bestErr, found
}

// bc7Mode1TryPartition fits both subsets of one partition, resolves the anchor
// constraints, and returns the packed block with its total error.
func bc7Mode1TryPartition(block *[16]rgba8, p int) ([16]byte, int) {
	part := &bc7PartitionSets[0][p]

	var q [4]rgba8
	var pbit [2]uint8
	var pal [2][8]rgba8
	for s := range 2 {
		// #nosec G115 -- s is 0 or 1.
		e0, e1, pb := bc7Mode1FitSubset(block, part, uint8(s))
		q[s*2], q[s*2+1], pbit[s] = e0, e1, pb
		pal[s] = bc7Mode1Palette(bc7ExpandRGB7(e0, pb), bc7ExpandRGB7(e1, pb))
	}

	idx := bc7Mode1Assign(block, part, &pal)

	// Each subset's anchor index must have its MSB clear (3-bit -> 2-bit).
	// If not, swap that subset's endpoints (inverting its indices) and re-assign.
	anchors := [2]int{0, bc7Mode1Anchor1(p)}
	for s := range 2 {
		if idx[anchors[s]]&0x04 != 0 {
			q[s*2], q[s*2+1] = q[s*2+1], q[s*2]
			pal[s] = bc7Mode1Palette(bc7ExpandRGB7(q[s*2], pbit[s]), bc7ExpandRGB7(q[s*2+1], pbit[s]))
			idx = bc7Mode1Assign(block, part, &pal)
		}
	}

	total := 0
	for i := range 16 {
		total += bc7SSE(block[i], pal[part[i]&0x03][idx[i]])
	}

	return bc7PackMode1(&q, &pbit, &idx, p), total
}

// bc7PackMode1 serializes a mode 1 block: the two mode bits, the 6-bit partition id,
// RGB endpoints (6 bits each, channel-major over the four endpoints),
// the two shared P-bits, and the indices (anchored texels use one fewer bit).
func bc7PackMode1(q *[4]rgba8, pbit *[2]uint8, idx *[16]uint8, p int) [16]byte {
	var w bptcWriter
	w.put(1<<1, 2) // mode 1: one zero bit then the set bit
	// #nosec G115 -- p is in [0,63].
	w.put(uint32(p), 6)

	for _, ch := range [3]func(rgba8) uint8{
		func(c rgba8) uint8 { return c.r },
		func(c rgba8) uint8 { return c.g },
		func(c rgba8) uint8 { return c.b },
	} {
		for e := range 4 {
			w.put(uint32(ch(q[e])), 6)
		}
	}

	w.put(uint32(pbit[0]), 1)
	w.put(uint32(pbit[1]), 1)

	part := &bc7PartitionSets[0][p]
	for i := range 16 {
		bits := 3
		if part[i]&0x80 != 0 {
			bits = 2
		}
		w.put(uint32(idx[i]), bits)
	}

	return w.bytes()
}
