// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "math"

// dxt1ColorEndpoints chooses initial endpoints using quality settings and optional refinement.
func dxt1ColorEndpoints(block [16]rgba8, opts EncodeOptions) (uint16, uint16) {
	w := getRGBWeightsFP(&opts, blockConstantR(block))
	settings := qualitySettingsForOpts(opts)

	var c0, c1 uint16
	if settings.usePCA {
		c0, c1 = dxt1EndpointsPCA(block)
	} else {
		c0, c1 = dxt1EndpointsFast(block)
	}

	if settings.colorTries > 0 || settings.lsqIters > 0 {
		c0, c1 = dxt1Refine(block, c0, c1, false, opts.AlphaThreshold, settings.colorStep, settings.colorTries, settings.lsqIters, w)
	}

	return c0, c1
}

// dot3 computes a 3-component dot product with explicit float64 conversions:
// per the Go spec they force intermediate rounding and forbid FMA contraction,
// so the result is bit-identical on every architecture
// (arm64 fuses x*y+z/ otherwise) and golden hashes stay portable.
// On amd64 nothing fuses anyway, so codegen is unchanged.
func dot3(a, b [3]float64) float64 {
	return float64(a[0]*b[0]) + float64(a[1]*b[1]) + float64(a[2]*b[2])
}

// pcaMinMax approximates principal-axis color extremes for better endpoint initialization.
func pcaMinMax(block [16]rgba8) (rgba8, rgba8) {
	var sumR, sumG, sumB int
	for _, px := range block {
		sumR += int(px.r)
		sumG += int(px.g)
		sumB += int(px.b)
	}
	mean := [3]float64{
		float64(sumR) / 16.0,
		float64(sumG) / 16.0,
		float64(sumB) / 16.0,
	}

	// Calculate the covariance matrix. Accumulated products are wrapped in
	// explicit float64 conversions for the same FMA-free guarantee as dot3.
	var cov [3][3]float64
	for _, px := range block {
		r := float64(px.r) - mean[0]
		g := float64(px.g) - mean[1]
		b := float64(px.b) - mean[2]
		cov[0][0] += float64(r * r)
		cov[0][1] += float64(r * g)
		cov[0][2] += float64(r * b)
		cov[1][1] += float64(g * g)
		cov[1][2] += float64(g * b)
		cov[2][2] += float64(b * b)
	}
	cov[1][0] = cov[0][1]
	cov[2][0] = cov[0][2]
	cov[2][1] = cov[1][2]

	// Power iteration to approximate the principal axis of the covariance matrix.
	axis := [3]float64{1, 1, 1}
	for range 8 {
		x := dot3(cov[0], axis)
		y := dot3(cov[1], axis)
		z := dot3(cov[2], axis)
		v := [3]float64{x, y, z}
		axisLen := math.Sqrt(dot3(v, v))
		if axisLen < 1e-5 {
			break
		}

		nx := x / axisLen
		ny := y / axisLen
		nz := z / axisLen
		if nx == axis[0] && ny == axis[1] && nz == axis[2] {
			break
		}
		axis[0] = nx
		axis[1] = ny
		axis[2] = nz
	}

	minDot := math.MaxFloat64
	maxDot := -math.MaxFloat64
	minC := rgba8{}
	maxC := rgba8{}
	hasExtremes := false

	// Iterate over the block to find the principal axis extremes.
	for _, px := range block {
		dot := dot3([3]float64{float64(px.r), float64(px.g), float64(px.b)}, axis)

		if !hasExtremes {
			minDot = dot
			maxDot = dot
			minC = px
			maxC = px
			hasExtremes = true
			continue
		}

		if dot < minDot {
			minDot = dot
			minC = px
		}
		if dot > maxDot {
			maxDot = dot
			maxC = px
		}
	}

	return minC, maxC
}

// vary565Into writes neighboring RGB565 endpoint candidates into out and returns count.
func vary565Into(c uint16, step int, out *[125]uint16) int {
	r := int((c >> 11) & 0x1F)
	g := int((c >> 5) & 0x3F)
	b := int(c & 0x1F)
	n := 0

	// Iterate over the neighboring RGB565 values (red).
	for dr := -step; dr <= step; dr++ {
		rr := r + dr
		if rr < 0 || rr > 0x1F {
			continue
		}

		// Iterate over the neighboring RGB565 values (green).
		for dg := -step; dg <= step; dg++ {
			gg := g + dg
			if gg < 0 || gg > 0x3F {
				continue
			}

			// Iterate over the neighboring RGB565 values (blue).
			for db := -step; db <= step; db++ {
				bb := b + db
				if bb < 0 || bb > 0x1F {
					continue
				}

				// #nosec G115 -- rr/gg/bb are range-checked.
				v := uint16((rr << 11) | (gg << 5) | bb)
				out[n] = v
				n++
			}
		}
	}

	return n
}
