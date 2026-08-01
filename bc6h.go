// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

// BC6H (HDR RGB, half-float) decoder.
// Block decoder ported from bcdec_bc6h_half in bcdec.h
// (MIT/Unlicense, (c) 2022 Sergii Kudlai).
// https://github.com/iOrange/bcdec

package bcn

import (
	"runtime"
	"sync"
)

// DecodeBC6H decodes BC6H-compressed data into a flat []uint16 of RGB half-float pixels.
// Layout: width*height*3 uint16 values in row-major order (R, G, B per texel).
// signed selects BC6H_SF16 (true) or BC6H_UF16 (false).
func DecodeBC6H(data []byte, width, height int, signed bool) ([]uint16, error) {
	return decodeBlocksBC6H(data, width, height, signed, nil)
}

// DecodeBC6HWithOptions is DecodeBC6H with explicit decode options.
func DecodeBC6HWithOptions(data []byte, width, height int, signed bool, opts *DecodeOptions) ([]uint16, error) {
	return decodeBlocksBC6H(data, width, height, signed, opts)
}

// DecodeBC6HFloat32 decodes BC6H data into a flat []float32 of RGB pixels.
func DecodeBC6HFloat32(data []byte, width, height int, signed bool) ([]float32, error) {
	return DecodeBC6HFloat32WithOptions(data, width, height, signed, nil)
}

// DecodeBC6HFloat32WithOptions is DecodeBC6HFloat32 with explicit decode options.
func DecodeBC6HFloat32WithOptions(data []byte, width, height int, signed bool, opts *DecodeOptions) ([]float32, error) {
	h, err := decodeBlocksBC6H(data, width, height, signed, opts)
	if err != nil {
		return nil, err
	}
	f := make([]float32, len(h))
	for i, v := range h {
		f[i] = float16ToFloat32(v)
	}
	return f, nil
}

// decodeBlocksBC6H is the internal parallel dispatcher for BC6H decoding.
func decodeBlocksBC6H(data []byte, width, height int, signed bool, opts *DecodeOptions) ([]uint16, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	bx := (width + 3) / 4
	by := (height + 3) / 4
	totalBlocks := bx * by
	if len(data) < totalBlocks*16 {
		return nil, ErrInsufficientData
	}
	_ = by

	out := make([]uint16, width*height*3)

	workers := runtime.GOMAXPROCS(0)
	if opts != nil && opts.Workers > 0 {
		workers = opts.Workers
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	if workers <= 1 || totalBlocks < 256*workers {
		decodeRangeBC6H(data, out, width, height, bx, 0, totalBlocks, signed)
		return out, nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		start := (totalBlocks * w) / workers
		end := (totalBlocks * (w + 1)) / workers
		go func(s, e int) {
			defer wg.Done()
			decodeRangeBC6H(data, out, width, height, bx, s, e, signed)
		}(start, end)
	}
	wg.Wait()

	return out, nil
}

// decodeRangeBC6H decodes blocks [start, end) into out.
func decodeRangeBC6H(data []byte, out []uint16, width, height, bx, start, end int, signed bool) {
	x := start % bx
	y := start / bx
	pos := start * 16

	for range end - start {
		block := decodeBlockBC6H(data[pos:pos+16], signed)
		storeBlockHDR(out, width, height, x, y, &block)
		pos += 16
		x++
		if x == bx {
			x = 0
			y++
		}
	}
}

// storeBlockHDR writes a decoded 4x4 block (48 uint16, RGB row-major) into dst.
func storeBlockHDR(dst []uint16, width, height, bx, by int, block *[48]uint16) {
	baseX := bx * 4
	baseY := by * 4
	for row := range 4 {
		py := baseY + row
		if py >= height {
			break
		}

		for col := range 4 {
			px := baseX + col
			if px >= width {
				break
			}

			texel := row*4 + col
			off := (py*width + px) * 3
			dst[off+0] = block[texel*3+0]
			dst[off+1] = block[texel*3+1]
			dst[off+2] = block[texel*3+2]
		}
	}
}

// bptcReadBitsR reads n bits and returns them with bit order reversed,
// as required by BC6H modes 13-14 for the high endpoint bits.
func bptcReadBitsR(r *bptcReader, n int) uint32 {
	bits := r.read(n)
	var result uint32
	for range n {
		result = (result << 1) | (bits & 1)
		bits >>= 1
	}

	return result
}

// bc6hExtendSign sign-extends val from bits to a full int.
func bc6hExtendSign(val, bits int) int {
	m := 1 << uint(bits-1)
	return (val ^ m) - m
}

// bc6hTransformInverse applies the BC6H delta transform.
func bc6hTransformInverse(val, a0, bits int, signed bool) int {
	val = (val + a0) & ((1 << uint(bits)) - 1)
	if signed {
		val = bc6hExtendSign(val, bits)
	}

	return val
}

// bc6hUnquantize expands a quantized endpoint to 16-bit range.
func bc6hUnquantize(val, bits int, signed bool) int {
	if !signed {
		if bits >= 15 {
			return val
		}
		if val == 0 {
			return 0
		}
		if val == (1<<uint(bits))-1 {
			return 0xFFFF
		}

		return ((val << 16) + 0x8000) >> uint(bits)
	}

	if bits >= 16 {
		return val
	}

	s := 0
	if val < 0 {
		s = 1
		val = -val
	}
	var unq int
	switch {
	case val == 0:
		unq = 0
	case val >= (1<<uint(bits-1))-1:
		unq = 0x7FFF
	default:
		unq = ((val << 15) + 0x4000) >> uint(bits-1)
	}
	if s != 0 {
		unq = -unq
	}

	return unq
}

// bc6hInterpolate linearly interpolates between a and b using the weight table.
func bc6hInterpolate(a, b int, weights []int, index int) int {
	return (a*(64-weights[index]) + b*weights[index] + 32) >> 6
}

// bc6hFinishUnquantize converts the interpolated 16-bit value to a half-float bit pattern.
func bc6hFinishUnquantize(val int, signed bool) uint16 {
	if !signed {
		return uint16(((val * 31) >> 6) & 0xFFFF)
	}

	if val < 0 {
		val = -((-val * 31) >> 5)
	} else {
		val = (val * 31) >> 5
	}
	var s uint16
	if val < 0 {
		s = 0x8000
		val = -val
	}

	return s | uint16(val&0x7FFF)
}

// decodeBlockBC6H decodes one BC6H block (16 bytes) into 16 RGB texels (48 uint16).
// Output layout: texels in row-major order, three uint16 per texel (R, G, B).
// signed selects BC6H_SF16 (true) or BC6H_UF16 (false).
func decodeBlockBC6H(data []byte, signed bool) [48]uint16 {
	var out [48]uint16
	r := newBPTCReader(data)

	var rr, gg, bb [4]int

	mode := int(r.read(2))
	if mode > 1 {
		mode |= int(r.read(3)) << 2
	}

	partition := 0

	switch mode {
	case 0: // spec mode 1: 10.555, 10.555, 10.555 -- 2-subset, 3-bit indices
		gg[2] |= int(r.readBit()) << 4
		bb[2] |= int(r.readBit()) << 4
		bb[3] |= int(r.readBit()) << 4
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(5))
		gg[3] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(5))
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 0

	case 1: // spec mode 2: 7.666, 7.666, 7.666 -- 2-subset, 3-bit indices
		gg[2] |= int(r.readBit()) << 5
		gg[3] |= int(r.readBit()) << 4
		gg[3] |= int(r.readBit()) << 5
		rr[0] |= int(r.read(7))
		bb[3] |= int(r.readBit())
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(7))
		bb[2] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 2
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(7))
		bb[3] |= int(r.readBit()) << 3
		bb[3] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(6))
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(6))
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(6))
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(6))
		rr[3] |= int(r.read(6))
		partition = int(r.read(5))
		mode = 1

	case 2: // spec mode 3: 11.555, 11.444, 11.444 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(5))
		rr[0] |= int(r.readBit()) << 10
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(4))
		gg[0] |= int(r.readBit()) << 10
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(4))
		bb[0] |= int(r.readBit()) << 10
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 2

	case 6: // spec mode 4: 11.444, 11.555, 11.444 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(4))
		rr[0] |= int(r.readBit()) << 10
		gg[3] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(5))
		gg[0] |= int(r.readBit()) << 10
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(4))
		bb[0] |= int(r.readBit()) << 10
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(4))
		bb[3] |= int(r.readBit())
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(4))
		gg[2] |= int(r.readBit()) << 4
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 3

	case 10: // spec mode 5: 11.444, 11.444, 11.555 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(4))
		rr[0] |= int(r.readBit()) << 10
		bb[2] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(4))
		gg[0] |= int(r.readBit()) << 10
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(5))
		bb[0] |= int(r.readBit()) << 10
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(4))
		bb[3] |= int(r.readBit()) << 1
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(4))
		bb[3] |= int(r.readBit()) << 4
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 4

	case 14: // spec mode 6: 9.555, 9.555, 9.555 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(9))
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(9))
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(9))
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(5))
		gg[3] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(5))
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 5

	case 18: // spec mode 7: 8.666, 8.555, 8.555 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(8))
		gg[3] |= int(r.readBit()) << 4
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(8))
		bb[3] |= int(r.readBit()) << 2
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(8))
		bb[3] |= int(r.readBit()) << 3
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(6))
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(5))
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(6))
		rr[3] |= int(r.read(6))
		partition = int(r.read(5))
		mode = 6

	case 22: // spec mode 8: 8.555, 8.666, 8.555 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(8))
		bb[3] |= int(r.readBit())
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(8))
		gg[2] |= int(r.readBit()) << 5
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(8))
		gg[3] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(5))
		gg[3] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(6))
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 7

	case 26: // spec mode 9: 8.555, 8.555, 8.666 -- 2-subset, 3-bit indices
		rr[0] |= int(r.read(8))
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(8))
		bb[2] |= int(r.readBit()) << 5
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(8))
		bb[3] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(5))
		gg[3] |= int(r.readBit()) << 4
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(5))
		bb[3] |= int(r.readBit())
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(6))
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 2
		rr[3] |= int(r.read(5))
		bb[3] |= int(r.readBit()) << 3
		partition = int(r.read(5))
		mode = 8

	case 30: // spec mode 10: 6.666, 6.666, 6.666 -- 2-subset, 3-bit indices, not transformed
		rr[0] |= int(r.read(6))
		gg[3] |= int(r.readBit()) << 4
		bb[3] |= int(r.readBit())
		bb[3] |= int(r.readBit()) << 1
		bb[2] |= int(r.readBit()) << 4
		gg[0] |= int(r.read(6))
		gg[2] |= int(r.readBit()) << 5
		bb[2] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 2
		gg[2] |= int(r.readBit()) << 4
		bb[0] |= int(r.read(6))
		gg[3] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 3
		bb[3] |= int(r.readBit()) << 5
		bb[3] |= int(r.readBit()) << 4
		rr[1] |= int(r.read(6))
		gg[2] |= int(r.read(4))
		gg[1] |= int(r.read(6))
		gg[3] |= int(r.read(4))
		bb[1] |= int(r.read(6))
		bb[2] |= int(r.read(4))
		rr[2] |= int(r.read(6))
		rr[3] |= int(r.read(6))
		partition = int(r.read(5))
		mode = 9

	case 3: // spec mode 11: 10.10, 10.10, 10.10 -- 1-subset, 4-bit indices, not transformed
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(10))
		gg[1] |= int(r.read(10))
		bb[1] |= int(r.read(10))
		mode = 10

	case 7: // spec mode 12: 11.9, 11.9, 11.9 -- 1-subset, 4-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(9))
		rr[0] |= int(r.readBit()) << 10
		gg[1] |= int(r.read(9))
		gg[0] |= int(r.readBit()) << 10
		bb[1] |= int(r.read(9))
		bb[0] |= int(r.readBit()) << 10
		mode = 11

	case 11: // spec mode 13: 12.8, 12.8, 12.8 -- 1-subset, 4-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(8))
		rr[0] |= int(bptcReadBitsR(&r, 2)) << 10
		gg[1] |= int(r.read(8))
		gg[0] |= int(bptcReadBitsR(&r, 2)) << 10
		bb[1] |= int(r.read(8))
		bb[0] |= int(bptcReadBitsR(&r, 2)) << 10
		mode = 12

	case 15: // spec mode 14: 16.4, 16.4, 16.4 -- 1-subset, 4-bit indices
		rr[0] |= int(r.read(10))
		gg[0] |= int(r.read(10))
		bb[0] |= int(r.read(10))
		rr[1] |= int(r.read(4))
		rr[0] |= int(bptcReadBitsR(&r, 6)) << 10
		gg[1] |= int(r.read(4))
		gg[0] |= int(bptcReadBitsR(&r, 6)) << 10
		bb[1] |= int(r.read(4))
		bb[0] |= int(bptcReadBitsR(&r, 6)) << 10
		mode = 13

	default:
		// Reserved modes (10011, 10111, 11011, 11111): return all zeros.
		return out
	}

	numPartitions := 1
	if mode >= 10 {
		numPartitions = 0
	}
	endpointCount := (numPartitions + 1) * 2

	actualBitsW := bc6hActualBitsCount[0][mode]

	if signed {
		rr[0] = bc6hExtendSign(rr[0], actualBitsW)
		gg[0] = bc6hExtendSign(gg[0], actualBitsW)
		bb[0] = bc6hExtendSign(bb[0], actualBitsW)
	}

	// mode 9 and mode 10 are non-transformed (endpoints stored directly).
	if mode != 9 && mode != 10 || signed {
		for i := 1; i < endpointCount; i++ {
			rr[i] = bc6hExtendSign(rr[i], bc6hActualBitsCount[1][mode])
			gg[i] = bc6hExtendSign(gg[i], bc6hActualBitsCount[2][mode])
			bb[i] = bc6hExtendSign(bb[i], bc6hActualBitsCount[3][mode])
		}
	}

	if mode != 9 && mode != 10 {
		for i := 1; i < endpointCount; i++ {
			rr[i] = bc6hTransformInverse(rr[i], rr[0], actualBitsW, signed)
			gg[i] = bc6hTransformInverse(gg[i], gg[0], actualBitsW, signed)
			bb[i] = bc6hTransformInverse(bb[i], bb[0], actualBitsW, signed)
		}
	}

	for i := range endpointCount {
		rr[i] = bc6hUnquantize(rr[i], actualBitsW, signed)
		gg[i] = bc6hUnquantize(gg[i], actualBitsW, signed)
		bb[i] = bc6hUnquantize(bb[i], actualBitsW, signed)
	}

	var weights []int
	indexBitsBase := 3
	if mode >= 10 {
		weights = bc6hAWeight4[:]
		indexBitsBase = 4
	} else {
		weights = bc6hAWeight3[:]
	}

	texel := 0
	for row := range 4 {
		for col := range 4 {
			var partitionSet int
			if mode >= 10 {
				if row == 0 && col == 0 {
					partitionSet = 128
				}
			} else {
				partitionSet = int(bc6hPartitionSets[partition][row*4+col])
			}

			indexBits := indexBitsBase
			if partitionSet&0x80 != 0 {
				indexBits--
			}
			subset := partitionSet & 0x01
			index := int(r.read(indexBits))
			epI := subset * 2

			out[texel*3+0] = bc6hFinishUnquantize(bc6hInterpolate(rr[epI], rr[epI+1], weights, index), signed)
			out[texel*3+1] = bc6hFinishUnquantize(bc6hInterpolate(gg[epI], gg[epI+1], weights, index), signed)
			out[texel*3+2] = bc6hFinishUnquantize(bc6hInterpolate(bb[epI], bb[epI+1], weights, index), signed)
			texel++
		}
	}

	return out
}
