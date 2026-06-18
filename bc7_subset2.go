// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// Shared machinery for the two-subset BC7 partition modes:
// the RGB subset least-squares fit (used by modes 1 and 3)
// and the per-endpoint-P-bit partition driver (used by modes 3 and 7).
// Each mode supplies its own endpoint fit, palette, index assignment, and packing;
// the driver runs the common partition search and anchor fix-ups.

// bc7SubsetLSQ refits continuous RGB endpoints for the pixels
// of one subset from their current nearest-palette assignment.
// ok is false on a degenerate weight distribution (all assigned texels share one weight).
func bc7SubsetLSQ(block *[16]rgba8, part *[16]uint8, subset uint8, pal []rgba8, weights []int32) (rgba8, rgba8, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	// TODO(avo): per-texel nearest-index search + accumulation is the hot loop.
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < len(pal); k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

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

// bc72Subset captures one two-subset mode's operations.
// fit returns a subset's two endpoints and their P-bits;
// palette expands an endpoint pair into the 4-entry interpolation table;
// assign picks the nearest index per texel; pack serializes the block.
type bc72Subset struct {
	fit     func(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8, uint8, uint8)
	palette func(q0 rgba8, pb0 uint8, q1 rgba8, pb1 uint8) [4]rgba8
	assign  func(block *[16]rgba8, part *[16]uint8, pal *[2][4]rgba8) [16]uint8
	pack    func(q *[4]rgba8, pbit *[4]uint8, idx *[16]uint8, p int) [16]byte
}

// encode tries the top maxPartitions ranked two-subset partitions
// and keeps the lowest-error result.
func (m bc72Subset) encode(block [16]rgba8, maxPartitions int) ([16]byte, int, bool) {
	order := bc7Rank2Subset(&block)
	tries := min(maxPartitions, 64)

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

// tryPartition fits both subsets of one partition,
// resolves the per-subset anchor constraints
// (the anchor index MSB must be clear),
// and returns the packed block with its total error.
func (m bc72Subset) tryPartition(block *[16]rgba8, p int) ([16]byte, int) {
	part := &bc7PartitionSets[0][p]

	var q [4]rgba8
	var pbit [4]uint8
	var pal [2][4]rgba8
	for s := range 2 {
		// #nosec G115 -- s is 0 or 1.
		e0, e1, pb0, pb1 := m.fit(block, part, uint8(s))
		q[s*2], q[s*2+1] = e0, e1
		pbit[s*2], pbit[s*2+1] = pb0, pb1
		pal[s] = m.palette(e0, pb0, e1, pb1)
	}

	idx := m.assign(block, part, &pal)

	anchors := [2]int{0, bc7Mode1Anchor1(p)}
	for s := range 2 {
		if idx[anchors[s]]&0x02 != 0 {
			q[s*2], q[s*2+1] = q[s*2+1], q[s*2]
			pbit[s*2], pbit[s*2+1] = pbit[s*2+1], pbit[s*2]
			pal[s] = m.palette(q[s*2], pbit[s*2], q[s*2+1], pbit[s*2+1])
			idx = m.assign(block, part, &pal)
		}
	}

	total := 0
	for i := range 16 {
		total += bc7SSE(block[i], pal[part[i]&0x03][idx[i]])
	}

	return m.pack(&q, &pbit, &idx, p), total
}
