// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "encoding/binary"

// EncodeDXT1 encodes an RGBA image (NRGBA layout) into DXT1 blocks.
// The input length must be width*height*4.
func EncodeDXT1(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT1, nil)
}

// DecodeDXT1 decodes DXT1 blocks into an RGBA image (NRGBA layout).
func DecodeDXT1(data []byte, width, height int) ([]byte, error) {
	return decodeBlocks(data, width, height, FormatDXT1)
}

// DecodeDXT1WithOptions decodes DXT1 blocks with explicit options.
func DecodeDXT1WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, FormatDXT1, opts)
}

// EncodeDXT1WithOptions encodes with explicit options.
// QualityLevel and AlphaThreshold influence endpoint selection and 1-bit alpha mode.
func EncodeDXT1WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT1, opts)
}

// decodeBlockDXT1 decodes one BC1/DXT1 block (8 bytes) into 16 NRGBA pixels
// laid out as 4 rows of 16 bytes.
func decodeBlockDXT1(data []byte) [64]byte {
	c0 := binary.LittleEndian.Uint16(data[0:2])
	c1 := binary.LittleEndian.Uint16(data[2:4])
	pal := dxt1PaletteLE(c0, c1)
	idx := binary.LittleEndian.Uint32(data[4:8])
	var out [64]byte
	for i := 0; i < 64; i += 4 {
		binary.LittleEndian.PutUint32(out[i:i+4], pal[idx&0x3])
		idx >>= 2
	}

	return out
}

// dxt1PaletteLE builds the BC1 palette as packed little-endian NRGBA words.
func dxt1PaletteLE(c0, c1 uint16) [4]uint32 {
	p := dxt1Palette(c0, c1)
	return dxt1PaletteToLE(p)
}

// dxt1OpaquePaletteLE builds the four-color palette required by BC2 and BC3.
func dxt1OpaquePaletteLE(c0, c1 uint16) [4]uint32 {
	p := dxt1OpaquePalette(c0, c1)
	return dxt1PaletteToLE(p)
}

func dxt1PaletteToLE(p [4]rgba8) [4]uint32 {
	var pal [4]uint32
	for k := range pal {
		pal[k] = uint32(p[k].r) | uint32(p[k].g)<<8 | uint32(p[k].b)<<16 | uint32(p[k].a)<<24
	}

	return pal
}

// dxt1OpaquePalette builds the four-color palette required by BC2 and BC3,
// even when c0 <= c1. BC1 uses dxt1Palette instead for 1-bit alpha mode.
func dxt1OpaquePalette(c0, c1 uint16) [4]rgba8 {
	p0 := rgbaFrom565(c0)
	p1 := rgbaFrom565(c1)
	return [4]rgba8{
		p0,
		p1,
		{r: mix3(2, 1, p0.r, p1.r), g: mix3(2, 1, p0.g, p1.g), b: mix3(2, 1, p0.b, p1.b), a: 255},
		{r: mix3(1, 2, p0.r, p1.r), g: mix3(1, 2, p0.g, p1.g), b: mix3(1, 2, p0.b, p1.b), a: 255},
	}
}

// dxt1Palette builds the 4-entry BC1 palette from two RGB565 endpoints.
func dxt1Palette(c0, c1 uint16) [4]rgba8 {
	p0 := rgbaFrom565(c0)
	p1 := rgbaFrom565(c1)
	var palette [4]rgba8
	palette[0] = p0
	palette[1] = p1
	if c0 > c1 {
		palette[2] = rgba8{
			r: mix3(2, 1, p0.r, p1.r),
			g: mix3(2, 1, p0.g, p1.g),
			b: mix3(2, 1, p0.b, p1.b),
			a: 255,
		}
		palette[3] = rgba8{
			r: mix3(1, 2, p0.r, p1.r),
			g: mix3(1, 2, p0.g, p1.g),
			b: mix3(1, 2, p0.b, p1.b),
			a: 255,
		}
	} else {
		palette[2] = rgba8{
			r: avg2(p0.r, p1.r),
			g: avg2(p0.g, p1.g),
			b: avg2(p0.b, p1.b),
			a: 255,
		}
		palette[3] = rgba8{0, 0, 0, 0}
	}

	return palette
}

// encodeBlockDXT1WithOptions encodes one 4x4 block using endpoint search + index packing.
func encodeBlockDXT1WithOptions(block [16]rgba8, opts EncodeOptions) [8]byte {
	// If any pixel falls below AlphaThreshold, force 3-color mode (with 1-bit alpha).
	hasAlpha := false
	for _, px := range block {
		if px.a < opts.AlphaThreshold {
			hasAlpha = true
			break
		}
	}

	w := getRGBWeightsFP(&opts, blockConstantR(block))
	settings := qualitySettingsForOpts(opts)
	var c0, c1 uint16
	if settings.usePCA {
		c0, c1 = dxt1EndpointsPCA(block)
	} else {
		c0, c1 = dxt1EndpointsFast(block)
	}
	if settings.colorTries > 0 || settings.lsqIters > 0 {
		c0, c1 = dxt1Refine(block, c0, c1, hasAlpha, opts.AlphaThreshold, settings.colorStep, settings.colorTries, settings.lsqIters, w)
	}

	c0, c1 = orderDXT1(c0, c1, hasAlpha)
	palette := dxt1Palette(c0, c1)
	indices := packDXT1IndicesWeighted(block, palette, hasAlpha, opts.AlphaThreshold, w)

	var out [8]byte
	binary.LittleEndian.PutUint16(out[0:2], c0)
	binary.LittleEndian.PutUint16(out[2:4], c1)
	binary.LittleEndian.PutUint32(out[4:8], indices)

	return out
}

// dxt1EndpointsFast picks endpoints from axis-aligned min/max with inset.
func dxt1EndpointsFast(block [16]rgba8) (uint16, uint16) {
	minC, maxC := findMinMax(block)
	minC, maxC = insetMinMax(minC, maxC)
	c0 := rgb565(maxC)
	c1 := rgb565(minC)

	return c0, c1
}

// dxt1EndpointsPCA picks endpoints from a PCA-estimated principal color axis.
func dxt1EndpointsPCA(block [16]rgba8) (uint16, uint16) {
	minC, maxC := pcaMinMax(block)
	minC, maxC = insetMinMax(minC, maxC)

	return rgb565(maxC), rgb565(minC)
}

// orderDXT1 enforces BC1 endpoint ordering rules for opaque vs alpha mode.
func orderDXT1(c0, c1 uint16, hasAlpha bool) (uint16, uint16) {
	if hasAlpha {
		if c0 > c1 {
			return c1, c0
		}
		return c0, c1
	}

	if c0 < c1 {
		return c1, c0
	}

	return c0, c1
}

// blockConstantR reports whether all pixels have identical red channel.
func blockConstantR(block [16]rgba8) bool {
	r := block[0].r
	for i := 1; i < 16; i++ {
		if block[i].r != r {
			return false
		}
	}
	return true
}

// packDXT1Indices packs palette indices with default perceptual RGB weights.
func packDXT1Indices(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8) uint32 {
	return packDXT1IndicesWeighted(block, palette, hasAlpha, alphaThreshold, defaultWeightsFP)
}

// packDXT1IndicesWeighted maps each pixel to the best palette entry and bit-packs indices,
// using the AVX2 kernel when available and the scalar path otherwise.
func packDXT1IndicesWeighted(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP) uint32 {
	if idx, ok := packDXT1IndicesASM(&block, &palette, hasAlpha, alphaThreshold, w); ok {
		return idx
	}

	return packDXT1IndicesGeneric(block, palette, hasAlpha, alphaThreshold, w)
}

// packDXT1IndicesGeneric is the scalar reference for palette index assignment.
func packDXT1IndicesGeneric(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP) uint32 {
	indices := uint32(0)

	for i, px := range block {
		var idx uint8
		switch {
		case hasAlpha && px.a < alphaThreshold:
			idx = 3

		case hasAlpha:
			idx = bestIndexWeighted(&palette, px, w, 3)

		default:
			idx = bestIndexWeighted(&palette, px, w, 4)
		}

		indices |= uint32(idx) << (2 * i)
	}

	return indices
}

// maxBlockErr is an effectively infinite cutoff for dxt1BlockError.
const maxBlockErr = int64(1) << 62

// weightedDist computes fixed-point weighted RGB SSE between a pixel
// (pre-split into int32 channels) and one palette entry.
// Weights sum to ~1024, so the value stays below 255^2*1026 < 2^31.
func weightedDist(p rgba8, cr, cg, cb int32, w rgbWeightsFP) int32 {
	dr := cr - int32(p.r)
	dg := cg - int32(p.g)
	db := cb - int32(p.b)

	return dr*dr*w.r + dg*dg*w.g + db*db*w.b
}

// bestIndexWeighted returns the best palette entry index under weighted RGB SSE.
func bestIndexWeighted(palette *[4]rgba8, c rgba8, w rgbWeightsFP, limit int) uint8 {
	idx, _ := bestIndexWeightedErr(palette, c, w, limit)
	return idx
}

// bestErrorWeighted returns only the minimal weighted error for a pixel.
// Unrolled: ties keep the lower index, same as the indexed variant.
func bestErrorWeighted(palette *[4]rgba8, c rgba8, w rgbWeightsFP, limit int) int32 {
	cr := int32(c.r)
	cg := int32(c.g)
	cb := int32(c.b)

	bestErr := weightedDist(palette[0], cr, cg, cb, w)
	if err := weightedDist(palette[1], cr, cg, cb, w); err < bestErr {
		bestErr = err
	}
	if err := weightedDist(palette[2], cr, cg, cb, w); err < bestErr {
		bestErr = err
	}
	if limit == 4 {
		if err := weightedDist(palette[3], cr, cg, cb, w); err < bestErr {
			bestErr = err
		}
	}

	return bestErr
}

// bestIndexWeightedErr computes best palette index and weighted RGB SSE together.
// Unrolled over the 4 palette entries; strict < keeps the first minimum on ties.
func bestIndexWeightedErr(palette *[4]rgba8, c rgba8, w rgbWeightsFP, limit int) (uint8, int32) {
	cr := int32(c.r)
	cg := int32(c.g)
	cb := int32(c.b)

	best := uint8(0)
	bestErr := weightedDist(palette[0], cr, cg, cb, w)
	if err := weightedDist(palette[1], cr, cg, cb, w); err < bestErr {
		bestErr = err
		best = 1
	}
	if err := weightedDist(palette[2], cr, cg, cb, w); err < bestErr {
		bestErr = err
		best = 2
	}
	if limit == 4 {
		if err := weightedDist(palette[3], cr, cg, cb, w); err < bestErr {
			bestErr = err
			best = 3
		}
	}

	return best, bestErr
}

// dxt1Refine performs local endpoint search around an initial candidate pair,
// then polishes the winner with iterated least-squares fitting.
// Both stages only accept strictly-lower-error candidates,
// so the result is never worse than the seed endpoints.
func dxt1Refine(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, step, maxTries, lsqIters int, w rgbWeightsFP) (uint16, uint16) {
	bestC0, bestC1 := orderDXT1(c0, c1, hasAlpha)
	bestErr := dxt1BlockError(block, bestC0, bestC1, hasAlpha, alphaThreshold, w, maxBlockErr)
	if bestErr == 0 {
		return bestC0, bestC1
	}

	if maxTries > 0 {
		var candidates0 [125]uint16
		var candidates1 [125]uint16
		n0 := vary565Into(bestC0, step, &candidates0)
		n1 := vary565Into(bestC1, step, &candidates1)

		tries := 0
	grid:
		for _, a := range candidates0[:n0] {
			for _, b := range candidates1[:n1] {
				ca, cb := orderDXT1(a, b, hasAlpha)
				err := dxt1BlockError(block, ca, cb, hasAlpha, alphaThreshold, w, bestErr)
				if err < bestErr {
					bestErr = err
					bestC0 = ca
					bestC1 = cb
					if bestErr == 0 {
						return bestC0, bestC1
					}
				}

				tries++
				if tries >= maxTries {
					break grid
				}
			}
		}
	}

	if lsqIters > 0 {
		bestC0, bestC1 = lsqColorRefine(block, bestC0, bestC1, hasAlpha, alphaThreshold, w, lsqIters, bestErr)
	}

	return bestC0, bestC1
}

// dxt1BlockError measures total weighted color error for one candidate block encoding.
// The opaque path uses the AVX2 score kernel (exact total) when available;
// the scalar path stops accumulating once the partial sum reaches cutoff.
// Per-pixel errors are non-negative, so a candidate that hits the cutoff can never beat the current best,
// and callers comparing with strict < reject it either way - making the exact-total kernel
// and the cutoff scalar interchangeable for the winner.
func dxt1BlockError(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP, cutoff int64) int64 {
	if !hasAlpha {
		if e, ok := scoreDXT1PaletteASM(&block, c0, c1, w); ok {
			return e
		}
	}

	return dxt1BlockErrorScalar(block, c0, c1, hasAlpha, alphaThreshold, w, cutoff)
}

// dxt1BlockErrorScalar is the pure-Go reference for dxt1BlockError.
func dxt1BlockErrorScalar(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP, cutoff int64) int64 {
	palette := dxt1Palette(c0, c1)
	err := int64(0)
	if hasAlpha {
		for _, px := range block {
			if px.a < alphaThreshold {
				continue
			}

			err += int64(bestErrorWeighted(&palette, px, w, 3))
			if err >= cutoff {
				return err
			}
		}
	} else {
		for _, px := range block {
			err += int64(bestErrorWeighted(&palette, px, w, 4))
			if err >= cutoff {
				return err
			}
		}
	}

	return err
}
