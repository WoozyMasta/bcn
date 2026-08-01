// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// bc6hU converts a signed int bit-pattern to uint32 for bptcWriter.put.
// All callers mask the result to the target field width,
// so the value is bounded and the conversion cannot produce an unintended high bit.
//
//nolint:gosec
func bc6hU(v int) uint32 { return uint32(v) }

// packBC6HMode0 packs a BC6H block using spec mode 1 (10.555, 2-subset, transformed, 3-bit indices).
// ep0/ep1 are the base endpoints; ep2/ep3 are the delta-encoded subset-1 endpoints.
func packBC6HMode0(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(0, 2)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0x1F, 5)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x1F, 5)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode1 packs a BC6H block using spec mode 2 (7.666, 2-subset, transformed, 3-bit indices).
func packBC6HMode1(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(1, 2)
	w.put(bc6hU(ep2[1]>>5)&1, 1)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep3[1]>>5)&1, 1)
	w.put(bc6hU(ep0[0])&0x7F, 7)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0x7F, 7)
	w.put(bc6hU(ep2[2]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0x7F, 7)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(ep3[2]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x3F, 6)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x3F, 6)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x3F, 6)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x3F, 6)
	w.put(bc6hU(ep3[0])&0x3F, 6)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode2 packs a BC6H block using spec mode 3 (11.544, 2-subset, transformed, 3-bit indices).
func packBC6HMode2(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(2, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0x1F, 5)
	w.put(bc6hU(ep0[0]>>10)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0xF, 4)
	w.put(bc6hU(ep0[1]>>10)&1, 1)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0xF, 4)
	w.put(bc6hU(ep0[2]>>10)&1, 1)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode3 packs a BC6H block using spec mode 4 (11.454, 2-subset, transformed, 3-bit indices).
func packBC6HMode3(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(6, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0xF, 4)
	w.put(bc6hU(ep0[0]>>10)&1, 1)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x1F, 5)
	w.put(bc6hU(ep0[1]>>10)&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0xF, 4)
	w.put(bc6hU(ep0[2]>>10)&1, 1)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0xF, 4)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0xF, 4)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode4 packs a BC6H block using spec mode 5 (11.445, 2-subset, transformed, 3-bit indices).
func packBC6HMode4(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(10, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0xF, 4)
	w.put(bc6hU(ep0[0]>>10)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0xF, 4)
	w.put(bc6hU(ep0[1]>>10)&1, 1)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x1F, 5)
	w.put(bc6hU(ep0[2]>>10)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0xF, 4)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0xF, 4)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode5 packs a BC6H block using spec mode 6 (9.555, 2-subset, transformed, 3-bit indices).
func packBC6HMode5(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(14, 5)
	w.put(bc6hU(ep0[0])&0x1FF, 9)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0x1FF, 9)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0x1FF, 9)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x1F, 5)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x1F, 5)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode6 packs a BC6H block using spec mode 7 (8.655, 2-subset, transformed, 3-bit indices).
func packBC6HMode6(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(18, 5)
	w.put(bc6hU(ep0[0])&0xFF, 8)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0xFF, 8)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0xFF, 8)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x3F, 6)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x1F, 5)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x3F, 6)
	w.put(bc6hU(ep3[0])&0x3F, 6)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode7 packs a BC6H block using spec mode 8 (8.565, 2-subset, transformed, 3-bit indices).
func packBC6HMode7(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(22, 5)
	w.put(bc6hU(ep0[0])&0xFF, 8)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0xFF, 8)
	w.put(bc6hU(ep2[1]>>5)&1, 1)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0xFF, 8)
	w.put(bc6hU(ep3[1]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x1F, 5)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x3F, 6)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode8 packs a BC6H block using spec mode 9 (8.556, 2-subset, transformed, 3-bit indices).
func packBC6HMode8(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(26, 5)
	w.put(bc6hU(ep0[0])&0xFF, 8)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0xFF, 8)
	w.put(bc6hU(ep2[2]>>5)&1, 1)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0xFF, 8)
	w.put(bc6hU(ep3[2]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x1F, 5)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x1F, 5)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x3F, 6)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep3[0])&0x1F, 5)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode9 packs a BC6H block using spec mode 10 (6.666, 2-subset, non-transformed, 3-bit indices).
func packBC6HMode9(ep0, ep1, ep2, ep3 [3]int, part int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(30, 5)
	w.put(bc6hU(ep0[0])&0x3F, 6)
	w.put(bc6hU(ep3[1]>>4)&1, 1)
	w.put(bc6hU(ep3[2])&1, 1)
	w.put(bc6hU(ep3[2]>>1)&1, 1)
	w.put(bc6hU(ep2[2]>>4)&1, 1)
	w.put(bc6hU(ep0[1])&0x3F, 6)
	w.put(bc6hU(ep2[1]>>5)&1, 1)
	w.put(bc6hU(ep2[2]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>2)&1, 1)
	w.put(bc6hU(ep2[1]>>4)&1, 1)
	w.put(bc6hU(ep0[2])&0x3F, 6)
	w.put(bc6hU(ep3[1]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>3)&1, 1)
	w.put(bc6hU(ep3[2]>>5)&1, 1)
	w.put(bc6hU(ep3[2]>>4)&1, 1)
	w.put(bc6hU(ep1[0])&0x3F, 6)
	w.put(bc6hU(ep2[1])&0xF, 4)
	w.put(bc6hU(ep1[1])&0x3F, 6)
	w.put(bc6hU(ep3[1])&0xF, 4)
	w.put(bc6hU(ep1[2])&0x3F, 6)
	w.put(bc6hU(ep2[2])&0xF, 4)
	w.put(bc6hU(ep2[0])&0x3F, 6)
	w.put(bc6hU(ep3[0])&0x3F, 6)
	w.put(bc6hU(part)&0x1F, 5)
	bc6hWriteIndices2Sub(&w, idx, part)
	return w.bytes()
}

// packBC6HMode10 packs a BC6H block using spec mode 11 (10.10.10, 1-subset, non-transformed, 4-bit indices).
func packBC6HMode10(ep0, ep1 [3]int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(3, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0x3FF, 10)
	w.put(bc6hU(ep1[1])&0x3FF, 10)
	w.put(bc6hU(ep1[2])&0x3FF, 10)
	bc6hWriteIndices1Sub(&w, idx)
	return w.bytes()
}

// packBC6HMode11 packs a BC6H block using spec mode 12 (11.9, 1-subset, transformed, 4-bit indices).
func packBC6HMode11(ep0, ep1 [3]int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(7, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0x1FF, 9)
	w.put(bc6hU(ep0[0]>>10)&1, 1)
	w.put(bc6hU(ep1[1])&0x1FF, 9)
	w.put(bc6hU(ep0[1]>>10)&1, 1)
	w.put(bc6hU(ep1[2])&0x1FF, 9)
	w.put(bc6hU(ep0[2]>>10)&1, 1)
	bc6hWriteIndices1Sub(&w, idx)
	return w.bytes()
}

// packBC6HMode12 packs a BC6H block using spec mode 13 (12.8, 1-subset, transformed, 4-bit indices).
// The two high bits of ep0 per channel are written MSB-first to mirror the reversed read in the decoder.
func packBC6HMode12(ep0, ep1 [3]int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(11, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0xFF, 8)
	// bits 11:10 of ep0 written MSB-first (reversed) per bptcReadBitsR(2) in decoder
	w.put(bc6hU(ep0[0]>>11)&1, 1)
	w.put(bc6hU(ep0[0]>>10)&1, 1)
	w.put(bc6hU(ep1[1])&0xFF, 8)
	w.put(bc6hU(ep0[1]>>11)&1, 1)
	w.put(bc6hU(ep0[1]>>10)&1, 1)
	w.put(bc6hU(ep1[2])&0xFF, 8)
	w.put(bc6hU(ep0[2]>>11)&1, 1)
	w.put(bc6hU(ep0[2]>>10)&1, 1)
	bc6hWriteIndices1Sub(&w, idx)
	return w.bytes()
}

// packBC6HMode13 packs a BC6H block using spec mode 14 (16.4, 1-subset, transformed, 4-bit indices).
// The six high bits of ep0 per channel are written MSB-first to match bptcReadBitsR(6) in the decoder.
func packBC6HMode13(ep0, ep1 [3]int, idx [16]byte) [16]byte {
	var w bptcWriter
	w.put(15, 5)
	w.put(bc6hU(ep0[0])&0x3FF, 10)
	w.put(bc6hU(ep0[1])&0x3FF, 10)
	w.put(bc6hU(ep0[2])&0x3FF, 10)
	w.put(bc6hU(ep1[0])&0xF, 4)
	// bits 15:10 of ep0 written MSB-first per bptcReadBitsR(6) in decoder
	for i := 15; i >= 10; i-- {
		w.put(bc6hU(ep0[0]>>uint(i))&1, 1)
	}
	w.put(bc6hU(ep1[1])&0xF, 4)
	for i := 15; i >= 10; i-- {
		w.put(bc6hU(ep0[1]>>uint(i))&1, 1)
	}
	w.put(bc6hU(ep1[2])&0xF, 4)
	for i := 15; i >= 10; i-- {
		w.put(bc6hU(ep0[2]>>uint(i))&1, 1)
	}
	bc6hWriteIndices1Sub(&w, idx)
	return w.bytes()
}

// bc6hWriteIndices1Sub writes 16 texel indices for 1-subset (4-bit each, anchor is 3-bit).
func bc6hWriteIndices1Sub(w *bptcWriter, idx [16]byte) {
	w.put(uint32(idx[0]), 3) // anchor: MSB implicit zero
	for i := 1; i < 16; i++ {
		w.put(uint32(idx[i]), 4)
	}
}

// bc6hWriteIndices2Sub writes 16 texel indices for 2-subset (3-bit each, two anchors are 2-bit).
func bc6hWriteIndices2Sub(w *bptcWriter, idx [16]byte, part int) {
	anchor1 := bc6hAnchorIndex2Sub[part]
	w.put(uint32(idx[0]), 2) // subset-0 anchor: MSB implicit zero
	for i := 1; i < 16; i++ {
		n := 3
		if i == anchor1 {
			n = 2 // subset-1 anchor: MSB implicit zero
		}
		w.put(uint32(idx[i]), n)
	}
}
