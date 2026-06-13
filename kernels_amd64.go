// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

//go:generate go run ./asm -out kernels_amd64.s -stubs kernels_stubs_amd64.go -pkg bcn

package bcn

import "unsafe"

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	if useASM {
		// [16]rgba8 is 64 contiguous bytes (four uint8 fields, no padding).
		v := findMinMaxSSE2((*[64]byte)(unsafe.Pointer(&block)))
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
	if !hasAVX2 {
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

	idx := packDXT1IndicesAVX2((*[64]byte)(unsafe.Pointer(block)), &params)
	return idx, true
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
	if !decodeRangeASMAvailable(hasAVX2, width, height, start, end) {
		return false
	}

	decodeRangeRows(decodeDXT1RowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeDXT5ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeDXT5ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(hasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(decodeDXT5RowAVX2, data, out, width, bx, start, end, 16)
	return true
}

// decodeRangeBC4ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC4ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(hasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(decodeBC4RowAVX2, data, out, width, bx, start, end, 8)
	return true
}

// decodeRangeBC5ASM decodes blocks [start, end) with the AVX2+BMI2 row kernel.
func decodeRangeBC5ASM(data, out []byte, width, height, bx, start, end int) bool {
	if !decodeRangeASMAvailable(hasAVX2BMI2, width, height, start, end) {
		return false
	}

	decodeRangeRows(decodeBC5RowAVX2, data, out, width, bx, start, end, 16)
	return true
}
