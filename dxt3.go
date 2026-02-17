// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

// EncodeDXT3 encodes an RGBA image (NRGBA layout) into DXT3 blocks.
func EncodeDXT3(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT3, nil)
}

// DecodeDXT3 decodes DXT3 blocks into an RGBA image (NRGBA layout).
func DecodeDXT3(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatDXT3)
}

// DecodeDXT3WithOptions decodes DXT3 blocks with explicit options.
func DecodeDXT3WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatDXT3, opts)
}

// EncodeDXT3WithOptions encodes with explicit options.
// QualityLevel affects color endpoint selection; alpha is explicit 4-bit.
func EncodeDXT3WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT3, opts)
}

func encodeBlockDXT3WithOptions(block [16]rgba8, opts EncodeOptions) [16]byte {
	alphaBits := uint64(0)
	for i, px := range block {
		q := clampU8((int(px.a) + 8) / 17)
		alphaBits |= uint64(q&0xF) << (4 * i)
	}

	c0, c1 := dxt1ColorEndpoints(block, opts)
	palette := dxt1Palette(c0, c1)
	indices := packDXT1Indices(block, palette, false, opts.AlphaThreshold)

	var out [16]byte
	binary.LittleEndian.PutUint64(out[0:8], alphaBits)
	binary.LittleEndian.PutUint16(out[8:10], c0)
	binary.LittleEndian.PutUint16(out[10:12], c1)
	binary.LittleEndian.PutUint32(out[12:16], indices)

	return out
}

func decodeBlockDXT3(data []byte) [16]rgba8 {
	alphaBits := binary.LittleEndian.Uint64(data[0:8])
	c0 := binary.LittleEndian.Uint16(data[8:10])
	c1 := binary.LittleEndian.Uint16(data[10:12])
	palette := dxt1Palette(c0, c1)
	idx := binary.LittleEndian.Uint32(data[12:16])
	var out [16]rgba8
	for i := range 16 {
		// #nosec G602 -- index masked to 0..3.
		c := palette[int(idx&0x3)]
		// #nosec G115 -- value is in 0..255 after masking.
		alpha := clampU8(int((alphaBits & 0xF) * 17))
		c.a = alpha
		out[i] = c // #nosec G602 -- i comes from range over fixed-size output block.
		idx >>= 2
		alphaBits >>= 4
	}

	return out
}
