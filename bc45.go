// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

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

// bc4Channel selects which pixel channel feeds a BC4 alpha block.
type bc4Channel int

const (
	bc4ChannelR bc4Channel = iota
	bc4ChannelG
)

// encodeBlockBC4 encodes one 4x4 block using a selected channel source.
func encodeBlockBC4(block [16]rgba8, opts EncodeOptions, channel bc4Channel) [8]byte {
	var alpha [16]uint8

	if channel == bc4ChannelG {
		for i := range block {
			alpha[i] = block[i].g
		}
	} else {
		for i := range block {
			alpha[i] = block[i].r
		}
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
	red := encodeBlockBC4(block, opts, bc4ChannelR)
	green := encodeBlockBC4(block, opts, bc4ChannelG)
	copy(out[0:8], red[:])
	copy(out[8:16], green[:])

	return out
}

// decodeBlockBC5 decodes BC5 into RG with fixed B=0 and A=255,
// laid out as 4 NRGBA rows of 16 bytes.
func decodeBlockBC5(data []byte) [64]byte {
	red := decodeAlphaBlock(data[0:8])
	green := decodeAlphaBlock(data[8:16])
	var out [64]byte
	for i := range 16 {
		out[i*4+0] = red[i]
		out[i*4+1] = green[i]
		out[i*4+2] = 0
		out[i*4+3] = 255
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
	bestErr := alphaBlockError(alpha, bestA0, bestA1, 1<<62)

	if alphaTries > 0 && bestErr != 0 {
		step := 1
		tries := alphaTries

		for i := range tries {
			cand0 := clampU8(int(a0) + (i%3-1)*step)
			cand1 := clampU8(int(a1) + ((i/3)%3-1)*step)
			err := alphaBlockError(alpha, cand0, cand1, bestErr)
			if err < bestErr {
				bestErr = err
				bestA0 = cand0
				bestA1 = cand1
			}
		}
	}

	palette := dxt5AlphaPalette(bestA0, bestA1)
	idx := packAlphaIndices(&palette, &alpha)

	var out [8]byte
	out[0] = bestA0
	out[1] = bestA1
	putAlphaIndices(out[2:8], idx)

	return out
}

// putAlphaIndices writes a 48-bit packed alpha index field
// as 6 little-endian bytes into dst (which must have length >= 6).
func putAlphaIndices(dst []byte, idx uint64) {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], idx)
	copy(dst[:6], tmp[:6])
}

// packAlphaIndices returns the 48-bit packed nearest-palette indices
// for 16 alpha samples (sample i at bit 3i), using the AVX2 kernel when available.
func packAlphaIndices(palette *[8]uint8, alpha *[16]uint8) uint64 {
	if idx, ok := bestAlphaIndices16ASM(alpha, palette); ok {
		return idx
	}

	var idx uint64
	for i := 15; i >= 0; i-- {
		best := bestAlphaIndex(palette, alpha[i])
		idx = (idx << 3) | uint64(best&0x7)
		if i == 0 {
			break
		}
	}

	return idx
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
// The AVX2 path returns the exact total; the scalar path stops at cutoff.
// As with dxt1BlockError, callers compare with strict <, so both select the same winner.
func alphaBlockError(alpha [16]uint8, a0, a1 uint8, cutoff int) int {
	palette := dxt5AlphaPalette(a0, a1)
	if e, ok := alphaBlockErrorASM(&alpha, &palette); ok {
		return e
	}

	return alphaBlockErrorScalar(&palette, &alpha, cutoff)
}

// alphaBlockErrorScalar is the pure-Go reference for alphaBlockError.
func alphaBlockErrorScalar(palette *[8]uint8, alpha *[16]uint8, cutoff int) int {
	err := 0

	for _, a := range alpha {
		_, bestErr := bestAlphaIndexErr(palette, a)
		err += bestErr
		if err >= cutoff {
			return err
		}
	}

	return err
}
