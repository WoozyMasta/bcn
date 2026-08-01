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

// EncodeBC4S encodes normalized RGBA into signed BC4 blocks using the red channel.
// Input values map from 0..255 to the signed normalized range -1..1.
func EncodeBC4S(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC4S, nil)
}

// DecodeBC4S decodes signed BC4 blocks into normalized RGBA (R replicated, A=255).
// Output values map the signed normalized range -1..1 to 0..255.
func DecodeBC4S(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC4S)
}

// DecodeBC4SWithOptions decodes signed BC4 blocks with explicit options.
func DecodeBC4SWithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC4S, opts)
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

// EncodeBC5S encodes normalized RGBA into signed BC5 blocks using red/green channels.
// Input values map from 0..255 to the signed normalized range -1..1.
func EncodeBC5S(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC5S, nil)
}

// DecodeBC5S decodes signed BC5 blocks into normalized RGBA.
// Output R/G map the signed normalized range -1..1 to 0..255; B=0 and A=255.
func DecodeBC5S(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC5S)
}

// DecodeBC5SWithOptions decodes signed BC5 blocks with explicit options.
func DecodeBC5SWithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC5S, opts)
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
	return encodeAlphaBlock(alpha, settings.alphaTries, settings.lsqIters)
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

// encodeAlphaBlock packs 16 alpha samples into BC3/BC4 alpha layout (8 bytes).
func encodeAlphaBlock(alpha [16]uint8, alphaTries, lsqIters int) [8]byte {
	// BC4/BC5 use the same 8-byte alpha block layout as BC3 alpha.
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

	if lsqIters > 0 {
		bestA0, bestA1 = lsqAlphaRefine(alpha, bestA0, bestA1, lsqIters)
	}

	idx := packAlphaIndices(bestA0, bestA1, &alpha)

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
// for 16 alpha samples (sample i at bit 3i) against the palette
// of endpoints a0, a1, using the AVX2 kernel when available.
func packAlphaIndices(a0, a1 uint8, alpha *[16]uint8) uint64 {
	if idx, ok := bestAlphaIndices16ASM(alpha, a0, a1); ok {
		return idx
	}

	palette := bc3AlphaPalette(a0, a1)
	var idx uint64
	for i := 15; i >= 0; i-- {
		best := bestAlphaIndex(&palette, alpha[i])
		idx = (idx << 3) | uint64(best&0x7)
		if i == 0 {
			break
		}
	}

	return idx
}

// decodeAlphaBlock unpacks one BC3/BC4 alpha payload to 16 samples.
func decodeAlphaBlock(data []byte) [16]uint8 {
	a0 := data[0]
	a1 := data[1]
	palette := bc3AlphaPalette(a0, a1)
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
// As with bc1BlockError, callers compare with strict <, so both select the same winner.
func alphaBlockError(alpha [16]uint8, a0, a1 uint8, cutoff int) int {
	if e, ok := alphaBlockErrorASM(&alpha, a0, a1); ok {
		return e
	}

	palette := bc3AlphaPalette(a0, a1)
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

// snormFromU8 maps an NRGBA component to the BC4/BC5 signed normalized range.
func snormFromU8(v byte) int {
	return (int(v)*254+127)/255 - 127
}

// u8FromSNORM maps a BC4/BC5 signed normalized value to an NRGBA component.
func u8FromSNORM(v int) byte {
	v = clampSNORM(v)
	v += 127
	return byte(v + (v+129)>>8) // #nosec G115 -- clamped result maps exactly to [0, 255].
}

// clampSNORM clamps an integer to the representable BC4/BC5 SNORM range.
func clampSNORM(v int) int {
	return min(max(v, -127), 127)
}

// signedAlphaPalette builds the eight-entry BC4/BC5 SNORM interpolation palette.
func signedAlphaPalette(a0, a1 int) [8]int {
	p := [8]int{a0, a1}
	if a0 > a1 {
		p[2] = (56173*a0 + 9363*a1 + 32768) >> 16
		p[3] = (46812*a0 + 18724*a1 + 32768) >> 16
		p[4] = (37450*a0 + 28086*a1 + 32768) >> 16
		p[5] = (28086*a0 + 37450*a1 + 32768) >> 16
		p[6] = (18724*a0 + 46812*a1 + 32768) >> 16
		p[7] = (9363*a0 + 56173*a1 + 32768) >> 16
	} else {
		p[2] = (52429*a0 + 13107*a1 + 32768) >> 16
		p[3] = (39321*a0 + 26215*a1 + 32768) >> 16
		p[4] = (26215*a0 + 39321*a1 + 32768) >> 16
		p[5] = (13107*a0 + 52429*a1 + 32768) >> 16
		p[6] = -127
		p[7] = 127
	}

	return p
}

// signedAlphaIndex returns the nearest signed alpha palette index and its squared error.
func signedAlphaIndex(palette *[8]int, value int) (int, int) {
	best, bestErr := 0, int(^uint(0)>>1)
	for i := range 8 {
		delta := value - palette[i]
		err := delta * delta
		if err < bestErr {
			best, bestErr = i, err
		}
	}

	return best, bestErr
}

// signedAlphaBlockError returns total squared error for a signed alpha endpoint pair.
// It stops once the accumulated error reaches cutoff.
func signedAlphaBlockError(samples *[16]int, a0, a1, cutoff int) int {
	palette := signedAlphaPalette(a0, a1)
	err := 0

	for _, sample := range samples {
		_, bestErr := signedAlphaIndex(&palette, sample)
		err += bestErr
		if err >= cutoff {
			return err
		}
	}

	return err
}

// encodeSignedAlphaBlock encodes sixteen SNORM samples into one BC4 alpha block.
func encodeSignedAlphaBlock(samples [16]int, alphaTries int) [8]byte {
	minV, maxV := samples[0], samples[0]
	for i := 1; i < len(samples); i++ {
		minV = min(minV, samples[i])
		maxV = max(maxV, samples[i])
	}

	a0, a1 := maxV, minV
	if a0 == a1 {
		if a0 > -127 {
			a1 = a0 - 1
		} else {
			a0 = a1 + 1
		}
	}

	bestA0, bestA1 := a0, a1
	bestErr := signedAlphaBlockError(&samples, a0, a1, 1<<62)

	for i := range alphaTries {
		cand0 := clampSNORM(a0 + (i%3 - 1))
		cand1 := clampSNORM(a1 + ((i/3)%3 - 1))
		if cand0 <= cand1 {
			continue
		}

		if err := signedAlphaBlockError(&samples, cand0, cand1, bestErr); err < bestErr {
			bestErr, bestA0, bestA1 = err, cand0, cand1
		}
	}

	palette := signedAlphaPalette(bestA0, bestA1)
	var idx uint64
	for i := len(samples) - 1; i >= 0; i-- {
		nearest, _ := signedAlphaIndex(&palette, samples[i])
		idx = (idx << 3) | uint64(nearest) // #nosec G115 -- nearest is an index in [0, 7].
	}

	var out [8]byte
	out[0] = byte(int8(bestA0)) // #nosec G115 -- endpoint is clamped to int8 SNORM range.
	out[1] = byte(int8(bestA1)) // #nosec G115 -- endpoint is clamped to int8 SNORM range.
	putAlphaIndices(out[2:], idx)
	return out
}

// decodeSignedAlphaBlock decodes one BC4 SNORM alpha block into signed samples.
func decodeSignedAlphaBlock(data []byte) [16]int {
	a0 := int(int8(data[0])) // #nosec G115 -- reinterpret raw SNORM endpoint as int8.
	a1 := int(int8(data[1])) // #nosec G115 -- reinterpret raw SNORM endpoint as int8.

	a0 = max(a0, -127)
	a1 = max(a1, -127)

	palette := signedAlphaPalette(a0, a1)
	idx := uint64(data[2]) |
		uint64(data[3])<<8 |
		uint64(data[4])<<16 |
		uint64(data[5])<<24 |
		uint64(data[6])<<32 |
		uint64(data[7])<<40

	var out [16]int
	for i := range out {
		out[i] = palette[idx&0x7]
		idx >>= 3
	}

	return out
}

// encodeBlockBC4S encodes one normalized NRGBA channel as a signed BC4 block.
func encodeBlockBC4S(block [16]rgba8, opts EncodeOptions, channel bc4Channel) [8]byte {
	var samples [16]int
	for i := range block {
		if channel == bc4ChannelG {
			samples[i] = snormFromU8(block[i].g)
		} else {
			samples[i] = snormFromU8(block[i].r)
		}
	}

	return encodeSignedAlphaBlock(samples, qualitySettingsForOpts(opts).alphaTries)
}

// decodeBlockBC4S decodes one signed BC4 block into normalized NRGBA pixels.
func decodeBlockBC4S(data []byte) [64]byte {
	samples := decodeSignedAlphaBlock(data)
	var out [64]byte
	for i := range samples {
		value := u8FromSNORM(samples[i])
		out[i*4+0] = value
		out[i*4+1] = value
		out[i*4+2] = value
		out[i*4+3] = 255
	}

	return out
}

// encodeBlockBC5S encodes red and green normalized NRGBA channels as signed BC5.
func encodeBlockBC5S(block [16]rgba8, opts EncodeOptions) [16]byte {
	var out [16]byte

	red := encodeBlockBC4S(block, opts, bc4ChannelR)
	green := encodeBlockBC4S(block, opts, bc4ChannelG)
	copy(out[0:8], red[:])
	copy(out[8:16], green[:])

	return out
}

// decodeBlockBC5S decodes one signed BC5 block into normalized NRGBA pixels.
func decodeBlockBC5S(data []byte) [64]byte {
	red := decodeSignedAlphaBlock(data[0:8])
	green := decodeSignedAlphaBlock(data[8:16])
	var out [64]byte
	for i := range red {
		out[i*4+0] = u8FromSNORM(red[i])
		out[i*4+1] = u8FromSNORM(green[i])
		out[i*4+2] = 0
		out[i*4+3] = 255
	}

	return out
}
