// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

// EncodeBC2 encodes an RGBA image (NRGBA layout) into BC2 blocks.
func EncodeBC2(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC2, nil)
}

// DecodeBC2 decodes BC2 blocks into an RGBA image (NRGBA layout).
func DecodeBC2(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatBC2)
}

// DecodeBC2WithOptions decodes BC2 blocks with explicit options.
func DecodeBC2WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatBC2, opts)
}

// EncodeBC2WithOptions encodes with explicit options.
// QualityLevel affects color endpoint selection; alpha is explicit 4-bit.
func EncodeBC2WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC2, opts)
}

// encodeBlockBC2WithOptions encodes one BC2 block (explicit 4-bit alpha + BC1 color).
func encodeBlockBC2WithOptions(block [16]rgba8, opts EncodeOptions) [16]byte {
	alphaBits := uint64(0)
	for i, px := range block {
		q := clampU8((int(px.a) + 8) / 17)
		alphaBits |= uint64(q&0xF) << (4 * i)
	}

	c0, c1 := bc1ColorEndpoints(block, opts)
	palette := bc1Palette(c0, c1)
	indices := packBC1Indices(block, palette, false, opts.AlphaThreshold)

	var out [16]byte
	binary.LittleEndian.PutUint64(out[0:8], alphaBits)
	binary.LittleEndian.PutUint16(out[8:10], c0)
	binary.LittleEndian.PutUint16(out[10:12], c1)
	binary.LittleEndian.PutUint32(out[12:16], indices)

	return out
}

// decodeBlockBC2 decodes one BC2 block into 16 NRGBA pixels
// laid out as 4 rows of 16 bytes.
func decodeBlockBC2(data []byte) [64]byte {
	alphaBits := binary.LittleEndian.Uint64(data[0:8])
	c0 := binary.LittleEndian.Uint16(data[8:10])
	c1 := binary.LittleEndian.Uint16(data[10:12])
	pal := bc1OpaquePaletteLE(c0, c1)
	idx := binary.LittleEndian.Uint32(data[12:16])

	var out [64]byte
	for i := 0; i < 64; i += 4 {
		// 4-bit alpha expands as a*17, at most 255.
		a := uint32(alphaBits&0xF) * 17
		binary.LittleEndian.PutUint32(out[i:i+4], pal[idx&0x3]&0x00FFFFFF|a<<24)
		idx >>= 2
		alphaBits >>= 4
	}

	return out
}
