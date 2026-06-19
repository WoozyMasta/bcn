// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sort"

// Shared scalar primitives used across the BC7 encoder modes:
// error metrics, endpoint quantization, max-distance endpoint seeds, and partition ranking.
// These are the per-texel hot loops the planned AVO kernels will target.

// bc7SSE returns the squared RGBA error between two pixels.
func bc7SSE(p, q rgba8) int {
	dr := int(p.r) - int(q.r)
	dg := int(p.g) - int(q.g)
	db := int(p.b) - int(q.b)
	da := int(p.a) - int(q.a)

	return dr*dr + dg*dg + db*db + da*da
}

// bc7RGBErr returns the squared RGB error between two pixels (alpha ignored).
func bc7RGBErr(p, q rgba8) int {
	dr := int(p.r) - int(q.r)
	dg := int(p.g) - int(q.g)
	db := int(p.b) - int(q.b)

	return dr*dr + dg*dg + db*db
}

// bc7Store7 rounds an 8-bit channel to the nearest 7-bit value reachable with the given P-bit:
// value ~= store<<1 | pbit. Used by modes 3 and 6 (precision 8).
func bc7Store7(v uint8, pbit int) uint8 {
	s := min(max((int(v)-pbit+1)>>1, 0), 127)

	// #nosec G115 -- s is clamped to [0,127].
	return uint8(s)
}

// bc7MaxDistPair returns the two most distant texels by RGBA error,
// an endpoint seed for the single-subset RGBA modes.
func bc7MaxDistPair(block [16]rgba8) (rgba8, rgba8) {
	bi, bj, bestD := 0, 0, -1
	for i := range 16 {
		for j := i + 1; j < 16; j++ {
			if d := bc7SSE(block[i], block[j]); d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}

	return block[bi], block[bj]
}

// bc7MaxDistRGB returns the two most distant texels by RGB error,
// a seed for the single-subset color modes.
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

// bc7SubsetMaxDist returns the two most distant RGB pixels of one subset,
// a seed for the two- and three-subset partition modes.
func bc7SubsetMaxDist(block *[16]rgba8, part *[16]uint8, subset uint8) (rgba8, rgba8) {
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

// bc7Rank2Subset orders the 64 two-subset partitions best-first by within-subset variance
// (lower means cleaner separation), ties broken by partition index.
// Returns a fixed array (keeps the hot path allocation-free).
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

	// Pack (score, index) into one key so a plain int sort orders by score
	// and then by partition index, deterministically and without a closure alloc.
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

// bc7Mode1Anchor1 returns the texel index of subset 1's anchor for a two-subset partition
// (the entry tagged 0x80 with subset id 1).
func bc7Mode1Anchor1(p int) int {
	part := &bc7PartitionSets[0][p]
	for i := range 16 {
		if part[i] == 0x80|1 {
			return i
		}
	}

	return 0
}
