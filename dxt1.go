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

// decodeBlockDXT1 decodes one BC1/DXT1 block (8 bytes) to 16 RGBA pixels.
func decodeBlockDXT1(data []byte) [16]rgba8 {
	c0 := binary.LittleEndian.Uint16(data[0:2])
	c1 := binary.LittleEndian.Uint16(data[2:4])
	palette := dxt1Palette(c0, c1)
	idx := binary.LittleEndian.Uint32(data[4:8])
	var out [16]rgba8
	for i := range 16 {
		// #nosec G602 -- index masked to 0..3.
		out[i] = palette[int(idx&0x3)]
		idx >>= 2
	}

	return out
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
			r: clampU8((2*int(p0.r) + int(p1.r) + 1) / 3),
			g: clampU8((2*int(p0.g) + int(p1.g) + 1) / 3),
			b: clampU8((2*int(p0.b) + int(p1.b) + 1) / 3),
			a: 255,
		}
		palette[3] = rgba8{
			r: clampU8((int(p0.r) + 2*int(p1.r) + 1) / 3),
			g: clampU8((int(p0.g) + 2*int(p1.g) + 1) / 3),
			b: clampU8((int(p0.b) + 2*int(p1.b) + 1) / 3),
			a: 255,
		}
	} else {
		palette[2] = rgba8{
			r: clampU8((int(p0.r) + int(p1.r)) / 2),
			g: clampU8((int(p0.g) + int(p1.g)) / 2),
			b: clampU8((int(p0.b) + int(p1.b)) / 2),
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

	rw, gw, bw := getRGBWeights(&opts, blockConstantR(block))
	settings := qualitySettingsForOpts(opts)
	var c0, c1 uint16
	if settings.usePCA {
		c0, c1 = dxt1EndpointsPCA(block)
	} else {
		c0, c1 = dxt1EndpointsFast(block)
	}
	if settings.colorTries > 0 {
		c0, c1 = dxt1Refine(block, c0, c1, hasAlpha, opts.AlphaThreshold, settings.colorStep, settings.colorTries, rw, gw, bw)
	}

	c0, c1 = orderDXT1(c0, c1, hasAlpha)
	palette := dxt1Palette(c0, c1)
	indices := packDXT1IndicesWeighted(block, palette, hasAlpha, opts.AlphaThreshold, rw, gw, bw)

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
	return packDXT1IndicesWeighted(block, palette, hasAlpha, alphaThreshold, 0.3, 0.6, 0.1)
}

// packDXT1IndicesWeighted maps each pixel to the best palette entry and bit-packs indices.
func packDXT1IndicesWeighted(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8, rw, gw, bw float64) uint32 {
	pf := paletteToFloat(palette)
	indices := uint32(0)

	for i, px := range block {
		var idx uint8
		switch {
		case hasAlpha && px.a < alphaThreshold:
			idx = 3
		case hasAlpha:
			idx = bestIndexWeightedFloat(pf, px, rw, gw, bw, 3)
		default:
			idx = bestIndexWeightedFloat(pf, px, rw, gw, bw, 4)
		}

		indices |= uint32(idx) << (2 * i)
	}

	return indices
}

// rgbf stores RGB channels as float64 for weighted error evaluation.
type rgbf struct {
	r, g, b float64
}

// paletteToFloat converts integer palette entries to float form once per block.
func paletteToFloat(palette [4]rgba8) [4]rgbf {
	var out [4]rgbf

	paletteView := palette[:]
	outView := out[:]
	for len(outView) > 0 {
		c := paletteView[0]
		outView[0] = rgbf{
			r: float64(c.r),
			g: float64(c.g),
			b: float64(c.b),
		}
		outView = outView[1:]
		paletteView = paletteView[1:]
	}

	return out
}

// bestIndexWeightedFloat returns the best palette entry index under weighted RGB SSE.
func bestIndexWeightedFloat(palette [4]rgbf, c rgba8, rw, gw, bw float64, limit int) uint8 {
	idx, _ := bestIndexWeightedFloatErr(palette, c, rw, gw, bw, limit)
	return idx
}

// weightedDistF computes weighted RGB SSE between a pixel and one palette entry.
func weightedDistF(p rgbf, cr, cg, cb, rw, gw, bw float64) float64 {
	dr := cr - p.r
	dg := cg - p.g
	db := cb - p.b

	return dr*dr*rw + dg*dg*gw + db*db*bw
}

// bestErrorWeightedFloat returns only the minimal weighted error for a pixel.
// Unrolled: ties keep the lower index, same as the indexed variant.
func bestErrorWeightedFloat(palette [4]rgbf, c rgba8, rw, gw, bw float64, limit int) float64 {
	cr := float64(c.r)
	cg := float64(c.g)
	cb := float64(c.b)

	bestErr := weightedDistF(palette[0], cr, cg, cb, rw, gw, bw)
	if err := weightedDistF(palette[1], cr, cg, cb, rw, gw, bw); err < bestErr {
		bestErr = err
	}
	if err := weightedDistF(palette[2], cr, cg, cb, rw, gw, bw); err < bestErr {
		bestErr = err
	}
	if limit == 4 {
		if err := weightedDistF(palette[3], cr, cg, cb, rw, gw, bw); err < bestErr {
			bestErr = err
		}
	}

	return bestErr
}

// bestIndexWeightedFloatErr computes best palette index and weighted RGB SSE together.
// Unrolled over the 4 palette entries; strict < keeps the first minimum on ties.
func bestIndexWeightedFloatErr(palette [4]rgbf, c rgba8, rw, gw, bw float64, limit int) (uint8, float64) {
	cr := float64(c.r)
	cg := float64(c.g)
	cb := float64(c.b)

	best := uint8(0)
	bestErr := weightedDistF(palette[0], cr, cg, cb, rw, gw, bw)
	if err := weightedDistF(palette[1], cr, cg, cb, rw, gw, bw); err < bestErr {
		bestErr = err
		best = 1
	}
	if err := weightedDistF(palette[2], cr, cg, cb, rw, gw, bw); err < bestErr {
		bestErr = err
		best = 2
	}
	if limit == 4 {
		if err := weightedDistF(palette[3], cr, cg, cb, rw, gw, bw); err < bestErr {
			bestErr = err
			best = 3
		}
	}

	return best, bestErr
}

// dxt1Refine performs local endpoint search around an initial candidate pair.
func dxt1Refine(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, step, maxTries int, rw, gw, bw float64) (uint16, uint16) {
	bestC0, bestC1 := orderDXT1(c0, c1, hasAlpha)
	bestErr := dxt1BlockError(block, bestC0, bestC1, hasAlpha, alphaThreshold, rw, gw, bw, 1e30)
	if bestErr == 0 {
		return bestC0, bestC1
	}

	var candidates0 [125]uint16
	var candidates1 [125]uint16
	n0 := vary565Into(bestC0, step, &candidates0)
	n1 := vary565Into(bestC1, step, &candidates1)

	tries := 0
	for _, a := range candidates0[:n0] {
		for _, b := range candidates1[:n1] {
			ca, cb := orderDXT1(a, b, hasAlpha)
			err := dxt1BlockError(block, ca, cb, hasAlpha, alphaThreshold, rw, gw, bw, bestErr)
			if err < bestErr {
				bestErr = err
				bestC0 = ca
				bestC1 = cb
			}

			tries++
			if tries >= maxTries {
				return bestC0, bestC1
			}
		}
	}

	return bestC0, bestC1
}

// dxt1BlockError measures total weighted color error for one candidate block encoding.
// Accumulation stops once the partial sum reaches cutoff: per-pixel errors are
// non-negative, so such a candidate can never beat the current best and callers
// comparing with strict < reject the returned value either way.
func dxt1BlockError(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, rw, gw, bw float64, cutoff float64) float64 {
	palette := dxt1Palette(c0, c1)
	pf := paletteToFloat(palette)
	err := 0.0
	if hasAlpha {
		for _, px := range block {
			if px.a < alphaThreshold {
				continue
			}

			err += bestErrorWeightedFloat(pf, px, rw, gw, bw, 3)
			if err >= cutoff {
				return err
			}
		}
	} else {
		for _, px := range block {
			err += bestErrorWeightedFloat(pf, px, rw, gw, bw, 4)
			if err >= cutoff {
				return err
			}
		}
	}

	return err
}
