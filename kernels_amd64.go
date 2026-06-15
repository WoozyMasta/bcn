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

// packDXT1IndicesASM assigns palette indices with the AVX2 kernel.
// Returns ok=false when AVX2 is unavailable and the caller must fall back.
func packDXT1IndicesASM(block *[16]rgba8, palette *[4]rgba8, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP) (uint32, bool) {
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

	idx := simd.PackDXT1IndicesAVX2((*[64]byte)(unsafe.Pointer(block)), &params)
	return idx, true
}

// scoreDXT1PaletteASM returns the total weighted block error of one opaque-mode
// BC1 endpoint pair via the AVX2 kernel. Returns ok=false when AVX2 is
// unavailable and the caller must use the scalar path.
func scoreDXT1PaletteASM(block *[16]rgba8, c0, c1 uint16, w rgbWeightsFP) (int64, bool) {
	if !simd.HasAVX2 {
		return 0, false
	}

	weights := [4]int32{w.r, w.g, w.b, 0}
	cc := uint32(c0) | uint32(c1)<<16
	e := simd.ScoreDXT1PaletteAVX2((*[64]byte)(unsafe.Pointer(block)), cc, &weights)
	return int64(e), true
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

// decodeRangeDXT1ASM decodes blocks [start, end) with the AVX2 row kernel.
// Returns false when the kernel cannot be used and the caller must take the
// generic path.
func decodeRangeDXT1ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeDXT1RowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeDXT3ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeDXT3ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeDXT3RowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// decodeRangeDXT5ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeDXT5ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(simd.HasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(simd.DecodeDXT5RowAVX2, data, out, width, bx, start, end, 16)
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
