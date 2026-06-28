// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"math"
	"runtime"
	"sync"
)

// bc6hAnchorIndex2Sub holds the subset-1 anchor texel for each of the 32 BC6H partitions.
// Texel 0 is always the subset-0 anchor. The subset-1 anchor has its MSB implicit zero
// (the index stored uses one fewer bit at that position).
var bc6hAnchorIndex2Sub = [32]int{
	15, 15, 15, 15, 15, 15, 15, 15,
	15, 15, 15, 15, 15, 15, 15, 15,
	15, 2, 8, 2, 2, 8, 8, 15,
	2, 8, 2, 2, 8, 8, 2, 2,
}

// EncodeBC6H encodes a flat []uint16 RGB half-float image into BC6H-compressed data.
// src must have length width*height*3.
// signed selects BC6H_SF16 (true) or BC6H_UF16 (false).
func EncodeBC6H(src []uint16, width, height int, signed bool) ([]byte, error) {
	return encodeBlocksBC6H(src, width, height, signed, nil)
}

// EncodeBC6HWithOptions is EncodeBC6H with explicit encode options.
func EncodeBC6HWithOptions(src []uint16, width, height int, signed bool, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksBC6H(src, width, height, signed, opts)
}

// EncodeBC6HFloat32 encodes a flat []float32 RGB image into BC6H-compressed data.
// src must have length width*height*3.
func EncodeBC6HFloat32(src []float32, width, height int, signed bool) ([]byte, error) {
	return EncodeBC6HFloat32WithOptions(src, width, height, signed, nil)
}

// EncodeBC6HFloat32WithOptions is EncodeBC6HFloat32 with explicit encode options.
func EncodeBC6HFloat32WithOptions(src []float32, width, height int, signed bool, opts *EncodeOptions) ([]byte, error) {
	if len(src) != width*height*3 {
		return nil, ErrInvalidHDRSliceLength
	}

	h := make([]uint16, len(src))
	for i, v := range src {
		h[i] = float32ToFloat16(v)
	}

	return encodeBlocksBC6H(h, width, height, signed, opts)
}

// bc6hPrequantize converts a half-float bit pattern to the internal encoder int domain.
// Inverse of bc6hFinishUnquantize.
func bc6hPrequantize(h uint16, signed bool) int {
	if !signed {
		return (int(h) << 6) / 31
	}

	s := h & 0x8000
	mag := int(h & 0x7FFF)
	if mag == 0 {
		return 0
	}

	q := (mag << 5) / 31
	if s != 0 {
		return -q
	}

	return q
}

// bc6hQuantize compresses a pre-quantized value to the mode's endpoint bit width.
// Inverse of bc6hUnquantize.
func bc6hQuantize(val, bits int, signed bool) int {
	if !signed {
		if bits >= 15 {
			return val
		}
		if val == 0 {
			return 0
		}
		if val == 0xFFFF {
			return (1 << uint(bits)) - 1
		}
		return ((val << uint(bits)) - 0x8000) >> 16
	}

	if bits >= 16 {
		return val
	}
	if val == 0 {
		return 0
	}

	if val > 0 {
		if val == 0x7FFF {
			return (1 << uint(bits-1)) - 1
		}
		return ((val << uint(bits-1)) - 0x4000) >> 15
	}

	v := -val
	if v == 0x7FFF {
		return -((1 << uint(bits-1)) - 1)
	}

	return -(((v << uint(bits-1)) + 0x4000) >> 15)
}

// bc6hQuantizeEP quantizes all three channels of one endpoint.
func bc6hQuantizeEP(ep [3]int, bits int, signed bool) [3]int {
	return [3]int{
		bc6hQuantize(ep[0], bits, signed),
		bc6hQuantize(ep[1], bits, signed),
		bc6hQuantize(ep[2], bits, signed),
	}
}

// bc6hCreateDelta computes the delta value epT-ep0 masked to deltaBits.
// Sets *bad if the delta overflows the signed deltaBits range.
func bc6hCreateDelta(ep0, epT, deltaBits int, bad *bool) int {
	delta := epT - ep0
	mask := (1 << uint(deltaBits)) - 1
	half := 1 << uint(deltaBits-1)
	if delta >= 0 {
		if delta >= half {
			*bad = true
			delta = half - 1
		}
	} else {
		if -delta > half {
			*bad = true
			delta = half
		} else {
			delta &= mask
		}
	}

	return delta
}

// bc6hCreateDeltaRGB applies bc6hCreateDelta to all three channels.
func bc6hCreateDeltaRGB(ep0, epT [3]int, db [3]int, bad *bool) [3]int {
	return [3]int{
		bc6hCreateDelta(ep0[0], epT[0], db[0], bad),
		bc6hCreateDelta(ep0[1], epT[1], db[1], bad),
		bc6hCreateDelta(ep0[2], epT[2], db[2], bad),
	}
}

// bc6hBlockPreQ converts a 4x4 RGB half-float block to the pre-quantized int domain.
func bc6hBlockPreQ(block [48]uint16, signed bool) [16][3]int {
	var out [16][3]int
	for i := range 16 {
		out[i][0] = bc6hPrequantize(block[i*3+0], signed)
		out[i][1] = bc6hPrequantize(block[i*3+1], signed)
		out[i][2] = bc6hPrequantize(block[i*3+2], signed)
	}

	return out
}

// bc6hMaxDistPair returns the two texels in pq with the greatest L2 distance.
func bc6hMaxDistPair(pq *[16][3]int) ([3]int, [3]int) {
	bi, bj, bestD := 0, 1, -1
	for i := range 16 {
		for j := i + 1; j < 16; j++ {
			dr := pq[i][0] - pq[j][0]
			dg := pq[i][1] - pq[j][1]
			db := pq[i][2] - pq[j][2]
			if d := dr*dr + dg*dg + db*db; d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}
	return pq[bi], pq[bj]
}

// bc6hSubsetMaxDistPair returns max-distance pair within one subset.
func bc6hSubsetMaxDistPair(pq *[16][3]int, part int, subset int) ([3]int, [3]int) {
	bi, bj, bestD := -1, -1, -1
	var first [3]int
	haveFirst := false
	for i := range 16 {
		if int(bc6hPartitionSets[part][i]&0x01) != subset {
			continue
		}

		if !haveFirst {
			first = pq[i]
			haveFirst = true
		}

		for j := i + 1; j < 16; j++ {
			if int(bc6hPartitionSets[part][j]&0x01) != subset {
				continue
			}

			dr := pq[i][0] - pq[j][0]
			dg := pq[i][1] - pq[j][1]
			db := pq[i][2] - pq[j][2]
			if d := dr*dr + dg*dg + db*db; d > bestD {
				bestD, bi, bj = d, i, j
			}
		}
	}

	if bi < 0 {
		return first, first
	}

	return pq[bi], pq[bj]
}

// bc6hRank2SubsetN ranks the 32 BC6H partitions by total within-subset variance
// in the pre-quantized int domain, returning the top maxN partition indices.
// blk32 is the SOA-layout block (R[16], G[16], B[16] as int32), already computed by the caller;
// it avoids the per-channel AOS dereference inside the inner loop.
func bc6hRank2SubsetN(blk32 *[48]int32, maxN int) [32]int {
	if maxN > 32 {
		maxN = 32
	}

	var keys [32]int
	for p := range 32 {
		var sum [2][3]int
		var sumSq [2][3]int
		var cnt [2]int
		parts := &bc6hPartitionSets[p]
		for i := range 16 {
			s := int(parts[i] & 0x01)
			r := int(blk32[i])
			g := int(blk32[16+i])
			b := int(blk32[32+i])
			sum[s][0] += r
			sum[s][1] += g
			sum[s][2] += b
			sumSq[s][0] += r * r
			sumSq[s][1] += g * g
			sumSq[s][2] += b * b
			cnt[s]++
		}

		total := 0
		for s := range 2 {
			if cnt[s] > 0 {
				for c := range 3 {
					mean := sum[s][c] / cnt[s]
					total += sumSq[s][c] - 2*mean*sum[s][c] + cnt[s]*mean*mean
				}
			}
		}
		keys[p] = total<<5 | p
	}

	// partial selection sort for the top maxN smallest keys
	for i := range maxN {
		m := i
		for j := i + 1; j < 32; j++ {
			if keys[j] < keys[m] {
				m = j
			}
		}
		keys[i], keys[m] = keys[m], keys[i]
	}

	var order [32]int
	for i := range maxN {
		order[i] = keys[i] & 0x1F
	}

	return order
}

// bc6hBlockSOA converts pq from AOS [16][3]int64 to SOA [48]int32 (R[16], G[16], B[16]).
// Pre-computing this once per block eliminates repeated per-call conversions.
func bc6hBlockSOA(pq *[16][3]int) [48]int32 {
	var out [48]int32
	for i := range 16 {
		out[i] = int32(pq[i][0])    // #nosec G115 -- pre-quantized, max ~65534 < 2^31.
		out[16+i] = int32(pq[i][1]) // #nosec G115
		out[32+i] = int32(pq[i][2]) // #nosec G115
	}

	return out
}

// bc6hFindIdx1 is the inner-loop variant of bc6hFindIndices1Sub.
// blk32 is the block pre-converted to SOA int32 (computed once per encode call).
// Falls back to bc6hFindIndices1SubGo when AVX2 is unavailable.
func bc6hFindIdx1(blk32 *[48]int32, pq *[16][3]int, ep0, ep1 [3]int) [16]byte {
	if idx, ok := bc6hFindIdx1ASM(blk32, ep0, ep1); ok {
		return idx
	}

	return bc6hFindIndices1SubGo(pq, ep0, ep1)
}

// bc6hFindIdx2 is the inner-loop variant of bc6hFindIndices2Sub.
func bc6hFindIdx2(blk32 *[48]int32, pq *[16][3]int, ep0, ep1 [3]int, part, subset int) [16]byte {
	if idx, ok := bc6hFindIdx2ASM(blk32, ep0, ep1, part, subset); ok {
		return idx
	}

	return bc6hFindIndices2SubGo(pq, ep0, ep1, part, subset)
}

// bc6hFindIndices1SubGo is the pure-Go fallback used by bc6hFindIdx1.
// Builds the palette in per-channel flat arrays to eliminate struct overhead
// in the inner loop and removes the -1 sentinel with math.MaxInt.
func bc6hFindIndices1SubGo(pq *[16][3]int, ep0, ep1 [3]int) [16]byte {
	var palR, palG, palB [16]int
	w := bc6hAWeight4
	for k := range 16 {
		wk := w[k]
		palR[k] = (ep0[0]*(64-wk) + ep1[0]*wk + 32) >> 6
		palG[k] = (ep0[1]*(64-wk) + ep1[1]*wk + 32) >> 6
		palB[k] = (ep0[2]*(64-wk) + ep1[2]*wk + 32) >> 6
	}

	var idx [16]byte
	for i := range 16 {
		pr, pg, pb := pq[i][0], pq[i][1], pq[i][2]
		bestE, best := math.MaxInt, 0
		for k := range 16 {
			dr := pr - palR[k]
			if dr < 0 {
				dr = -dr
			}
			dg := pg - palG[k]
			if dg < 0 {
				dg = -dg
			}
			db := pb - palB[k]
			if db < 0 {
				db = -db
			}
			if e := dr + dg + db; e < bestE {
				bestE, best = e, k
			}
		}
		idx[i] = byte(best) // #nosec G115 -- best is in [0,15].
	}

	return idx
}

// bc6hFindIndices2SubGo is the pure-Go fallback used by bc6hFindIdx2.
func bc6hFindIndices2SubGo(pq *[16][3]int, ep0, ep1 [3]int, part, subset int) [16]byte {
	var palR, palG, palB [8]int
	w := bc6hAWeight3
	for k := range 8 {
		wk := w[k]
		palR[k] = (ep0[0]*(64-wk) + ep1[0]*wk + 32) >> 6
		palG[k] = (ep0[1]*(64-wk) + ep1[1]*wk + 32) >> 6
		palB[k] = (ep0[2]*(64-wk) + ep1[2]*wk + 32) >> 6
	}

	var idx [16]byte
	for i := range 16 {
		if int(bc6hPartitionSets[part][i]&0x01) != subset {
			continue
		}

		pr, pg, pb := pq[i][0], pq[i][1], pq[i][2]
		bestE, best := math.MaxInt, 0
		for k := range 8 {
			dr := pr - palR[k]
			if dr < 0 {
				dr = -dr
			}
			dg := pg - palG[k]
			if dg < 0 {
				dg = -dg
			}
			db := pb - palB[k]
			if db < 0 {
				db = -db
			}
			if e := dr + dg + db; e < bestE {
				bestE, best = e, k
			}
		}
		idx[i] = byte(best) // #nosec G115 -- best is in [0,7].
	}

	return idx
}

// bc6hSwap1Sub swaps ep0/ep1 if the anchor index (texel 0) has its MSB set,
// then re-assigns all indices. blk32 is the SOA block pre-converted by the caller.
func bc6hSwap1Sub(pq *[16][3]int, blk32 *[48]int32, ep0, ep1 *[3]int, idx *[16]byte) {
	if idx[0]&0x08 == 0 {
		return
	}

	*ep0, *ep1 = *ep1, *ep0
	*idx = bc6hFindIdx1(blk32, pq, *ep0, *ep1)
}

// bc6hSwap2Sub swaps endpoints for one subset if its anchor index has its MSB set.
func bc6hSwap2Sub(pq *[16][3]int, blk32 *[48]int32, ep0, ep1 *[3]int, idx *[16]byte, part, subset int) {
	anchor := 0
	if subset == 1 {
		anchor = bc6hAnchorIndex2Sub[part]
	}
	if idx[anchor]&0x04 == 0 {
		return
	}

	*ep0, *ep1 = *ep1, *ep0
	bc6hReassign2Sub(pq, blk32, ep0, ep1, idx, part, subset)
}

func bc6hReassign2Sub(pq *[16][3]int, blk32 *[48]int32, ep0, ep1 *[3]int, idx *[16]byte, part, subset int) {
	tmp := bc6hFindIdx2(blk32, pq, *ep0, *ep1, part, subset)
	for i := range 16 {
		if int(bc6hPartitionSets[part][i]&0x01) == subset {
			idx[i] = tmp[i]
		}
	}
}

// bc6hBlockError computes total squared error between the original half-float block
// and the decoded result in the finishUnquantize half-int domain.
func bc6hBlockError(original, decoded [48]uint16) int64 {
	var total int64
	for i := range 48 {
		d := int64(original[i]&0x7FFF) - int64(decoded[i]&0x7FFF)
		total += d * d
	}

	return total
}

// bc6hLSQ1Sub refits endpoints using least-squares for a fixed index assignment.
// Returns false if degenerate.
func bc6hLSQ1Sub(pq *[16][3]int, idx *[16]byte) ([3]int, [3]int, bool) {
	var saa, sbb, sab int
	var sap, sbp [3]int

	w := bc6hAWeight4[:]
	for i, px := range pq {
		b := w[idx[i]]
		a := 64 - b
		saa += a * a
		sbb += b * b
		sab += a * b
		for c := range 3 {
			sap[c] += a * px[c]
			sbp[c] += b * px[c]
		}
	}

	denom := int64(saa)*int64(sbb) - int64(sab)*int64(sab)
	if denom == 0 {
		return [3]int{}, [3]int{}, false
	}

	var ep0, ep1 [3]int
	for c := range 3 {
		v0, v1 := lsqSolvePair(64, saa, sbb, sab, sap[c], sbp[c], denom)
		ep0[c] = v0
		ep1[c] = v1
	}
	return ep0, ep1, true
}

// bc6hModeDesc groups mode metadata used by the encoder.
type bc6hModeDesc struct {
	wBits       int
	deltaBits   [3]int
	numSubsets  int // 1 or 2
	transformed bool
}

var bc6hModes = [14]bc6hModeDesc{
	{10, [3]int{5, 5, 5}, 2, true},     // mode 0
	{7, [3]int{6, 6, 6}, 2, true},      // mode 1
	{11, [3]int{5, 4, 4}, 2, true},     // mode 2
	{11, [3]int{4, 5, 4}, 2, true},     // mode 3
	{11, [3]int{4, 4, 5}, 2, true},     // mode 4
	{9, [3]int{5, 5, 5}, 2, true},      // mode 5
	{8, [3]int{6, 5, 5}, 2, true},      // mode 6
	{8, [3]int{5, 6, 5}, 2, true},      // mode 7
	{8, [3]int{5, 5, 6}, 2, true},      // mode 8
	{6, [3]int{6, 6, 6}, 2, false},     // mode 9  (non-transformed)
	{10, [3]int{10, 10, 10}, 1, false}, // mode 10 (non-transformed)
	{11, [3]int{9, 9, 9}, 1, true},     // mode 11
	{12, [3]int{8, 8, 8}, 1, true},     // mode 12
	{16, [3]int{4, 4, 4}, 1, true},     // mode 13
}

// encodeBlockBC6H encodes one BC6H block, returning the best 16 bytes found.
func encodeBlockBC6H(block [48]uint16, signed bool, quality int) [16]byte {
	pq := bc6hBlockPreQ(block, signed)

	// Pre-convert block to SOA int32 once; all bc6hFindIdx* calls reuse this.
	blk32 := bc6hBlockSOA(&pq)

	// number of 2-subset partitions to try
	maxParts := 4
	if quality >= 8 {
		maxParts = 32
	} else if quality >= 4 {
		maxParts = 8
	}

	var best [16]byte
	bestErr := int64(-1)

	tryCandidate := func(encoded [16]byte) {
		dec := decodeBlockBC6H(encoded[:], signed)
		err := bc6hBlockError(block, dec)
		if bestErr < 0 || err < bestErr {
			bestErr = err
			best = encoded
		}
	}

	// 1-subset modes
	ep0raw, ep1raw := bc6hMaxDistPair(&pq)

	// clamp to positive for unsigned
	if !signed {
		for c := range 3 {
			if ep0raw[c] < 0 {
				ep0raw[c] = 0
			}
			if ep1raw[c] < 0 {
				ep1raw[c] = 0
			}
		}
	}

	// mode 10: 10-bit non-transformed (always succeeds, always try it)
	{
		md := bc6hModes[10]
		q0 := bc6hQuantizeEP(ep0raw, md.wBits, signed)
		q1 := bc6hQuantizeEP(ep1raw, md.wBits, signed)

		uq0 := [3]int{
			bc6hUnquantize(q0[0], md.wBits, signed),
			bc6hUnquantize(q0[1], md.wBits, signed),
			bc6hUnquantize(q0[2], md.wBits, signed),
		}
		uq1 := [3]int{
			bc6hUnquantize(q1[0], md.wBits, signed),
			bc6hUnquantize(q1[1], md.wBits, signed),
			bc6hUnquantize(q1[2], md.wBits, signed),
		}

		idx := bc6hFindIdx1(&blk32, &pq, uq0, uq1)
		bc6hSwap1Sub(&pq, &blk32, &uq0, &uq1, &idx)

		q0 = bc6hQuantizeEP(uq0, md.wBits, signed)
		q1 = bc6hQuantizeEP(uq1, md.wBits, signed)
		tryCandidate(packBC6HMode10(q0, q1, idx))

		if quality >= 4 {
			// LSQ refit iterations
			for range 4 {
				nr0, nr1, ok := bc6hLSQ1Sub(&pq, &idx)
				if !ok {
					break
				}

				nq0 := bc6hQuantizeEP(nr0, md.wBits, signed)
				nq1 := bc6hQuantizeEP(nr1, md.wBits, signed)

				nuq0 := [3]int{
					bc6hUnquantize(nq0[0], md.wBits, signed),
					bc6hUnquantize(nq0[1], md.wBits, signed),
					bc6hUnquantize(nq0[2], md.wBits, signed),
				}
				nuq1 := [3]int{
					bc6hUnquantize(nq1[0], md.wBits, signed),
					bc6hUnquantize(nq1[1], md.wBits, signed),
					bc6hUnquantize(nq1[2], md.wBits, signed),
				}

				nidx := bc6hFindIdx1(&blk32, &pq, nuq0, nuq1)
				bc6hSwap1Sub(&pq, &blk32, &nuq0, &nuq1, &nidx)

				nq0 = bc6hQuantizeEP(nuq0, md.wBits, signed)
				nq1 = bc6hQuantizeEP(nuq1, md.wBits, signed)
				tryCandidate(packBC6HMode10(nq0, nq1, nidx))
				if nidx == idx {
					break
				}

				idx = nidx
			}
		}
	}

	// modes 11, 12, 13: transformed 1-subset, try when quality >= 4
	if quality >= 4 {
		for _, modeIdx := range []int{11, 12, 13} {
			md := bc6hModes[modeIdx]
			q0 := bc6hQuantizeEP(ep0raw, md.wBits, signed)
			q1raw := bc6hQuantizeEP(ep1raw, md.wBits, signed)

			// pre-check delta overflow with initial seed before paying for index assignment
			bad := false
			bc6hCreateDeltaRGB(q0, q1raw, md.deltaBits, &bad)
			if bad {
				continue
			}

			uq0 := [3]int{
				bc6hUnquantize(q0[0], md.wBits, signed),
				bc6hUnquantize(q0[1], md.wBits, signed),
				bc6hUnquantize(q0[2], md.wBits, signed),
			}
			uq1 := [3]int{
				bc6hUnquantize(q1raw[0], md.wBits, signed),
				bc6hUnquantize(q1raw[1], md.wBits, signed),
				bc6hUnquantize(q1raw[2], md.wBits, signed),
			}

			idx := bc6hFindIdx1(&blk32, &pq, uq0, uq1)
			bc6hSwap1Sub(&pq, &blk32, &uq0, &uq1, &idx)

			q0f := bc6hQuantizeEP(uq0, md.wBits, signed)
			q1f := bc6hQuantizeEP(uq1, md.wBits, signed)
			bad = false
			q1d := bc6hCreateDeltaRGB(q0f, q1f, md.deltaBits, &bad)
			if bad {
				continue
			}

			var enc [16]byte
			switch modeIdx {
			case 11:
				enc = packBC6HMode11(q0f, q1d, idx)
			case 12:
				enc = packBC6HMode12(q0f, q1d, idx)
			case 13:
				enc = packBC6HMode13(q0f, q1d, idx)
			}

			tryCandidate(enc)
		}
	}

	// 2-subset modes
	if quality >= 2 {
		partOrder := bc6hRank2SubsetN(&blk32, maxParts)
		packFuncs2Sub := [10]func([3]int, [3]int, [3]int, [3]int, int, [16]byte) [16]byte{
			packBC6HMode0, packBC6HMode1, packBC6HMode2, packBC6HMode3, packBC6HMode4,
			packBC6HMode5, packBC6HMode6, packBC6HMode7, packBC6HMode8, packBC6HMode9,
		}
		modeOrder := []int{9, 5, 0, 1, 2, 3, 4, 6, 7, 8} // non-transformed first, then by wBits
		if quality < 4 {
			modeOrder = []int{9, 5}
		}

		for pi := range maxParts {
			part := partOrder[pi]

			ep0s, ep1s := bc6hSubsetMaxDistPair(&pq, part, 0)
			ep2s, ep3s := bc6hSubsetMaxDistPair(&pq, part, 1)

			if !signed {
				for c := range 3 {
					if ep0s[c] < 0 {
						ep0s[c] = 0
					}
					if ep1s[c] < 0 {
						ep1s[c] = 0
					}
					if ep2s[c] < 0 {
						ep2s[c] = 0
					}
					if ep3s[c] < 0 {
						ep3s[c] = 0
					}
				}
			}

			for _, mi := range modeOrder {
				md := bc6hModes[mi]
				q0 := bc6hQuantizeEP(ep0s, md.wBits, signed)
				q1raw := bc6hQuantizeEP(ep1s, md.wBits, signed)
				q2raw := bc6hQuantizeEP(ep2s, md.wBits, signed)
				q3raw := bc6hQuantizeEP(ep3s, md.wBits, signed)

				// pre-check: reject this mode if initial endpoints can't be delta-encoded
				if md.transformed {
					bad := false
					bc6hCreateDeltaRGB(q0, q1raw, md.deltaBits, &bad)
					bc6hCreateDeltaRGB(q0, q2raw, md.deltaBits, &bad)
					bc6hCreateDeltaRGB(q0, q3raw, md.deltaBits, &bad)
					if bad {
						continue
					}
				}

				uq0 := [3]int{
					bc6hUnquantize(q0[0], md.wBits, signed),
					bc6hUnquantize(q0[1], md.wBits, signed),
					bc6hUnquantize(q0[2], md.wBits, signed),
				}
				uq1 := [3]int{
					bc6hUnquantize(q1raw[0], md.wBits, signed),
					bc6hUnquantize(q1raw[1], md.wBits, signed),
					bc6hUnquantize(q1raw[2], md.wBits, signed),
				}
				uq2 := [3]int{
					bc6hUnquantize(q2raw[0], md.wBits, signed),
					bc6hUnquantize(q2raw[1], md.wBits, signed),
					bc6hUnquantize(q2raw[2], md.wBits, signed),
				}
				uq3 := [3]int{
					bc6hUnquantize(q3raw[0], md.wBits, signed),
					bc6hUnquantize(q3raw[1], md.wBits, signed),
					bc6hUnquantize(q3raw[2], md.wBits, signed),
				}

				var idx [16]byte
				s0idx := bc6hFindIdx2(&blk32, &pq, uq0, uq1, part, 0)
				s1idx := bc6hFindIdx2(&blk32, &pq, uq2, uq3, part, 1)
				for i := range 16 {
					if bc6hPartitionSets[part][i]&0x01 == 0 {
						idx[i] = s0idx[i]
					} else {
						idx[i] = s1idx[i]
					}
				}

				bc6hSwap2Sub(&pq, &blk32, &uq0, &uq1, &idx, part, 0)
				bc6hSwap2Sub(&pq, &blk32, &uq2, &uq3, &idx, part, 1)

				q0f := bc6hQuantizeEP(uq0, md.wBits, signed)
				q1f := bc6hQuantizeEP(uq1, md.wBits, signed)
				q2f := bc6hQuantizeEP(uq2, md.wBits, signed)
				q3f := bc6hQuantizeEP(uq3, md.wBits, signed)

				var q1d, q2d, q3d [3]int
				if md.transformed {
					bad := false
					q1d = bc6hCreateDeltaRGB(q0f, q1f, md.deltaBits, &bad)
					q2d = bc6hCreateDeltaRGB(q0f, q2f, md.deltaBits, &bad)
					q3d = bc6hCreateDeltaRGB(q0f, q3f, md.deltaBits, &bad)
					if bad {
						continue
					}
				} else {
					q1d = q1f
					q2d = q2f
					q3d = q3f
				}

				tryCandidate(packFuncs2Sub[mi](q0f, q1d, q2d, q3d, part, idx))
			}
		}
	}

	return best
}

// encodeRangeBC6H encodes a contiguous range of 4x4 blocks [start, end) into dst.
// Blocks are addressed in row-major order; x and y track position within the block grid.
func encodeRangeBC6H(src []uint16, dst []byte, width, height, bx, start, end int, signed bool, quality int) {
	x := start % bx
	y := start / bx
	spos := start * 16

	for range end - start {
		var block [48]uint16
		bxOff := x * 4
		byOff := y * 4
		for row := range 4 {
			py := byOff + row
			if py >= height {
				py = height - 1
			}
			for col := range 4 {
				px := bxOff + col
				if px >= width {
					px = width - 1
				}
				off := (py*width + px) * 3
				block[(row*4+col)*3+0] = src[off+0]
				block[(row*4+col)*3+1] = src[off+1]
				block[(row*4+col)*3+2] = src[off+2]
			}
		}

		enc := encodeBlockBC6H(block, signed, quality)
		copy(dst[spos:spos+16], enc[:])
		spos += 16
		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeBlocksBC6H is the internal parallel dispatcher for BC6H encoding.
// It splits the block grid into per-worker ranges and calls encodeRangeBC6H on each.
func encodeBlocksBC6H(src []uint16, width, height int, signed bool, opts *EncodeOptions) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}
	if len(src) != width*height*3 {
		return nil, ErrInvalidHDRSliceLength
	}

	bx := (width + 3) / 4
	by := (height + 3) / 4
	total := bx * by
	dst := make([]byte, total*16)

	quality := QualityLevelBalanced
	workers := runtime.GOMAXPROCS(0)
	if opts != nil {
		if opts.QualityLevel > 0 {
			quality = opts.QualityLevel
		}
		if opts.Workers > 0 {
			workers = opts.Workers
		}
	}
	if workers > total {
		workers = total
	}

	if workers <= 1 || total < 256*workers {
		encodeRangeBC6H(src, dst, width, height, bx, 0, total, signed, quality)
		return dst, nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		s := (total * w) / workers
		e := (total * (w + 1)) / workers
		go func(s, e int) {
			defer wg.Done()
			encodeRangeBC6H(src, dst, width, height, bx, s, e, signed, quality)
		}(s, e)
	}
	wg.Wait()

	_ = by
	return dst, nil
}
