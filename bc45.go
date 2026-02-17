// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// EncodeBC4 encodes an RGBA image into BC4 blocks using the red channel.
// Other channels are ignored.
func EncodeBC4(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC4, nil)
}

// DecodeBC4 decodes BC4 blocks into an RGBA image (R replicated, A=255).
func DecodeBC4(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC4)
}

// DecodeBC4WithOptions decodes BC4 blocks with explicit options.
func DecodeBC4WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC4, opts)
}

// EncodeBC5 encodes an RGBA image into BC5 blocks using red/green channels.
// Blue/alpha are ignored.
func EncodeBC5(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC5, nil)
}

// DecodeBC5 decodes BC5 blocks into an RGBA image (R/G from block, B=0, A=255).
func DecodeBC5(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC5)
}

// DecodeBC5WithOptions decodes BC5 blocks with explicit options.
func DecodeBC5WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC5, opts)
}

// encodeBlockBC4 encodes one 4x4 block using a selected channel source.
func encodeBlockBC4(block [16]rgba8, opts EncodeOptions, channel func(rgba8) uint8) [8]byte {
	var alpha [16]uint8

	blockView := block[:]
	alphaView := alpha[:]
	for len(alphaView) > 0 {
		alphaView[0] = channel(blockView[0])
		alphaView = alphaView[1:]
		blockView = blockView[1:]
	}

	settings := qualitySettingsForOpts(opts)
	return encodeAlphaBlock(alpha, settings.alphaTries)
}

// decodeBlockBC4 decodes one BC4 block into 16 scalar samples.
func decodeBlockBC4(data []byte) [16]uint8 {
	return decodeAlphaBlock(data)
}

// encodeBlockBC5 encodes BC5 as two BC4 blocks (R then G).
func encodeBlockBC5(block [16]rgba8, opts EncodeOptions) [16]byte {
	var out [16]byte
	red := encodeBlockBC4(block, opts, func(c rgba8) uint8 { return c.r })
	green := encodeBlockBC4(block, opts, func(c rgba8) uint8 { return c.g })
	copy(out[0:8], red[:])
	copy(out[8:16], green[:])

	return out
}

// decodeBlockBC5 decodes BC5 into RG with fixed B=0 and A=255.
func decodeBlockBC5(data []byte) [16]rgba8 {
	red := decodeAlphaBlock(data[0:8])
	green := decodeAlphaBlock(data[8:16])
	var out [16]rgba8

	redView := red[:]
	greenView := green[:]
	outView := out[:]
	for len(outView) > 0 {
		outView[0] = rgba8{r: redView[0], g: greenView[0], b: 0, a: 255}
		outView = outView[1:]
		redView = redView[1:]
		greenView = greenView[1:]
	}

	return out
}

// encodeAlphaBlock packs 16 alpha samples into DXT5/BC4 alpha layout (8 bytes).
func encodeAlphaBlock(alpha [16]uint8, alphaTries int) [8]byte {
	// BC4/BC5 use the same 8-byte alpha block layout as DXT5 alpha.
	minA, maxA := alpha[0], alpha[0]
	for i := 1; i < 16; i++ {
		if alpha[i] < minA {
			minA = alpha[i]
		}
		if alpha[i] > maxA {
			maxA = alpha[i]
		}
	}

	a0, a1 := maxA, minA
	if a0 == a1 {
		if a0 > 0 {
			a1 = a0 - 1
		} else {
			a1 = 1
		}
	}
	bestA0, bestA1 := a0, a1
	bestErr := alphaBlockError(alpha, bestA0, bestA1)

	if alphaTries > 0 && bestErr != 0 {
		step := 1
		tries := alphaTries

		for i := range tries {
			cand0 := clampU8(int(a0) + (i%3-1)*step)
			cand1 := clampU8(int(a1) + ((i/3)%3-1)*step)
			err := alphaBlockError(alpha, cand0, cand1)
			if err < bestErr {
				bestErr = err
				bestA0 = cand0
				bestA1 = cand1
			}
		}
	}

	palette := dxt5AlphaPalette(bestA0, bestA1)
	var idx uint64
	for i := 15; i >= 0; i-- {
		best := bestAlphaIndex(&palette, alpha[i])
		idx = (idx << 3) | uint64(best&0x7)
		if i == 0 {
			break
		}
	}

	var out [8]byte
	out[0] = bestA0
	out[1] = bestA1
	out[2] = byte(idx)
	out[3] = byte(idx >> 8)
	out[4] = byte(idx >> 16)
	out[5] = byte(idx >> 24)
	out[6] = byte(idx >> 32)
	out[7] = byte(idx >> 40)

	return out
}

// decodeAlphaBlock unpacks one DXT5/BC4 alpha payload to 16 samples.
func decodeAlphaBlock(data []byte) [16]uint8 {
	a0 := data[0]
	a1 := data[1]
	palette := dxt5AlphaPalette(a0, a1)
	idx := uint64(data[2]) | uint64(data[3])<<8 | uint64(data[4])<<16 | uint64(data[5])<<24 | uint64(data[6])<<32 | uint64(data[7])<<40

	var out [16]uint8
	for i := range 16 {
		// #nosec G115 -- masked to 0..7 before conversion.
		pi := min(int(idx&0x7), 7)
		out[i] = palette[pi]
		idx >>= 3
	}

	return out
}

// alphaBlockError computes total squared error for a candidate alpha endpoint pair.
func alphaBlockError(alpha [16]uint8, a0, a1 uint8) int {
	palette := dxt5AlphaPalette(a0, a1)
	err := 0

	for _, a := range alpha {
		_, bestErr := bestAlphaIndexErr(&palette, a)
		err += bestErr
	}

	return err
}
