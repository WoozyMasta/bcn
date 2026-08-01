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
	case FormatBC1:
		decodeRangeBC1(data, out, width, height, bx, start, end)

	case FormatBC2:
		decodeRangeBC2(data, out, width, height, bx, start, end)

	case FormatBC3:
		decodeRangeBC3(data, out, width, height, bx, start, end)

	case FormatBC4:
		decodeRangeBC4(data, out, width, height, bx, start, end)

	case FormatBC5:
		decodeRangeBC5(data, out, width, height, bx, start, end)

	case FormatBC4S:
		decodeRangeBC4S(data, out, width, height, bx, start, end)

	case FormatBC5S:
		decodeRangeBC5S(data, out, width, height, bx, start, end)

	case FormatBC7:
		decodeRangeBC7(data, out, width, height, bx, start, end)

	case FormatBC6HU, FormatBC6HS:
		return ErrBC6HUsesHDRAPI

	default:
		return ErrUnsupportedFormat
	}

	return nil
}

// encodeBlockRange encodes blocks [start, end) into a tightly packed payload.
func encodeBlockRange(format Format, rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) error {
	switch format {
	case FormatBC1:
		encodeRangeBC1(rgba, out, width, height, bx, start, end, options)

	case FormatBC2:
		encodeRangeBC2(rgba, out, width, height, bx, start, end, options)

	case FormatBC3:
		encodeRangeBC3(rgba, out, width, height, bx, start, end, options)

	case FormatBC4:
		encodeRangeBC4(rgba, out, width, height, bx, start, end, options)

	case FormatBC5:
		encodeRangeBC5(rgba, out, width, height, bx, start, end, options)

	case FormatBC4S:
		encodeRangeBC4S(rgba, out, width, height, bx, start, end, options)

	case FormatBC5S:
		encodeRangeBC5S(rgba, out, width, height, bx, start, end, options)

	case FormatBC7:
		encodeRangeBC7(rgba, out, width, height, bx, start, end, options)

	case FormatBC6HU, FormatBC6HS:
		return ErrBC6HUsesHDRAPI

	default:
		return ErrUnsupportedFormat
	}

	return nil
}

// decodeRangeBC1 decodes a BC1 block range (8 bytes per block).
func decodeRangeBC1(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC1ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := decodeBlockBC1(data[pos : pos+8])
		storeBlock(out, width, height, x, y, &block)
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC2 decodes a BC2 block range (16 bytes per block).
func decodeRangeBC2(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC2ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC2(data[pos : pos+16])
		storeBlock(out, width, height, x, y, &block)
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC3 decodes a BC3 block range (16 bytes per block).
func decodeRangeBC3(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC3ASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC3(data[pos : pos+16])
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

// decodeRangeBC4S decodes signed BC4 blocks.
func decodeRangeBC4S(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC4SASM(data, out, width, height, bx, start, end) {
		return
	}

	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := decodeBlockBC4S(data[pos : pos+8])
		storeBlock(out, width, height, x, y, &block)
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// decodeRangeBC5S decodes signed BC5 blocks.
func decodeRangeBC5S(data, out []byte, width, height, bx, start, end int) {
	if decodeRangeBC5SASM(data, out, width, height, bx, start, end) {
		return
	}
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC5S(data[pos : pos+16])
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

// encodeRangeBC1 encodes a BC1 block range (8 bytes per block).
func encodeRangeBC1(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC1WithOptions(block, options)
		copy(out[pos:pos+8], b[:])
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC2 encodes a BC2 block range (16 bytes per block).
func encodeRangeBC2(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC2WithOptions(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC3 encodes a BC3 block range (16 bytes per block).
func encodeRangeBC3(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC3WithOptions(block, options)
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

// encodeRangeBC4S encodes signed BC4 blocks.
func encodeRangeBC4S(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 8

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC4S(block, options, bc4ChannelR)
		copy(out[pos:pos+8], b[:])
		pos += 8

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// encodeRangeBC5S encodes signed BC5 blocks.
func encodeRangeBC5S(rgba, out []byte, width, height, bx, start, end int, options EncodeOptions) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := extractBlock(rgba, width, height, x, y)
		b := encodeBlockBC5S(block, options)
		copy(out[pos:pos+16], b[:])
		pos += 16

		x++
		if x == bx {
			x = 0
			y++
		}
	}
}
