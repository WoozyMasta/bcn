// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

// Shared BPTC (BC6H/BC7) bit-stream reader and BC7 mode tables.
// The decoders are ported from bcdec.h (MIT/Unlicense, (c) 2022 Sergii Kudlai);
// the large partition tables live in bptc_tables.go.

// bptcReader consumes a 16-byte block as a little-endian 128-bit window
// and pulls fields LSB-first, matching the BPTC bit layout used by bcdec.
type bptcReader struct {
	lo, hi uint64
}

// newBPTCReader loads a 16-byte block into the 128-bit reader window.
func newBPTCReader(b []byte) bptcReader {
	return bptcReader{
		lo: binary.LittleEndian.Uint64(b[0:8]),
		hi: binary.LittleEndian.Uint64(b[8:16]),
	}
}

// read pops the low n bits (1..32) and advances the window. n must be >= 1.
func (r *bptcReader) read(n int) uint32 {
	mask := uint64(1)<<uint(n) - 1
	// #nosec G115 -- n <= 8 in BPTC, so the masked value always fits uint32.
	bits := uint32(r.lo & mask)
	r.lo = (r.lo >> uint(n)) | (r.hi&mask)<<uint(64-n)
	r.hi >>= uint(n)

	return bits
}

// readBit pops a single bit.
func (r *bptcReader) readBit() uint32 {
	return r.read(1)
}

// readN pops n bits as int32, for the endpoint/index fields.
// BPTC fields are at most 8 bits, so the value always fits.
func (r *bptcReader) readN(n int) int32 {
	// #nosec G115 -- n <= 8 in BPTC, so the value always fits int32.
	return int32(r.read(n))
}

// bptcWriter is the inverse of bptcReader:
// it appends fields LSB-first into a little-endian 128-bit window,
// so encoded blocks decode with the same layout.
type bptcWriter struct {
	lo, hi uint64
	pos    int
}

// put appends the low n bits of v at the current position and advances.
// The field is inserted in one shift-and-mask step,
// splitting across the 64-bit halves only when it straddles the boundary (n is 1..32).
func (w *bptcWriter) put(v uint32, n int) {
	val := uint64(v) & (1<<uint(n) - 1)
	pos := w.pos
	w.pos += n

	if pos >= 64 {
		w.hi |= val << uint(pos-64)
		return
	}

	w.lo |= val << uint(pos)
	if pos+n > 64 {
		// The field crosses the lo/hi boundary; the high bits spill into hi.
		w.hi |= val >> uint(64-pos)
	}
}

// bytes returns the packed 16-byte block.
func (w *bptcWriter) bytes() [16]byte {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], w.lo)
	binary.LittleEndian.PutUint64(b[8:16], w.hi)
	return b
}

// BC7 interpolation weight tables for 2/3/4-bit indices (aWeight2/3/4 in bcdec).
var (
	bc7Weight2 = [4]int32{0, 21, 43, 64}
	bc7Weight3 = [8]int32{0, 9, 18, 27, 37, 46, 55, 64}
	bc7Weight4 = [16]int32{0, 4, 9, 13, 17, 21, 26, 30, 34, 38, 43, 47, 51, 55, 60, 64}
)

// bc7ColorBits and bc7AlphaBits are the RGB
// and alpha endpoint precisions per mode (actual_bits_count rows in bcdec).
var (
	bc7ColorBits = [8]int{4, 6, 5, 7, 5, 7, 7, 5}
	bc7AlphaBits = [8]int{0, 0, 0, 0, 6, 8, 7, 5}
)

// bc7ModeHasPBits marks the modes (0,1,3,6,7) that carry per-endpoint P-bits.
const bc7ModeHasPBits = 0b11001011

// bc7Interpolate blends two endpoint components by a weight table entry,
// rounding to the nearest 8-bit value: (a*(64-w) + b*w + 32) >> 6.
// With 8-bit endpoints and weights in [0,64] the result is always in [0,255].
func bc7Interpolate(a, b int32, weights []int32, index int) uint8 {
	w := weights[index]
	// #nosec G115 -- result is in [0,255] for 8-bit endpoints.
	return uint8((a*(64-w) + b*w + 32) >> 6)
}
