// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// Per-format block-range loops shared by the sequential drivers and the
// worker pools. Hoisting the format switch out of the per-block loop removes
// a branch per block and lets each loop body stay monomorphic.

// decodeBlockRange decodes blocks [start, end) of a tightly packed payload.
func decodeBlockRange(format Format, data, out []byte, width, height, bx, start, end int) error {
	switch format {
	case FormatDXT1:
		decodeRangeDXT1(data, out, width, height, bx, start, end)

	case FormatDXT3:
		decodeRangeDXT3(data, out, width, height, bx, start, end)

	case FormatDXT5:
		decodeRangeDXT5(data, out, width, height, bx, start, end)

	case FormatBC4:
		decodeRangeBC4(data, out, width, height, bx, start, end)

	case FormatBC5:
		decodeRangeBC5(data, out, width, height, bx, start, end)

	case FormatBC7:
		decodeRangeBC7(data, out, width, height, bx, start, end)

	default:
		return ErrUnsupportedFormat
	}

	return nil
}

// encodeBlockRange encodes blocks [start, end) into a tightly packed payload.
func encodeBlockRange(format Format, rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) error {
	switch format {
	case FormatDXT1:
		encodeRangeDXT1(rgba, out, width, height, bx, start, end, options)

	case FormatDXT3:
		encodeRangeDXT3(rgba, out, width, height, bx, start, end, options)

	case FormatDXT5:
		encodeRangeDXT5(rgba, out, width, height, bx, start, end, options)

	case FormatBC4:
		encodeRangeBC4(rgba, out, width, height, bx, start, end, options)

	case FormatBC5:
		encodeRangeBC5(rgba, out, width, height, bx, start, end, options)

	case FormatBC7:
		encodeRangeBC7(rgba, out, width, height, bx, start, end, options)

	default:
		return ErrUnsupportedFormat
	}

	return nil
}

// decodeRangeDXT1 decodes a DXT1 block range (8 bytes per block).
func decodeRangeDXT1(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeDXT1ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := decodeBlockDXT1(data[pos : pos+8])
		storeBlock(out, width, height, x, y, &block)
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeDXT3 decodes a DXT3 block range (16 bytes per block).
func decodeRangeDXT3(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeDXT3ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockDXT3(data[pos : pos+16])
		storeBlock(out, width, height, x, y, &block)
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeDXT5 decodes a DXT5 block range (16 bytes per block).
func decodeRangeDXT5(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeDXT5ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockDXT5(data[pos : pos+16])
		storeBlock(out, width, height, x, y, &block)
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC4 decodes a BC4 block range (8 bytes per block).
func decodeRangeBC4(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC4ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		alpha := decodeBlockBC4(data[pos : pos+8])
		var block [64]byte
		expandBC4Block(&block, &alpha)
		storeBlock(out, width, height, x, y, &block)
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC5 decodes a BC5 block range (16 bytes per block).
func decodeRangeBC5(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC5ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC5(data[pos : pos+16])
		storeBlock(out, width, height, x, y, &block)
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC7 encodes a BC7 block range (16 bytes per block).
func encodeRangeBC7(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC7(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC7 decodes a BC7 block range (16 bytes per block).
func decodeRangeBC7(data, out []byte, width, height, bx, start, end int) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC7(data[pos : pos+16])
		storeBlock(out, width, height, x, y, &block)
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeDXT1 encodes a DXT1 block range (8 bytes per block).
func encodeRangeDXT1(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockDXT1WithOptions(block, options)
		copy(out[pos:pos+8], b[:])
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeDXT3 encodes a DXT3 block range (16 bytes per block).
func encodeRangeDXT3(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockDXT3WithOptions(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeDXT5 encodes a DXT5 block range (16 bytes per block).
func encodeRangeDXT5(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockDXT5WithOptions(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC4 encodes a BC4 block range (8 bytes per block).
func encodeRangeBC4(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC4(block, options, bc4ChannelR)
		copy(out[pos:pos+8], b[:])
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC5 encodes a BC5 block range (16 bytes per block).
func encodeRangeBC5(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC5(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}
