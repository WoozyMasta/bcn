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

// EncodeBC5 encodes an RGBA image into BC5 blocks using red/green channels.
// Blue/alpha are ignored.
func EncodeBC5(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC5, nil)
}

// DecodeBC5 decodes BC5 blocks into an RGBA image (R/G from block, B=0, A=255).
func DecodeBC5(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC5)
}

func encodeBlockBC4(block [16]rgba8, opts EncodeOptions, channel func(rgba8) uint8) [8]byte {
	var alpha [16]uint8
	for i := 0; i < 16; i++ {
		alpha[i] = channel(block[i])
	}

	return encodeAlphaBlock(alpha, opts.Quality)
}

func decodeBlockBC4(data []byte) [16]uint8 {
	return decodeAlphaBlock(data)
}

func encodeBlockBC5(block [16]rgba8, opts EncodeOptions) [16]byte {
	var out [16]byte
	red := encodeBlockBC4(block, opts, func(c rgba8) uint8 { return c.r })
	green := encodeBlockBC4(block, opts, func(c rgba8) uint8 { return c.g })
	copy(out[0:8], red[:])
	copy(out[8:16], green[:])

	return out
}

func decodeBlockBC5(data []byte) [16]rgba8 {
	red := decodeAlphaBlock(data[0:8])
	green := decodeAlphaBlock(data[8:16])
	var out [16]rgba8
	for i := 0; i < 16; i++ {
		out[i] = rgba8{r: red[i], g: green[i], b: 0, a: 255}
	}

	return out
}

func encodeAlphaBlock(alpha [16]uint8, quality Quality) [8]byte {
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

	if quality != QualityFast {
		step := 1
		tries := 64

		if quality == QualityBest {
			tries = 256
		}

		for i := 0; i < tries; i++ {
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
		best := bestAlphaIndex(palette, alpha[i])
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

func decodeAlphaBlock(data []byte) [16]uint8 {
	a0 := data[0]
	a1 := data[1]
	palette := dxt5AlphaPalette(a0, a1)
	idx := uint64(data[2]) | uint64(data[3])<<8 | uint64(data[4])<<16 | uint64(data[5])<<24 | uint64(data[6])<<32 | uint64(data[7])<<40

	var out [16]uint8
	for i := 0; i < 16; i++ {
		// #nosec G115 -- masked to 0..7 before conversion.
		pi := int(idx & 0x7)
		if pi > 7 {
			pi = 7
		}
		out[i] = palette[pi]
		idx >>= 3
	}

	return out
}

func alphaBlockError(alpha [16]uint8, a0, a1 uint8) int {
	palette := dxt5AlphaPalette(a0, a1)
	err := 0
	for i := 0; i < 16; i++ {
		// #nosec G602 -- bestAlphaIndex returns 0..7.
		best := palette[int(bestAlphaIndex(palette, alpha[i]))]
		derr := int(alpha[i]) - int(best)
		err += derr * derr
	}

	return err
}
