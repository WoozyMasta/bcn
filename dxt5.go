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
		var alpha [16]uint8
		for i := range block {
			alpha[i] = block[i].a
		}
		alphaIdx = packAlphaIndices(a0, a1, &alpha)
	}

	c0, c1 := dxt1ColorEndpoints(block, opts)
	palette := dxt1Palette(c0, c1)
	w := getRGBWeightsFP(&opts, minC.r == maxC.r)
	indices := packDXT1IndicesWeighted(block, palette, false, opts.AlphaThreshold, w)

	var out [16]byte
	out[0] = a0
	out[1] = a1
	// 48-bit alpha indices, little-endian
	putAlphaIndices(out[2:8], alphaIdx)
	binary.LittleEndian.PutUint16(out[8:10], c0)
	binary.LittleEndian.PutUint16(out[10:12], c1)
	binary.LittleEndian.PutUint32(out[12:16], indices)

	return out
}

// decodeBlockDXT5 decodes one BC3/DXT5 block into 16 NRGBA pixels
// laid out as 4 rows of 16 bytes.
func decodeBlockDXT5(data []byte) [64]byte {
	a0 := data[0]
	a1 := data[1]
	alphaPalette := dxt5AlphaPalette(a0, a1)
	alphaIdx := uint64(data[2]) | uint64(data[3])<<8 | uint64(data[4])<<16 | uint64(data[5])<<24 | uint64(data[6])<<32 | uint64(data[7])<<40
	c0 := binary.LittleEndian.Uint16(data[8:10])
	c1 := binary.LittleEndian.Uint16(data[10:12])
	pal := dxt1PaletteLE(c0, c1)
	idx := binary.LittleEndian.Uint32(data[12:16])

	var out [64]byte
	for i := 0; i < 64; i += 4 {
		a := uint32(alphaPalette[alphaIdx&0x7])
		binary.LittleEndian.PutUint32(out[i:i+4], pal[idx&0x3]&0x00FFFFFF|a<<24)
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
		p[2] = mix7(6, 1, a0, a1)
		p[3] = mix7(5, 2, a0, a1)
		p[4] = mix7(4, 3, a0, a1)
		p[5] = mix7(3, 4, a0, a1)
		p[6] = mix7(2, 5, a0, a1)
		p[7] = mix7(1, 6, a0, a1)
	} else {
		p[2] = mix5(4, 1, a0, a1)
		p[3] = mix5(3, 2, a0, a1)
		p[4] = mix5(2, 3, a0, a1)
		p[5] = mix5(1, 4, a0, a1)
		p[6] = 0
		p[7] = 255
	}

	return p
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
