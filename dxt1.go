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

// EncodeDXT1WithOptions encodes with explicit options.
// Quality and AlphaThreshold influence endpoint selection and 1-bit alpha mode.
func EncodeDXT1WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatDXT1, opts)
}

func decodeBlockDXT1(data []byte) [16]rgba8 {
	c0 := binary.LittleEndian.Uint16(data[0:2])
	c1 := binary.LittleEndian.Uint16(data[2:4])
	palette := dxt1Palette(c0, c1)
	idx := binary.LittleEndian.Uint32(data[4:8])
	var out [16]rgba8
	for i := 0; i < 16; i++ {
		// #nosec G602 -- index masked to 0..3.
		out[i] = palette[int(idx&0x3)]
		idx >>= 2
	}

	return out
}

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

func encodeBlockDXT1WithOptions(block [16]rgba8, opts EncodeOptions) [8]byte {
	// If any pixel falls below AlphaThreshold, force 3-color mode (with 1-bit alpha).
	hasAlpha := false
	for i := 0; i < 16; i++ {
		if block[i].a < opts.AlphaThreshold {
			hasAlpha = true
			break
		}
	}

	rw, gw, bw := getRGBWeights(&opts, blockConstantR(block))
	var c0, c1 uint16
	switch opts.Quality {
	case QualityFast:
		c0, c1 = dxt1EndpointsFast(block)
	case QualityBalanced:
		c0, c1 = dxt1EndpointsPCA(block)
		c0, c1 = dxt1Refine(block, c0, c1, hasAlpha, opts.AlphaThreshold, 1, 64, rw, gw, bw)
	case QualityBest:
		c0, c1 = dxt1EndpointsPCA(block)
		c0, c1 = dxt1Refine(block, c0, c1, hasAlpha, opts.AlphaThreshold, 2, 256, rw, gw, bw)
	default:
		c0, c1 = dxt1EndpointsFast(block)
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

func dxt1EndpointsFast(block [16]rgba8) (uint16, uint16) {
	minC, maxC := findMinMax(block)
	minC, maxC = insetMinMax(minC, maxC)
	c0 := rgb565(maxC)
	c1 := rgb565(minC)

	return c0, c1
}

func dxt1EndpointsPCA(block [16]rgba8) (uint16, uint16) {
	minC, maxC := pcaMinMax(block)
	minC, maxC = insetMinMax(minC, maxC)

	return rgb565(maxC), rgb565(minC)
}

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

func blockConstantR(block [16]rgba8) bool {
	r := block[0].r
	for i := 1; i < 16; i++ {
		if block[i].r != r {
			return false
		}
	}
	return true
}

func packDXT1Indices(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8) uint32 {
	return packDXT1IndicesWeighted(block, palette, hasAlpha, alphaThreshold, 0.3, 0.6, 0.1)
}

func packDXT1IndicesWeighted(block [16]rgba8, palette [4]rgba8, hasAlpha bool, alphaThreshold uint8, rw, gw, bw float64) uint32 {
	indices := uint32(0)
	for i := 15; i >= 0; i-- {
		var idx uint8
		if hasAlpha && block[i].a < alphaThreshold {
			idx = 3
		} else {
			idx = bestIndexWeighted(palette, block[i], rw, gw, bw, hasAlpha)
		}

		indices = (indices << 2) | uint32(idx)
		if i == 0 {
			break
		}
	}

	return indices
}

func bestIndexWeighted(palette [4]rgba8, c rgba8, rw, gw, bw float64, hasAlpha bool) uint8 {
	best := 0
	bestErr := 1e30
	limit := 4
	if hasAlpha {
		limit = 3
	}

	for i := 0; i < limit; i++ {
		dr := float64(int(c.r) - int(palette[i].r))
		dg := float64(int(c.g) - int(palette[i].g))
		db := float64(int(c.b) - int(palette[i].b))
		err := dr*dr*rw + dg*dg*gw + db*db*bw
		if err < bestErr {
			bestErr = err
			best = i
		}
	}

	return clampU8(best)
}

func dxt1Refine(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, step, maxTries int, rw, gw, bw float64) (uint16, uint16) {
	bestC0, bestC1 := orderDXT1(c0, c1, hasAlpha)
	bestErr := dxt1BlockError(block, bestC0, bestC1, hasAlpha, alphaThreshold, rw, gw, bw)
	candidates0 := vary565(bestC0, step)
	candidates1 := vary565(bestC1, step)

	tries := 0
	for _, a := range candidates0 {
		for _, b := range candidates1 {
			ca, cb := orderDXT1(a, b, hasAlpha)
			err := dxt1BlockError(block, ca, cb, hasAlpha, alphaThreshold, rw, gw, bw)
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

func dxt1BlockError(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, rw, gw, bw float64) float64 {
	palette := dxt1Palette(c0, c1)
	err := 0.0
	for i := 0; i < 16; i++ {
		if hasAlpha && block[i].a < alphaThreshold {
			continue
		}

		idx := bestIndexWeighted(palette, block[i], rw, gw, bw, hasAlpha)
		p := palette[idx]
		dr := float64(int(block[i].r) - int(p.r))
		dg := float64(int(block[i].g) - int(p.g))
		db := float64(int(block[i].b) - int(p.b))
		err += dr*dr*rw + dg*dg*gw + db*db*bw
	}

	return err
}
