// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// Least-squares endpoint refinement for the BC1-style color palette and the BC3/BC4/BC5 alpha palette.
// Given a fixed index assignment, the optimal pair of endpoints minimizing
// the squared reconstruction error has a closed-form solution (normal equations).
// We iterate assign -> solve -> quantize and keep a candidate only when it lowers the block error,
// so the result is never worse than the seed and the encoder output stays monotonic in quality.
//
// The idea follows the public-domain solver in
// BCnEncoder.NET/BCnEnc.Net/Encoder/LeastSquares.cs (from GPURealTimeBC6H),
// specialized here to integer LDR channels:
// accumulation is exact integer math and only the final per-endpoint divide is float64 (FMA-free, like dot3),
// so the result is bit-identical across architectures and the purego/asm builds.

// colorBetaNumerator maps a BC1 palette index to the c1-interpolation weight numerator b (with denominator d):
// the endpoint pair is reconstructed as pixel ~= ep0*(d-b)/d + ep1*b/d.
// In 4-color mode the entries are
// {c0, c1, (2c0+c1)/3, (c0+2c1)/3} -> b = {0, 3, 1, 2} with d=3;
// in 3-color mode they are {c0, c1, (c0+c1)/2, transparent} -> b = {0, 2, 1, n/a} with d=2.
// Index 3 in 3-color mode is the alpha hole and is excluded from the fit.
var (
	colorBetaNum4 = [4]int{0, 3, 1, 2}
	colorBetaNum3 = [4]int{0, 2, 1, 0}
)

type lsqColorSums struct {
	saa, sbb, sab    int // Normal matrix: sum(a*a), sum(b*b), sum(a*b).
	sapR, sapG, sapB int // Endpoint-0 projections: sum(a*R), sum(a*G), sum(a*B).
	sbpR, sbpG, sbpB int // Endpoint-1 projections: sum(b*R), sum(b*G), sum(b*B).
}

// lsqColorRefine polishes an ordered BC1 endpoint pair with iterated
// least-squares fitting. seedErr is the current block error of (c0, c1);
// candidates are accepted only when they strictly improve it.
// The returned pair is ordered per hasAlpha (4-color when opaque, 3-color when alpha).
func lsqColorRefine(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP, iters int, seedErr int64) (uint16, uint16) {
	bestC0, bestC1 := c0, c1
	bestErr := seedErr
	if bestErr == 0 {
		return bestC0, bestC1
	}

	for range iters {
		nc0, nc1, ok := lsqColorSolve(block, bestC0, bestC1, hasAlpha, alphaThreshold, w)
		if !ok {
			break
		}

		nc0, nc1 = orderDXT1(nc0, nc1, hasAlpha)
		if nc0 == bestC0 && nc1 == bestC1 {
			break
		}

		err := dxt1BlockError(block, nc0, nc1, hasAlpha, alphaThreshold, w, bestErr)
		if err >= bestErr {
			break
		}

		bestErr = err
		bestC0, bestC1 = nc0, nc1
		if bestErr == 0 {
			break
		}
	}

	return bestC0, bestC1
}

// lsqColorSolve assigns each pixel to its nearest palette entry of (c0, c1)
// and returns the least-squares-optimal endpoints for that assignment,
// quantized back to RGB565.
// ok is false when the index distribution is degenerate (no usable span),
// in which case the caller keeps the previous endpoints.
func lsqColorSolve(block [16]rgba8, c0, c1 uint16, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP) (uint16, uint16, bool) {
	palette := dxt1Palette(c0, c1)

	d := 3
	betaNum := &colorBetaNum4
	limit := 4
	if hasAlpha {
		d = 2
		betaNum = &colorBetaNum3
		limit = 3
	}

	sums, ok := lsqColorAccumulateASM(&block, &palette, hasAlpha, alphaThreshold, w, d, betaNum)
	if !ok {
		sums = lsqColorAccumulateGeneric(block, &palette, hasAlpha, alphaThreshold, w, d, betaNum, limit)
	}

	denom := int64(sums.saa)*int64(sums.sbb) - int64(sums.sab)*int64(sums.sab)
	if denom == 0 {
		return 0, 0, false
	}

	r0, r1 := lsqSolvePair(d, sums.saa, sums.sbb, sums.sab, sums.sapR, sums.sbpR, denom)
	g0, g1 := lsqSolvePair(d, sums.saa, sums.sbb, sums.sab, sums.sapG, sums.sbpG, denom)
	b0, b1 := lsqSolvePair(d, sums.saa, sums.sbb, sums.sab, sums.sapB, sums.sbpB, denom)

	e0 := rgba8{r: clampU8(r0), g: clampU8(g0), b: clampU8(b0), a: 255}
	e1 := rgba8{r: clampU8(r1), g: clampU8(g1), b: clampU8(b1), a: 255}

	return rgb565(e0), rgb565(e1), true
}

// lsqColorAccumulateGeneric is the scalar reference for BC1 LSQ index assignment and accumulation.
func lsqColorAccumulateGeneric(block [16]rgba8, palette *[4]rgba8, hasAlpha bool, alphaThreshold uint8, w rgbWeightsFP, d int, betaNum *[4]int, limit int) lsqColorSums {
	var sums lsqColorSums
	for _, px := range block {
		if hasAlpha && px.a < alphaThreshold {
			continue
		}

		idx := bestIndexWeighted(palette, px, w, limit)
		b := betaNum[idx]
		a := d - b

		sums.saa += a * a
		sums.sbb += b * b
		sums.sab += a * b
		sums.sapR += a * int(px.r)
		sums.sapG += a * int(px.g)
		sums.sapB += a * int(px.b)
		sums.sbpR += b * int(px.r)
		sums.sbpG += b * int(px.g)
		sums.sbpB += b * int(px.b)
	}

	return sums
}

// lsqSolvePair solves the 2x2 normal equations for one channel
// and returns the rounded endpoint values at beta=0 and beta=1.
// All products are computed in int64 (exact for LDR ranges),
// and the final divide uses integer round-half-away-from-zero to match math.Round semantics.
func lsqSolvePair(d, saa, sbb, sab, sap, sbp int, denom int64) (int, int) {
	num0 := int64(d) * (int64(sap)*int64(sbb) - int64(sbp)*int64(sab))
	num1 := int64(d) * (int64(sbp)*int64(saa) - int64(sap)*int64(sab))

	return roundRatio(num0, denom), roundRatio(num1, denom)
}

// roundRatio rounds num/denom to the nearest integer,
// with halves rounded away from zero. denom must be positive;
// LSQ determinants are checked before use.
func roundRatio(num, denom int64) int {
	if num < 0 {
		return -int((-num + denom/2) / denom)
	}

	return int((num + denom/2) / denom)
}

// alphaBetaNum maps a BC3/BC4 8-value alpha palette index to its interpolation numerator b (denominator 7):
// entries are {a0, a1, (6a0+a1)/7 ... (a0+6a1)/7}
// so index {0,1,2,3,4,5,6,7} -> b {0,7,1,2,3,4,5,6}.
// The encoder always emits the 8-value mode (a0 >= a1),
// so the 6-value mapping is not needed here.
var alphaBetaNum = [8]int{0, 7, 1, 2, 3, 4, 5, 6}

type lsqAlphaSums struct {
	saa, sbb, sab int
	sap, sbp      int
}

// lsqAlphaRefine polishes an ordered alpha endpoint pair (a0 >= a1)
// with iterated least-squares fitting, accepting a candidate only when it lowers the alpha block error.
// The returned pair keeps a0 >= a1 (8-value mode) and never collapses to a0 == a1.
func lsqAlphaRefine(alpha [16]uint8, a0, a1 uint8, iters int) (uint8, uint8) {
	bestA0, bestA1 := a0, a1
	bestErr := alphaBlockError(alpha, bestA0, bestA1, maxAlphaErr)
	if bestErr == 0 {
		return bestA0, bestA1
	}

	for range iters {
		na0, na1, ok := lsqAlphaSolve(alpha, bestA0, bestA1)
		if !ok {
			break
		}
		if na0 < na1 {
			na0, na1 = na1, na0
		}
		if na0 == na1 || (na0 == bestA0 && na1 == bestA1) {
			break
		}

		err := alphaBlockError(alpha, na0, na1, bestErr)
		if err >= bestErr {
			break
		}

		bestErr = err
		bestA0, bestA1 = na0, na1
		if bestErr == 0 {
			break
		}
	}

	return bestA0, bestA1
}

// maxAlphaErr is an effectively infinite cutoff for alphaBlockError.
const maxAlphaErr = 1 << 30

// lsqAlphaSolve assigns each sample to its nearest entry of the 8-value palette for (a0, a1)
// and returns the least-squares-optimal endpoints. ok is false on a degenerate distribution.
func lsqAlphaSolve(alpha [16]uint8, a0, a1 uint8) (uint8, uint8, bool) {
	palette := dxt5AlphaPalette(a0, a1)

	sums, ok := lsqAlphaAccumulateASM(&alpha, a0, a1)
	if !ok {
		sums = lsqAlphaAccumulateGeneric(alpha, &palette)
	}

	denom := int64(sums.saa)*int64(sums.sbb) - int64(sums.sab)*int64(sums.sab)
	if denom == 0 {
		return 0, 0, false
	}

	v0, v1 := lsqSolvePair(7, sums.saa, sums.sbb, sums.sab, sums.sap, sums.sbp, denom)

	return clampU8(v0), clampU8(v1), true
}

// lsqAlphaAccumulateGeneric is the scalar reference for BC3/BC4 LSQ index assignment and accumulation.
func lsqAlphaAccumulateGeneric(alpha [16]uint8, palette *[8]uint8) lsqAlphaSums {
	var sums lsqAlphaSums
	for _, s := range alpha {
		idx := bestAlphaIndex(palette, s)
		b := alphaBetaNum[idx]
		a := 7 - b

		sums.saa += a * a
		sums.sbb += b * b
		sums.sab += a * b
		sums.sap += a * int(s)
		sums.sbp += b * int(s)
	}

	return sums
}
