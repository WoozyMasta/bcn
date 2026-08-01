// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

package bcn

import (
	"unsafe"

	"github.com/woozymasta/bcn/internal/simd"
)

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	if simd.Enabled {
		// [16]rgba8 is 64 contiguous bytes (four uint8 fields, no padding).
		v := simd.FindMinMaxSSE2((*[64]byte)(unsafe.Pointer(&block)))
		return rgba8FromPacked(uint32(v)), rgba8FromPacked(uint32(v >> 32)) // #nosec G115 -- intentional 32-bit halves.
	}

	return findMinMaxGeneric(block)
}

// rgba8FromPacked splits a packed little-endian RGBA word.
func rgba8FromPacked(v uint32) rgba8 {
	// #nosec G115 -- intentional byte extraction.
	return rgba8{r: uint8(v), g: uint8(v >> 8), b: uint8(v >> 16), a: uint8(v >> 24)}
}

// pack3Penalty keeps palette entry 3 from winning the argmin in alpha mode.
// It exceeds the maximum per-pixel weighted SSE (255^2*1024*3 < 2^28) while
// leaving headroom against int32 overflow when added to a real error.
const pack3Penalty = int32(1) << 30

// packBC1IndicesASM assigns palette indices with the AVX2 kernel.
// Returns ok=false when AVX2 is unavailable and the caller must fall back.
func packBC1IndicesASM(block *[16]rgba8, palette *[4]rgba8, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP) (uint32, bool) {
	if !simd.HasAVX2 {
		return 0, false
	}

	var params [20]int32
	for k := range 4 {
		params[k] = int32(palette[k].r)
		params[4+k] = int32(palette[k].g)
		params[8+k] = int32(palette[k].b)
	}
	params[12] = w.r
	params[13] = w.g
	params[14] = w.b
	if hasAlpha {
		// Sub-threshold pixels are forced to entry 3; the penalty turns the
		// remaining argmin into a limit-3 search (entry 3 never wins).
		params[15] = int32(alphaThreshold)
		params[16] = pack3Penalty
	}
	// Opaque mode leaves threshold and penalty zero: entry 3 competes (limit 4)
	// and no pixel is forced (alpha is always >= 0).

	idx := simd.PackBC1IndicesAVX2((*[64]byte)(unsafe.Pointer(block)), &params)
	return idx, true
}

// scoreBC1PaletteASM returns the total weighted block error of one opaque-mode
// BC1 endpoint pair via the AVX2 kernel. Returns ok=false when AVX2 is
// unavailable and the caller must use the scalar path.
func scoreBC1PaletteASM(block *[16]rgba8, c0, c1 uint16, w rgbWeightsFP) (int64, bool) {
	if !simd.HasAVX2 {
		return 0, false
	}

	weights := [4]int32{w.r, w.g, w.b, 0}
	cc := uint32(c0) | uint32(c1)<<16
	e := simd.ScoreBC1PaletteAVX2((*[64]byte)(unsafe.Pointer(block)), cc, &weights)
	return int64(e), true
}

// bc7Mode6IndicesASM assigns each texel to the nearest mode 6 palette entry.
// Returns ok=false when AVX2 is unavailable.
func bc7Mode6IndicesASM(block *[16]rgba8, pal *[16]rgba8) ([16]uint8, int, bool) {
	var idx [16]uint8
	if !simd.HasAVX2 {
		return idx, 0, false
	}

	var params [64]int32
	for k := range 16 {
		params[k] = int32(pal[k].r)
		params[16+k] = int32(pal[k].g)
		params[32+k] = int32(pal[k].b)
		params[48+k] = int32(pal[k].a)
	}

	var idx32 [16]int32
	total := simd.BC7Mode6IndicesAVX2((*[64]byte)(unsafe.Pointer(block)), &params, &idx32)
	for i := range 16 {
		idx[i] = uint8(idx32[i]) // #nosec G115 -- kernel writes values in [0,15].
	}

	return idx, int(total), true
}

// bc7Color4LSQASM assigns texels to a 4-entry BC7 RGB palette
// and accumulates least-squares sums with BC7 weight2 beta numerators.
// Returns ok=false when AVX2 is unavailable.
func bc7Color4LSQASM(block *[16]rgba8, pal *[4]rgba8) (lsqColorSums, bool) {
	betaNum := [4]int{0, int(bc7Weight2[1]), int(bc7Weight2[2]), int(bc7Weight2[3])}

	// Equal positive weights preserve the unweighted bc7RGBErr argmin while
	// reusing the existing weighted BC1 LSQ kernel.
	return lsqColorAccumulateASM(block, pal, false, 0, rgbWeightsFP{r: 1, g: 1, b: 1}, 64, &betaNum)
}

// bc7SubsetEvalASM finds, for the texels of one subset,
// the nearest of up to 8 RGB palette entries and returns
// least-squares sums together with the total nearest-entry error.
// Palette is padded to 8 entries with entry 0, which can never win the argmin
// (it ties entry 0 and strict-less keeps the lower index),
// so a 4-entry palette behaves exactly like the scalar path.
// Returns ok=false when AVX2 is unavailable.
func bc7SubsetEvalASM(block *[16]rgba8, part *[16]uint8, subset uint8, pal []rgba8, weights []int32) (lsqColorSums, int, bool) {
	if !simd.HasAVX2 {
		return lsqColorSums{}, 0, false
	}

	var params [34]int32
	for k := range 8 {
		src := k
		if k >= len(pal) {
			src = 0 // pad unused slots with entry 0 (never selected)
		}
		params[k] = int32(pal[src].r)
		params[8+k] = int32(pal[src].g)
		params[16+k] = int32(pal[src].b)
		if src < len(weights) {
			params[24+k] = weights[src]
		}
	}
	params[32] = 64 // interpolation denominator d
	params[33] = int32(subset)

	var outv [10]int32
	simd.BC7SubsetEvalAVX2(
		(*[64]byte)(unsafe.Pointer(block)),
		(*[16]byte)(unsafe.Pointer(part)),
		&params, &outv,
	)

	sums := lsqColorSums{
		saa:  int(outv[0]),
		sbb:  int(outv[1]),
		sab:  int(outv[2]),
		sapR: int(outv[3]),
		sapG: int(outv[4]),
		sapB: int(outv[5]),
		sbpR: int(outv[6]),
		sbpG: int(outv[7]),
		sbpB: int(outv[8]),
	}

	return sums, int(outv[9]), true
}

// bc7Mode7SubsetEvalASM is the RGBA counterpart of bc7SubsetEvalASM for mode 7:
// for the texels of one subset it finds the nearest of 4 RGBA palette entries
// and returns the full RGBA least-squares sums plus the total error.
// Returns ok=false when AVX2 is unavailable.
func bc7Mode7SubsetEvalASM(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) (bc7Mode7Sums, int, bool) {
	if !simd.HasAVX2 {
		return bc7Mode7Sums{}, 0, false
	}

	var params [22]int32
	for k := range 4 {
		params[k] = int32(pal[k].r)
		params[4+k] = int32(pal[k].g)
		params[8+k] = int32(pal[k].b)
		params[12+k] = int32(pal[k].a)
		params[16+k] = bc7Weight2[k]
	}
	params[20] = 64 // interpolation denominator d
	params[21] = int32(subset)

	var outv [12]int32
	simd.BC7Mode7SubsetEvalAVX2(
		(*[64]byte)(unsafe.Pointer(block)),
		(*[16]byte)(unsafe.Pointer(part)),
		&params, &outv,
	)

	sums := bc7Mode7Sums{
		saa: int(outv[0]),
		sbb: int(outv[1]),
		sab: int(outv[2]),
		sap: [4]int{int(outv[3]), int(outv[4]), int(outv[5]), int(outv[6])},
		sbp: [4]int{int(outv[7]), int(outv[8]), int(outv[9]), int(outv[10])},
	}

	return sums, int(outv[11]), true
}

// alphaBlockErrorASM scores 16 alpha samples against
// palette of endpoints a0, a1 via AVX2 (the kernel builds the palette).
// Returns ok=false when AVX2 is unavailable.
func alphaBlockErrorASM(samples *[16]uint8, a0, a1 uint8) (int, bool) {
	if !simd.HasAVX2 {
		return 0, false
	}

	return int(simd.AlphaBlockErrorAVX2(samples, uint32(a0)|uint32(a1)<<8)), true
}

// bestAlphaIndices16ASM packs the nearest palette indices
// for 16 alpha samples against the palette of endpoints a0, a1 via AVX2.
// Returns ok=false when AVX2 is unavailable.
func bestAlphaIndices16ASM(samples *[16]uint8, a0, a1 uint8) (uint64, bool) {
	if !simd.HasAVX2 {
		return 0, false
	}

	return simd.BestAlphaIndices16AVX2(samples, uint32(a0)|uint32(a1)<<8), true
}

// lsqColorAccumulateASM uses AVX2 for the LSQ assign step
// and normal-equation accumulation. Returns ok=false when AVX2 is unavailable.
func lsqColorAccumulateASM(
	block *[16]rgba8,
	palette *[4]rgba8,
	hasAlpha bool,
	alphaThreshold uint8,
	w rgbWeightsFP,
	d int,
	betaNum *[4]int,
) (lsqColorSums, bool) {
	if !simd.HasAVX2 {
		return lsqColorSums{}, false
	}

	var params [22]int32
	for k := range 4 {
		params[k] = int32(palette[k].r)
		params[4+k] = int32(palette[k].g)
		params[8+k] = int32(palette[k].b)
		params[18+k] = int32(betaNum[k]) // #nosec G115 -- beta numerators are fixed small palette weights.
	}
	params[12] = w.r
	params[13] = w.g
	params[14] = w.b
	if hasAlpha {
		params[15] = int32(alphaThreshold)
		params[16] = pack3Penalty
	}
	params[17] = int32(d) // #nosec G115 -- denominator is 2 or 3.

	var out [9]int32
	simd.LSQColorAccumulateAVX2((*[64]byte)(unsafe.Pointer(block)), &params, &out)

	return lsqColorSums{
		saa:  int(out[0]),
		sbb:  int(out[1]),
		sab:  int(out[2]),
		sapR: int(out[3]),
		sapG: int(out[4]),
		sapB: int(out[5]),
		sbpR: int(out[6]),
		sbpG: int(out[7]),
		sbpB: int(out[8]),
	}, true
}

// lsqAlphaAccumulateASM uses AVX2 for the LSQ assign step
// and normal-equation accumulation. Returns ok=false when AVX2 is unavailable.
func lsqAlphaAccumulateASM(samples *[16]uint8, a0, a1 uint8) (lsqAlphaSums, bool) {
	if !simd.HasAVX2 {
		return lsqAlphaSums{}, false
	}

	var out [5]int32
	simd.LSQAlphaAccumulateAVX2(samples, uint32(a0)|uint32(a1)<<8, &out)

	return lsqAlphaSums{
		saa: int(out[0]),
		sbb: int(out[1]),
		sab: int(out[2]),
		sap: int(out[3]),
		sbp: int(out[4]),
	}, true
}

// decodeRowKernel is an assembly routine decoding n consecutive interior
// blocks into 4 destination rows spaced stride bytes apart.
type decodeRowKernel func(dst *byte, src *byte, n int, stride int)

// decodeRangeRows feeds contiguous same-row block runs of [start, end) into
// an assembly row kernel. blockSize is the compressed block size in bytes.
func decodeRangeRows(kernel decodeRowKernel, data, out []byte, width, bx, start, end, blockSize int) {
	stride := width * 4
	for idx := start; idx < end; {
		y := idx / bx
		x := idx % bx
		n := min(end, (y+1)*bx) - idx
		kernel(&out[y*4*stride+x*16], &data[idx*blockSize], n, stride)
		idx += n
	}
}

// decodeRangeASMAvailable reports whether row kernels can decode this range:
// edge blocks (width/height not multiples of 4) need the generic path.
func decodeRangeASMAvailable(enabled bool, width, height, start, end int) bool {
	return enabled && width%4 == 0 && height%4 == 0 && start < end
}

// decodeRangeBC1ASM decodes blocks [start, end) with the AVX2 row kernel.
// Returns false when the kernel cannot be used and the caller must take the
// generic path.
func decodeRangeBC1ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC1RowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeBC2ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC2ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC2RowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// decodeRangeBC3ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC3ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC3RowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// decodeRangeBC4ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC4ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC4RowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeBC5ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC5ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC5RowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// decodeRangeBC4SASM decodes signed blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC4SASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC4SRowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeBC5SASM decodes signed blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC5SASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeBC5SRowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// bc6hFindIdx1ASM is the SOA-block variant of bc6hFindIndices1SubASM.
// The caller pre-converts the block once; only the palette is built here.
func bc6hFindIdx1ASM(blk *[48]int32, ep0, ep1 [3]int) ([16]byte, bool) {
	if !simd.HasAVX2 {
		return [16]byte{}, false
	}

	var pal [48]int32
	w := bc6hAWeight4
	for k := range 16 {
		wk := w[k]
		pal[k] = int32((ep0[0]*(64-wk) + ep1[0]*wk + 32) >> 6)    // #nosec G115 -- interpolated endpoint, max ~65534 < 2^31.
		pal[16+k] = int32((ep0[1]*(64-wk) + ep1[1]*wk + 32) >> 6) // #nosec G115
		pal[32+k] = int32((ep0[2]*(64-wk) + ep1[2]*wk + 32) >> 6) // #nosec G115
	}

	var idx32 [16]int32
	simd.BC6HFindIndices1SubAVX2(blk, &pal, &idx32)

	var idx [16]byte
	for i := range 16 {
		idx[i] = byte(idx32[i]) // #nosec G115 -- kernel writes values in [0,15].
	}

	return idx, true
}

// bc6hFindIdx2ASM is the SOA-block variant of bc6hFindIndices2SubASM.
func bc6hFindIdx2ASM(blk *[48]int32, ep0, ep1 [3]int, part, subset int) ([16]byte, bool) {
	if !simd.HasAVX2 {
		return [16]byte{}, false
	}

	var pal [24]int32
	w := bc6hAWeight3
	for k := range 8 {
		wk := w[k]
		pal[k] = int32((ep0[0]*(64-wk) + ep1[0]*wk + 32) >> 6)    // #nosec G115 -- interpolated endpoint, max ~65534 < 2^31.
		pal[8+k] = int32((ep0[1]*(64-wk) + ep1[1]*wk + 32) >> 6)  // #nosec G115
		pal[16+k] = int32((ep0[2]*(64-wk) + ep1[2]*wk + 32) >> 6) // #nosec G115
	}

	var idx32 [16]int32
	simd.BC6HFindIndices2SubAVX2(
		blk, &pal,
		&bc6hPartitionSets[part],
		int32(subset), // #nosec G115 -- subset is 0 or 1.
		&idx32,
	)

	var idx [16]byte
	for i := range 16 {
		idx[i] = byte(idx32[i]) // #nosec G115 -- kernel writes values in [0,7].
	}

	return idx, true
}

// downscaleNRGBARow2xASM downsamples one row using the AVX2 mipmap kernel.
// n must be even; odd tails and clamp edges are handled by the Go caller.
func downscaleNRGBARow2xASM(dst, row0, row1 []byte, n int) bool {
	if !simd.HasAVX2 || n < 2 {
		return false
	}

	simd.DownscaleNRGBARow2xAVX2(&dst[0], &row0[0], &row1[0], n)
	return true
}
