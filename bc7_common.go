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

// bc7Rank2SubsetN orders only the first maxPartitions two-subset partitions
// needed by the caller. The first N entries match a full 64-partition sort,
// but the unsorted tail is intentionally unspecified.
func bc7Rank2SubsetN(block *[16]rgba8, maxPartitions int) [64]int {
	limit := min(max(maxPartitions, 0), 64)
	if limit == 0 {
		return [64]int{}
	}

	var score [64]int
	var keys [64]int
	for p := range 64 {
		part := &bc7PartitionSets[0][p]
		var sum [2][4]int
		var sumSq [2][4]int
		var cnt [2]int
		for i := range 16 {
			s := part[i] & 0x03
			r := int(block[i].r)
			g := int(block[i].g)
			b := int(block[i].b)
			a := int(block[i].a)
			sum[s][0] += r
			sum[s][1] += g
			sum[s][2] += b
			sum[s][3] += a
			sumSq[s][0] += r * r
			sumSq[s][1] += g * g
			sumSq[s][2] += b * b
			sumSq[s][3] += a * a
			cnt[s]++
		}

		total := 0
		for s := range 2 {
			if cnt[s] > 0 {
				for c := range 4 {
					// Match the old two-pass score exactly: mean is integer
					// truncated toward zero, then sum((x - mean)^2).
					mean := sum[s][c] / cnt[s]
					total += sumSq[s][c] - 2*mean*sum[s][c] + cnt[s]*mean*mean
				}
			}
		}

		score[p] = total
	}

	for p := range 64 {
		keys[p] = score[p]<<6 | p
	}
	if limit == 64 {
		sort.Ints(keys[:])
	} else {
		sortTopKeys(keys[:], limit)
	}

	var order [64]int
	for i := range limit {
		order[i] = keys[i] & 0x3F
	}

	return order
}

// sortTopKeys sorts only the smallest limit keys in-place.
// It preserves the same ordering as sort.Ints(keys) for keys[:limit],
// including tie-breaking already encoded into the key value.
func sortTopKeys(keys []int, limit int) {
	if limit <= 0 {
		return
	}

	for i := range limit {
		minPos := i
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[minPos] {
				minPos = j
			}
		}

		keys[i], keys[minPos] = keys[minPos], keys[i]
	}
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
