// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

// EncodeDXT5 encodes an RGBA image (NRGBA layout) into DXT5 blocks.
func EncodeDXT5(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT5, nil)
}

// DecodeDXT5 decodes DXT5 blocks into an RGBA image (NRGBA layout).
func DecodeDXT5(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatDXT5)
}

// DecodeDXT5WithOptions decodes DXT5 blocks with explicit options.
func DecodeDXT5WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatDXT5, opts)
}

// EncodeDXT5WithOptions encodes with explicit options.
// QualityLevel affects color endpoint selection; alpha is interpolated (BC3).
func EncodeDXT5WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT5, opts)
}

// encodeBlockDXT5WithOptions encodes one BC3/DXT5 block with interpolated alpha.
func encodeBlockDXT5WithOptions(block [16]rgba8, opts EncodeOptions) [16]byte {
	minC, maxC := findMinMax(block)

	a0 := maxC.a
	a1 := minC.a
	alphaIdx := uint64(0)
	if a0 != a1 {
		alphaPalette := dxt5AlphaPalette(a0, a1)
		for i := 15; i >= 0; i-- {
			idx := bestAlphaIndex(&alphaPalette, block[i].a)
			alphaIdx = (alphaIdx << 3) | uint64(idx&0x7)
			if i == 0 {
				break
			}
		}
	}

	c0, c1 := dxt1ColorEndpoints(block, opts)
	palette := dxt1Palette(c0, c1)
	rw, gw, bw := getRGBWeights(&opts, minC.r == maxC.r)
	indices := packDXT1IndicesWeighted(block, palette, false, opts.AlphaThreshold, rw, gw, bw)

	var out [16]byte
	out[0] = a0
	out[1] = a1
	// 48-bit alpha indices, little-endian
	out[2] = byte(alphaIdx)
	out[3] = byte(alphaIdx >> 8)
	out[4] = byte(alphaIdx >> 16)
	out[5] = byte(alphaIdx >> 24)
	out[6] = byte(alphaIdx >> 32)
	out[7] = byte(alphaIdx >> 40)
	binary.LittleEndian.PutUint16(out[8:10], c0)
	binary.LittleEndian.PutUint16(out[10:12], c1)
	binary.LittleEndian.PutUint32(out[12:16], indices)

	return out
}

// decodeBlockDXT5 decodes one BC3/DXT5 block into 16 RGBA pixels.
func decodeBlockDXT5(data []byte) [16]rgba8 {
	a0 := data[0]
	a1 := data[1]
	alphaPalette := dxt5AlphaPalette(a0, a1)
	alphaIdx := uint64(data[2]) | uint64(data[3])<<8 | uint64(data[4])<<16 | uint64(data[5])<<24 | uint64(data[6])<<32 | uint64(data[7])<<40
	c0 := binary.LittleEndian.Uint16(data[8:10])
	c1 := binary.LittleEndian.Uint16(data[10:12])
	palette := dxt1Palette(c0, c1)
	idx := binary.LittleEndian.Uint32(data[12:16])
	var out [16]rgba8
	for i := range 16 {
		// #nosec G602 -- index masked to 0..3.
		c := palette[int(idx&0x3)]
		alpha := alphaFromPalette(alphaPalette, alphaIdx)
		c.a = alpha
		out[i] = c
		idx >>= 2
		alphaIdx >>= 3
	}

	return out
}

// dxt5AlphaPalette builds the 8-entry alpha palette defined by BC3 rules.
func dxt5AlphaPalette(a0, a1 uint8) [8]uint8 {
	var p [8]uint8
	p[0] = a0
	p[1] = a1
	if a0 > a1 {
		p[2] = clampU8((6*int(a0) + 1*int(a1) + 3) / 7)
		p[3] = clampU8((5*int(a0) + 2*int(a1) + 3) / 7)
		p[4] = clampU8((4*int(a0) + 3*int(a1) + 3) / 7)
		p[5] = clampU8((3*int(a0) + 4*int(a1) + 3) / 7)
		p[6] = clampU8((2*int(a0) + 5*int(a1) + 3) / 7)
		p[7] = clampU8((1*int(a0) + 6*int(a1) + 3) / 7)
	} else {
		p[2] = clampU8((4*int(a0) + 1*int(a1) + 2) / 5)
		p[3] = clampU8((3*int(a0) + 2*int(a1) + 2) / 5)
		p[4] = clampU8((2*int(a0) + 3*int(a1) + 2) / 5)
		p[5] = clampU8((1*int(a0) + 4*int(a1) + 2) / 5)
		p[6] = 0
		p[7] = 255
	}

	return p
}

// alphaFromPalette reads the current 3-bit alpha index and resolves it to value.
func alphaFromPalette(p [8]uint8, idx uint64) uint8 {
	switch idx & 0x7 {
	case 0:
		return p[0]
	case 1:
		return p[1]
	case 2:
		return p[2]
	case 3:
		return p[3]
	case 4:
		return p[4]
	case 5:
		return p[5]
	case 6:
		return p[6]
	default:
		return p[7]
	}
}

// bestAlphaIndex returns the nearest alpha palette index for one sample.
func bestAlphaIndex(palette *[8]uint8, a uint8) uint8 {
	idx, _ := bestAlphaIndexErr(palette, a)
	return idx
}

// bestAlphaIndexErr returns nearest alpha index and squared error.
func bestAlphaIndexErr(palette *[8]uint8, a uint8) (uint8, int) {
	best := 0
	bestErr := 1<<31 - 1
	av := int(a)
	for i := range 8 {
		d := av - int(palette[i])
		err := d * d
		if err < bestErr {
			bestErr = err
			best = i
		}
	}

	return clampU8(best), bestErr
}
