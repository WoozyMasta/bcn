// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sort"

// BC7 three-subset opaque RGB modes 0 and 2.
// They split a block into three color regions (the partition tables in bc7PartitionSets[1]),
// which neither a single subset nor a two-subset partition can fit.
// The two modes share this framework and differ only in precision and budget:
//
//	mode 0: 4-bit endpoints + per-endpoint P-bit, 3-bit indices, 16 partitions
//	mode 2: 5-bit endpoints (no P-bit),           2-bit indices, 64 partitions
//
// Both reach 5-bit endpoint precision, so a single expand/quantize pair serves.

// bc73Mode holds the per-mode parameters of a three-subset opaque encoder.
type bc73Mode struct {
	weights   []int32
	modeVal   int
	partBits  int
	numParts  int
	colorBits int
	indexBits int
	nLevels   int
	hasPBit   bool
}

var (
	bc7Mode0Params = bc73Mode{
		modeVal: 0, partBits: 4, numParts: 16,
		colorBits: 4, hasPBit: true, indexBits: 3, weights: bc7Weight3[:], nLevels: 8,
	}
	bc7Mode2Params = bc73Mode{
		modeVal: 2, partBits: 6, numParts: 64,
		colorBits: 5, hasPBit: false, indexBits: 2, weights: bc7Weight2[:], nLevels: 4,
	}
)

var (
	// bc7Mode0StoreTable stores the exact mode 0 scalar channel quantization
	// result for each 8-bit input and endpoint P-bit.
	bc7Mode0StoreTable = bc7MakeMode0StoreTable()

	// bc7Mode2StoreTable stores the exact mode 2 scalar channel quantization
	// result for each 8-bit input.
	bc7Mode2StoreTable = bc7MakeMode2StoreTable()
)

// bc7MakeMode0StoreTable builds the mode 0 channel quantization table.
func bc7MakeMode0StoreTable() [2][256]uint8 {
	var table [2][256]uint8
	for pbit := range 2 {
		for target := range 256 {
			// #nosec G115 -- pbit is 0/1 and target is in [0,255].
			table[pbit][target] = bc7Mode0StoreSlow(uint8(target), uint8(pbit))
		}
	}

	return table
}

// bc7MakeMode2StoreTable builds the mode 2 channel quantization table.
func bc7MakeMode2StoreTable() [256]uint8 {
	var table [256]uint8
	for target := range 256 {
		// #nosec G115 -- target is in [0,255].
		table[target] = bc7Mode2StoreSlow(uint8(target))
	}

	return table
}

// bc7Expand5 expands a 5-bit raw value (4 bits plus a P-bit, or a bare 5-bit value) to 8 bits,
// matching the decoder's 5-bit-precision unquantize.
func bc7Expand5(raw uint8) uint8 {
	v := raw << 3
	return v | v>>5
}

// store rounds an 8-bit channel to this mode's stored width for the given P-bit.
func (m bc73Mode) store(target, pbit uint8) uint8 {
	if m.hasPBit {
		return bc7Mode0StoreTable[pbit&1][target]
	}

	return bc7Mode2StoreTable[target]
}

// bc7Mode0StoreSlow computes the mode 0 channel quantization directly.
func bc7Mode0StoreSlow(target, pbit uint8) uint8 {
	const maxS = 15
	rawSeed := int(target) * 31 / 255
	seed := (rawSeed - int(pbit)) >> 1

	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 2; ds++ {
		s := seed + ds
		if s < 0 || s > maxS {
			continue
		}

		// #nosec G115 -- s <= 15, pbit is 0/1.
		raw := uint8(s<<1) | pbit
		d := int(bc7Expand5(raw)) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr, best = d, s
		}
	}

	// #nosec G115 -- best <= 31.
	return uint8(best)
}

// bc7Mode2StoreSlow computes the mode 2 channel quantization directly.
func bc7Mode2StoreSlow(target uint8) uint8 {
	const maxS = 31
	seed := int(target) * 31 / 255

	best, bestErr := 0, 1<<30
	for ds := -1; ds <= 2; ds++ {
		s := seed + ds
		if s < 0 || s > maxS {
			continue
		}

		// #nosec G115 -- s <= 31.
		d := int(bc7Expand5(uint8(s))) - int(target)
		if d < 0 {
			d = -d
		}
		if d < bestErr {
			bestErr, best = d, s
		}
	}

	// #nosec G115 -- best <= 31.
	return uint8(best)
}

// expand reconstructs an 8-bit RGB endpoint (alpha forced to 255).
func (m bc73Mode) expand(q rgba8, pbit uint8) rgba8 {
	raw := func(v uint8) uint8 {
		if m.hasPBit {
			return v<<1 | pbit
		}
		return v
	}

	return rgba8{r: bc7Expand5(raw(q.r)), g: bc7Expand5(raw(q.g)), b: bc7Expand5(raw(q.b)), a: 255}
}

// quant quantizes one RGB endpoint. For P-bit modes the single P-bit
// (shared by the endpoint's channels) is chosen to minimize the round-trip error.
func (m bc73Mode) quant(c rgba8) (rgba8, uint8) {
	if !m.hasPBit {
		return rgba8{r: m.store(c.r, 0), g: m.store(c.g, 0), b: m.store(c.b, 0)}, 0
	}

	var best rgba8
	var bestPBit uint8
	bestErr := -1
	for pb := range 2 {
		// #nosec G115 -- pb is 0 or 1.
		pbit := uint8(pb)
		q := rgba8{r: m.store(c.r, pbit), g: m.store(c.g, pbit), b: m.store(c.b, pbit)}
		if e := bc7RGBErr(m.expand(q, pbit), c); bestErr < 0 || e < bestErr {
			bestErr, best, bestPBit = e, q, pbit
		}
	}

	return best, bestPBit
}

// palette builds this mode's interpolated RGB entries for an endpoint pair.
func (m bc73Mode) palette(e0, e1 rgba8) [8]rgba8 {
	var pal [8]rgba8
	for i := range m.nLevels {
		pal[i] = rgba8{
			r: bc7Interpolate(int32(e0.r), int32(e1.r), m.weights, i),
			g: bc7Interpolate(int32(e0.g), int32(e1.g), m.weights, i),
			b: bc7Interpolate(int32(e0.b), int32(e1.b), m.weights, i),
			a: 255,
		}
	}

	return pal
}

// nearest returns the nearest palette index (RGB) and its error.
func (m bc73Mode) nearest(px rgba8, pal *[8]rgba8) (int, int) {
	if m.nLevels == 4 {
		return bc7NearestRGB4(px, pal)
	}

	return bc7NearestRGB8(px, pal)
}

// bc7NearestRGB4 returns the nearest entry in a 4-color RGB palette.
func bc7NearestRGB4(px rgba8, pal *[8]rgba8) (int, int) {
	best, bestErr := 0, bc7RGBErr(px, pal[0])
	if e := bc7RGBErr(px, pal[1]); e < bestErr {
		best, bestErr = 1, e
	}
	if e := bc7RGBErr(px, pal[2]); e < bestErr {
		best, bestErr = 2, e
	}
	if e := bc7RGBErr(px, pal[3]); e < bestErr {
		best, bestErr = 3, e
	}

	return best, bestErr
}

// bc7NearestRGB8 returns the nearest entry in an 8-color RGB palette.
func bc7NearestRGB8(px rgba8, pal *[8]rgba8) (int, int) {
	best, bestErr := 0, bc7RGBErr(px, pal[0])
	if e := bc7RGBErr(px, pal[1]); e < bestErr {
		best, bestErr = 1, e
	}
	if e := bc7RGBErr(px, pal[2]); e < bestErr {
		best, bestErr = 2, e
	}
	if e := bc7RGBErr(px, pal[3]); e < bestErr {
		best, bestErr = 3, e
	}
	if e := bc7RGBErr(px, pal[4]); e < bestErr {
		best, bestErr = 4, e
	}
	if e := bc7RGBErr(px, pal[5]); e < bestErr {
		best, bestErr = 5, e
	}
	if e := bc7RGBErr(px, pal[6]); e < bestErr {
		best, bestErr = 6, e
	}
	if e := bc7RGBErr(px, pal[7]); e < bestErr {
		best, bestErr = 7, e
	}

	return best, bestErr
}

// subsetError sums the nearest-entry RGB error over the pixels of one subset.
func (m bc73Mode) subsetError(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[8]rgba8) int {
	if _, total, ok := bc7SubsetEvalASM(block, part, subset, pal[:m.nLevels], nil); ok {
		return total
	}

	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		_, e := m.nearest(block[i], pal)
		total += e
	}

	return total
}

// subsetLSQ refits continuous RGB endpoints
// for one subset from its current nearest-index assignment.
func (m bc73Mode) subsetLSQ(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[8]rgba8) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	if sums, _, ok := bc7SubsetEvalASM(block, part, subset, pal[:m.nLevels], m.weights); ok {
		saa, sbb, sab = sums.saa, sums.sbb, sums.sab
		sap = [3]int{sums.sapR, sums.sapG, sums.sapB}
		sbp = [3]int{sums.sbpR, sums.sbpG, sums.sbpB}
	} else {
		for i := range 16 {
			if part[i]&0x03 != subset {
				continue
			}
			idx, _ := m.nearest(block[i], pal)
			b := int(m.weights[idx])
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

// fitSubset fits one subset's RGB endpoints (max-distance seed + least squares).
func (m bc73Mode) fitSubset(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8, uint8, uint8) {
	c0, c1 := bc7SubsetMaxDist(block, part, subset)
	q0, pb0 := m.quant(c0)
	q1, pb1 := m.quant(c1)
	pal := m.palette(m.expand(q0, pb0), m.expand(q1, pb1))
	bestErr := m.subsetError(block, part, subset, &pal)

	const maxIters = 8
	for range maxIters {
		nc0, nc1, ok := m.subsetLSQ(block, part, subset, &pal)
		if !ok {
			break
		}

		nq0, npb0 := m.quant(nc0)
		nq1, npb1 := m.quant(nc1)
		npal := m.palette(m.expand(nq0, npb0), m.expand(nq1, npb1))
		if err := m.subsetError(block, part, subset, &npal); err < bestErr {
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

// assign assigns every texel its nearest index within its subset's palette.
func (m bc73Mode) assign(block *[16]rgba8, part *[16]uint8, pal *[3][8]rgba8) [16]uint8 {
	var idx [16]uint8
	for i := range 16 {
		s := part[i] & 0x03
		best, _ := m.nearest(block[i], &pal[s])
		// #nosec G115 -- best < nLevels <= 8.
		idx[i] = uint8(best)
	}

	return idx
}

// bc73Anchors returns the anchor texel of each subset: subset 0 is texel 0,
// the others are the entries tagged 0x80 with subset id 1 and 2.
func bc73Anchors(p int) [3]int {
	a := [3]int{0, 0, 0}
	part := &bc7PartitionSets[1][p]
	for i := range 16 {
		switch part[i] {
		case 0x80 | 1:
			a[1] = i
		case 0x80 | 2:
			a[2] = i
		}
	}

	return a
}

// bc7Rank3SubsetN orders only the first maxPartitions three-subset partitions needed by the caller.
// The first N entries match a full numParts sort,
// while the unused tail is left unspecified to avoid unnecessary hot-path work.
func bc7Rank3SubsetN(block *[16]rgba8, numParts, maxPartitions int) [64]int {
	numParts = min(max(numParts, 0), 64)
	limit := min(max(maxPartitions, 0), numParts)
	if limit == 0 {
		return [64]int{}
	}

	var scores [64]int
	for p := range numParts {
		part := &bc7PartitionSets[1][p]
		var sum [3][3]int
		var sumSq [3][3]int
		var cnt [3]int
		for i := range 16 {
			s := part[i] & 0x03
			r := int(block[i].r)
			g := int(block[i].g)
			b := int(block[i].b)
			sum[s][0] += r
			sum[s][1] += g
			sum[s][2] += b
			sumSq[s][0] += r * r
			sumSq[s][1] += g * g
			sumSq[s][2] += b * b
			cnt[s]++
		}

		total := 0
		for s := range 3 {
			if cnt[s] > 0 {
				for c := range 3 {
					// Match the old two-pass score exactly: mean is integer
					// truncated toward zero, then sum((x - mean)^2).
					mean := sum[s][c] / cnt[s]
					total += sumSq[s][c] - 2*mean*sum[s][c] + cnt[s]*mean*mean
				}
			}
		}

		scores[p] = total
	}

	var keys [64]int
	for p := range numParts {
		keys[p] = scores[p]<<6 | p
	}
	if limit == numParts {
		sort.Ints(keys[:numParts])
	} else {
		sortTopKeys(keys[:numParts], limit)
	}

	var order [64]int
	for i := range limit {
		order[i] = keys[i] & 0x3F
	}

	return order
}

// encodeBC7Mode02 encodes a fully opaque block with one three-subset mode,
// trying the top maxPartitions ranked partitions.
func encodeBC7Mode02(m bc73Mode, block [16]rgba8, maxPartitions int) ([16]byte, int, bool) {
	tries := min(maxPartitions, m.numParts)
	order := bc7Rank3SubsetN(&block, m.numParts, tries)

	var bestBytes [16]byte
	bestErr := 1 << 30
	found := false
	for t := range tries {
		b, err := m.tryPartition(&block, order[t])
		if err < bestErr {
			bestErr, bestBytes, found = err, b, true
			if bestErr == 0 {
				break
			}
		}
	}

	return bestBytes, bestErr, found
}

// tryPartition fits all three subsets of one partition,
// resolves the per-subset anchor constraints,
// and returns the packed block with its total error.
func (m bc73Mode) tryPartition(block *[16]rgba8, p int) ([16]byte, int) {
	part := &bc7PartitionSets[1][p]

	var q [6]rgba8
	var pbit [6]uint8
	var pal [3][8]rgba8
	for s := range 3 {
		// #nosec G115 -- s is in [0,2].
		e0, e1, pb0, pb1 := m.fitSubset(block, part, uint8(s))
		q[s*2], q[s*2+1] = e0, e1
		pbit[s*2], pbit[s*2+1] = pb0, pb1
		pal[s] = m.palette(m.expand(e0, pb0), m.expand(e1, pb1))
	}

	idx := m.assign(block, part, &pal)

	// Each subset's anchor index must have its MSB clear.
	anchorMSB := uint8(1) << (m.indexBits - 1)
	anchors := bc73Anchors(p)
	for s := range 3 {
		if idx[anchors[s]]&anchorMSB != 0 {
			q[s*2], q[s*2+1] = q[s*2+1], q[s*2]
			pbit[s*2], pbit[s*2+1] = pbit[s*2+1], pbit[s*2]
			pal[s] = m.palette(m.expand(q[s*2], pbit[s*2]), m.expand(q[s*2+1], pbit[s*2+1]))
			idx = m.assign(block, part, &pal)
		}
	}

	total := 0
	for i := range 16 {
		total += bc7SSE(block[i], pal[part[i]&0x03][idx[i]])
	}

	return m.pack(&q, &pbit, &idx, p), total
}

// pack serializes a three-subset block: mode bits, partition id,
// RGB endpoints (channel-major over the six endpoints),
// optional per-endpoint P-bits, and the indices (anchored texels use one fewer bit).
func (m bc73Mode) pack(q *[6]rgba8, pbit *[6]uint8, idx *[16]uint8, p int) [16]byte {
	var w bptcWriter
	// #nosec G115 -- modeVal is 0 or 2.
	w.put(uint32(1<<m.modeVal), m.modeVal+1)
	// #nosec G115 -- p < numParts.
	w.put(uint32(p), m.partBits)

	for _, ch := range [3]func(rgba8) uint8{
		func(c rgba8) uint8 { return c.r },
		func(c rgba8) uint8 { return c.g },
		func(c rgba8) uint8 { return c.b },
	} {
		for e := range 6 {
			w.put(uint32(ch(q[e])), m.colorBits)
		}
	}

	if m.hasPBit {
		for e := range 6 {
			w.put(uint32(pbit[e]), 1)
		}
	}

	part := &bc7PartitionSets[1][p]
	for i := range 16 {
		bits := m.indexBits
		if part[i]&0x80 != 0 {
			bits--
		}
		w.put(uint32(idx[i]), bits)
	}

	return w.bytes()
}
