// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 (BPTC unorm) decoding.
// The block decoder is a faithful port of bcdec_bc7 from bcdec.h (MIT/Unlicense, (c) 2022 Sergii Kudlai).
// Encoding is added separately.

// DecodeBC7 decodes BC7 blocks into an RGBA image (NRGBA layout).
func DecodeBC7(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC7)
}

// DecodeBC7WithOptions decodes BC7 blocks with explicit options.
func DecodeBC7WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC7, opts)
}

// decodeBlockBC7 decodes one BC7 block (16 bytes) into 16 NRGBA pixels
// laid out as 4 rows of 16 bytes.
// An unrecognized mode yields transparent black, as in the reference decoder.
func decodeBlockBC7(data []byte) [64]byte {
	var out [64]byte
	r := newBPTCReader(data)

	// Mode is unary: the count of leading zero bits before the first set bit.
	mode := 0
	for mode < 8 && r.readBit() == 0 {
		mode++
	}
	if mode >= 8 {
		return out // transparent black
	}

	partition := 0
	numPartitions := 1
	rotation := 0
	indexSelectionBit := uint32(0)

	switch mode {
	case 0, 1, 2, 3, 7:
		if mode == 0 || mode == 2 {
			numPartitions = 3
		} else {
			numPartitions = 2
		}
		if mode == 0 {
			partition = int(r.read(4))
		} else {
			partition = int(r.read(6))
		}
	}

	numEndpoints := numPartitions * 2

	if mode == 4 || mode == 5 {
		rotation = int(r.read(2))
		if mode == 4 {
			indexSelectionBit = r.readBit()
		}
	}

	var endpoints [6][4]int32

	colorBits := bc7ColorBits[mode]
	for i := range 3 {
		for j := range numEndpoints {
			endpoints[j][i] = r.readN(colorBits)
		}
	}

	alphaBits := bc7AlphaBits[mode]
	if alphaBits > 0 {
		for j := range numEndpoints {
			endpoints[j][3] = r.readN(alphaBits)
		}
	}

	hasPBits := bc7ModeHasPBits&(1<<uint(mode)) != 0
	if mode == 0 || mode == 1 || mode == 3 || mode == 6 || mode == 7 {
		for i := range numEndpoints {
			for k := range 4 {
				endpoints[i][k] <<= 1
			}
		}

		if mode == 1 {
			// Shared P-bit: one bit for endpoints 0/1, one for 2/3 (RGB only).
			pi := r.readN(1)
			pj := r.readN(1)
			for k := range 3 {
				endpoints[0][k] |= pi
				endpoints[1][k] |= pi
				endpoints[2][k] |= pj
				endpoints[3][k] |= pj
			}
		} else if hasPBits {
			// Unique P-bit per endpoint, applied to all four components.
			for i := range numEndpoints {
				p := r.readN(1)
				for k := range 4 {
					endpoints[i][k] |= p
				}
			}
		}
	}

	// Expand each endpoint component to 8 bits by left-aligning its MSB
	// and replicating the high bits into the freed low bits.
	pbit := 0
	if hasPBits {
		pbit = 1
	}
	colorPrec := colorBits + pbit
	alphaPrec := alphaBits + pbit
	for i := range numEndpoints {
		for k := range 3 {
			v := endpoints[i][k] << uint(8-colorPrec)
			endpoints[i][k] = v | (v >> uint(colorPrec))
		}
		a := endpoints[i][3] << uint(8-alphaPrec)
		endpoints[i][3] = a | (a >> uint(alphaPrec))
	}
	if alphaBits == 0 {
		for j := range numEndpoints {
			endpoints[j][3] = 0xFF
		}
	}

	indexBits := 2
	switch mode {
	case 0, 1:
		indexBits = 3
	case 6:
		indexBits = 4
	}

	indexBits2 := 0
	switch mode {
	case 4:
		indexBits2 = 3
	case 5:
		indexBits2 = 2
	}

	var weights, weights2 []int32
	switch indexBits {
	case 2:
		weights = bc7Weight2[:]
	case 3:
		weights = bc7Weight3[:]
	default:
		weights = bc7Weight4[:]
	}

	if indexBits2 == 2 {
		weights2 = bc7Weight2[:]
	} else {
		weights2 = bc7Weight3[:]
	}

	// Pass 1: read all primary (color) indices.
	// Indices are not interleaved with the secondary set,
	// so two passes over the texels are required.
	var indices [16]int32
	for i := range 4 {
		for j := range 4 {
			ps := bc7PartitionAt(numPartitions, partition, i, j)
			ib := indexBits
			if ps&0x80 != 0 {
				ib-- // anchor index stored with one less bit
			}
			indices[i*4+j] = r.readN(ib)
		}
	}

	// Pass 2: read secondary indices (if any), interpolate, and apply rotation.
	for i := range 4 {
		for j := range 4 {
			ps := bc7PartitionAt(numPartitions, partition, i, j)
			sub := int(ps & 0x03)
			idx := int(indices[i*4+j])
			e0 := &endpoints[sub*2]
			e1 := &endpoints[sub*2+1]

			var cr, cg, cb, ca uint8
			if indexBits2 == 0 {
				cr = bc7Interpolate(e0[0], e1[0], weights, idx)
				cg = bc7Interpolate(e0[1], e1[1], weights, idx)
				cb = bc7Interpolate(e0[2], e1[2], weights, idx)
				ca = bc7Interpolate(e0[3], e1[3], weights, idx)
			} else {
				ib2 := indexBits2
				if i|j == 0 {
					ib2--
				}
				idx2 := int(r.read(ib2))
				if indexSelectionBit == 0 {
					cr = bc7Interpolate(e0[0], e1[0], weights, idx)
					cg = bc7Interpolate(e0[1], e1[1], weights, idx)
					cb = bc7Interpolate(e0[2], e1[2], weights, idx)
					ca = bc7Interpolate(e0[3], e1[3], weights2, idx2)
				} else {
					cr = bc7Interpolate(e0[0], e1[0], weights2, idx2)
					cg = bc7Interpolate(e0[1], e1[1], weights2, idx2)
					cb = bc7Interpolate(e0[2], e1[2], weights2, idx2)
					ca = bc7Interpolate(e0[3], e1[3], weights, idx)
				}
			}

			switch rotation {
			case 1:
				ca, cr = cr, ca
			case 2:
				ca, cg = cg, ca
			case 3:
				ca, cb = cb, ca
			}

			o := (i*4 + j) * 4
			out[o+0] = cr
			out[o+1] = cg
			out[o+2] = cb
			out[o+3] = ca
		}
	}

	return out
}

// bc7PartitionAt returns the partition/anchor byte for texel (i, j).
// With a single subset only texel (0,0) is the anchor;
// otherwise the value comes from the partition table
// (the 0x80 bit marks anchors, the low 2 bits the subset).
func bc7PartitionAt(numPartitions, partition, i, j int) uint8 {
	if numPartitions == 1 {
		if i|j == 0 {
			return 128
		}
		return 0
	}

	return bc7PartitionSets[numPartitions-2][partition][i*4+j]
}
