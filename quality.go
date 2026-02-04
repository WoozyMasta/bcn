package bcn

import "math"

func dxt1ColorEndpoints(block [16]rgba8, opts EncodeOptions) (uint16, uint16) {
	switch opts.Quality {
	case QualityFast:
		return dxt1EndpointsFast(block)
	case QualityBalanced:
		c0, c1 := dxt1EndpointsPCA(block)
		return dxt1Refine(block, c0, c1, false, opts.AlphaThreshold, 1, 64)
	case QualityBest:
		c0, c1 := dxt1EndpointsPCA(block)
		return dxt1Refine(block, c0, c1, false, opts.AlphaThreshold, 2, 256)
	default:
		return dxt1EndpointsFast(block)
	}
}

func pcaMinMax(block [16]rgba8) (rgba8, rgba8) {
	var mean [3]float64
	for _, px := range block {
		mean[0] += float64(px.r)
		mean[1] += float64(px.g)
		mean[2] += float64(px.b)
	}
	mean[0] /= 16
	mean[1] /= 16
	mean[2] /= 16

	var cov [3][3]float64
	for _, px := range block {
		r := float64(px.r) - mean[0]
		g := float64(px.g) - mean[1]
		b := float64(px.b) - mean[2]
		cov[0][0] += r * r
		cov[0][1] += r * g
		cov[0][2] += r * b
		cov[1][0] += g * r
		cov[1][1] += g * g
		cov[1][2] += g * b
		cov[2][0] += b * r
		cov[2][1] += b * g
		cov[2][2] += b * b
	}

	// Power iteration to approximate the principal axis of the covariance matrix.
	axis := [3]float64{1, 1, 1}
	for i := 0; i < 8; i++ {
		x := cov[0][0]*axis[0] + cov[0][1]*axis[1] + cov[0][2]*axis[2] // #nosec G602 -- fixed-size 3x3 matrix.
		y := cov[1][0]*axis[0] + cov[1][1]*axis[1] + cov[1][2]*axis[2] // #nosec G602 -- fixed-size 3x3 matrix.
		z := cov[2][0]*axis[0] + cov[2][1]*axis[1] + cov[2][2]*axis[2] // #nosec G602 -- fixed-size 3x3 matrix.
		axisLen := math.Sqrt(x*x + y*y + z*z)
		if axisLen < 1e-5 {
			break
		}

		axis[0] = x / axisLen
		axis[1] = y / axisLen
		axis[2] = z / axisLen
	}

	minDot := math.MaxFloat64
	maxDot := -math.MaxFloat64
	minC := block[0]
	maxC := block[0]
	for _, px := range block {
		r := float64(px.r)
		g := float64(px.g)
		b := float64(px.b)
		dot := r*axis[0] + g*axis[1] + b*axis[2]
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

func vary565(c uint16, step int) []uint16 {
	r := int((c >> 11) & 0x1F)
	g := int((c >> 5) & 0x3F)
	b := int(c & 0x1F)
	seen := make(map[uint16]struct{})
	out := make([]uint16, 0, (2*step+1)*(2*step+1)*(2*step+1))
	for dr := -step; dr <= step; dr++ {
		rr := r + dr
		if rr < 0 || rr > 0x1F {
			continue
		}

		for dg := -step; dg <= step; dg++ {
			gg := g + dg
			if gg < 0 || gg > 0x3F {
				continue
			}

			for db := -step; db <= step; db++ {
				bb := b + db
				if bb < 0 || bb > 0x1F {
					continue
				}

				// #nosec G115 -- rr/gg/bb are range-checked.
				v := uint16((rr << 11) | (gg << 5) | bb)
				if _, ok := seen[v]; ok {
					continue
				}

				seen[v] = struct{}{}
				out = append(out, v)
			}
		}
	}

	return out
}
