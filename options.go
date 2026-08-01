// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "math"

const (
	// QualityLevelFast prioritizes speed over quality.
	QualityLevelFast = 1
	// QualityLevelBalanced is the default, balancing speed and quality.
	QualityLevelBalanced = 6
	// QualityLevelBest prioritizes quality and can be slower.
	QualityLevelBest = 8

	qualityLevelMin = 1
	qualityLevelMax = 10
)

// RGBWeights are used when choosing BC1 palette indices (and in refinement).
// R, G, B are relative weights; they are normalized when used.
// Used to preserve channels that matter (e.g. blue in normal maps).
type RGBWeights struct {
	R, G, B float64
}

// Presets for RGBWeights when encoding BC1/BC3 RGB block.
var (
	// DefaultRGBWeights is luminance-oriented (green dominant). Use for typical photos/UI.
	DefaultRGBWeights = RGBWeights{R: 0.3, G: 0.6, B: 0.1}
	// BalancedRGBWeights treats R, G, B equally. Use when all channels matter (e.g. normal maps).
	BalancedRGBWeights = RGBWeights{R: 1.0 / 3.0, G: 1.0 / 3.0, B: 1.0 / 3.0}
)

// rgbWeightsFP holds fixed-point channel weights normalized to sum ~1024.
// The error metric is scale-invariant, so normalization keeps per-pixel
// weighted SSE below 255^2*1026 (fits int32) without changing any decision.
type rgbWeightsFP struct {
	r, g, b int32
}

// Fixed-point forms of the weight presets.
var (
	defaultWeightsFP  = fixedRGBWeights(DefaultRGBWeights.R, DefaultRGBWeights.G, DefaultRGBWeights.B)
	balancedWeightsFP = fixedRGBWeights(BalancedRGBWeights.R, BalancedRGBWeights.G, BalancedRGBWeights.B)
)

// fixedRGBWeights converts relative float weights to the fixed-point scale.
// Negative weights are treated as zero; an all-zero set stays all-zero.
func fixedRGBWeights(rw, gw, bw float64) rgbWeightsFP {
	rw = math.Max(rw, 0)
	gw = math.Max(gw, 0)
	bw = math.Max(bw, 0)

	sum := rw + gw + bw
	if sum <= 0 {
		return rgbWeightsFP{}
	}

	s := 1024.0 / sum
	return rgbWeightsFP{
		r: int32(math.Round(rw * s)),
		g: int32(math.Round(gw * s)),
		b: int32(math.Round(bw * s)),
	}
}

// EncodeOptions configures block encoding and mipmap generation.
type EncodeOptions struct {
	// RGBWeights overrides weights for BC1 palette index selection (R, G, B). Nil = default;
	// for BC3, if nil and block has constant R (e.g. nohq), Balanced is used automatically.
	RGBWeights *RGBWeights
	// Refinement overrides quality behavior when non-nil (applied on top of QualityLevel).
	Refinement *RefinementOptions
	// qualitySettings is an internal cache of quality settings derived from QualityLevel and Refinement.
	qualitySettings *qualitySettings
	// weightsFP is an internal fixed-point cache of RGBWeights.
	weightsFP *rgbWeightsFP
	// QualityLevel provides a 1..10 quality scale. 0 = default (Balanced).
	// Recommended: 1=fast, 6=balanced, 8=best, 9-10=extreme.
	//
	// Beyond level 1, endpoint selection adds a PCA seed, a grid search,
	// and a least-squares endpoint refit (see RefinementOptions.LSQIters).
	// The refit trades some encode speed for higher quality.
	QualityLevel int
	// Workers controls parallel block encoding. 0 = auto (GOMAXPROCS), 1 = disable parallelism,
	// N > 1 = use N workers. Defaults to 0.
	Workers int
	// GenerateMipmaps enables mipmap generation from the input image.
	GenerateMipmaps bool
	// UseSRGB enables sRGB-aware downscale for mip generation.
	UseSRGB bool
	// AlphaThreshold controls BC1 1-bit alpha cutout (0..255). Default 128.
	AlphaThreshold uint8
}

// DecodeOptions configures block decoding.
type DecodeOptions struct {
	// Workers controls parallel block decoding. 0 = auto (GOMAXPROCS), 1 = disable parallelism,
	// N > 1 = use N workers. Defaults to 0 (auto) when options are omitted.
	Workers int
}

// RefinementOptions allows overriding quality behavior derived from QualityLevel.
// Nil fields mean "use defaults".
type RefinementOptions struct {
	UsePCA     *bool // UsePCA overrides quality behavior derived from QualityLevel.
	ColorTries *int  // ColorTries overrides quality behavior derived from QualityLevel.
	AlphaTries *int  // AlphaTries overrides quality behavior derived from QualityLevel.
	ColorStep  *int  // ColorStep overrides quality behavior derived from QualityLevel.

	// LSQIters overrides the least-squares endpoint polish iterations applied after the grid search.
	// 0 disables LSQ (grid search still runs); nil uses the quality-derived default.
	LSQIters *int
}

// normalizeEncodeOptions applies defaults, bounds and cached derived settings.
func normalizeEncodeOptions(opts *EncodeOptions) EncodeOptions {
	if opts == nil {
		out := EncodeOptions{QualityLevel: QualityLevelBalanced, AlphaThreshold: 128}
		qs := resolveQualitySettings(out)
		out.qualitySettings = &qs
		return out
	}

	out := *opts
	if out.AlphaThreshold == 0 {
		out.AlphaThreshold = 128
	}
	if out.QualityLevel == 0 {
		out.QualityLevel = QualityLevelBalanced
	}
	if out.QualityLevel < qualityLevelMin {
		out.QualityLevel = qualityLevelMin
	}
	if out.QualityLevel > qualityLevelMax {
		out.QualityLevel = qualityLevelMax
	}
	if out.Refinement != nil {
		ref := *out.Refinement
		out.Refinement = &ref
		normalizeRefinement(out.Refinement)
	}
	if out.RGBWeights != nil {
		fp := fixedRGBWeights(out.RGBWeights.R, out.RGBWeights.G, out.RGBWeights.B)
		out.weightsFP = &fp
	}
	qs := resolveQualitySettings(out)
	out.qualitySettings = &qs

	return out
}

// qualitySettings stores resolved low-level encoder tuning knobs.
type qualitySettings struct {
	colorTries    int
	colorStep     int
	alphaTries    int
	lsqIters      int
	bc7Partitions int
	usePCA        bool
}

// qualitySettingsForOpts returns cached settings when available, otherwise resolves them.
func qualitySettingsForOpts(opts EncodeOptions) qualitySettings {
	if opts.qualitySettings != nil {
		return *opts.qualitySettings
	}
	return resolveQualitySettings(opts)
}

// resolveQualitySettings combines QualityLevel and optional Refinement overrides.
func resolveQualitySettings(opts EncodeOptions) qualitySettings {
	var settings qualitySettings
	if opts.QualityLevel == 0 {
		settings = qualitySettingsFromLevel(QualityLevelBalanced)
	} else {
		settings = qualitySettingsFromLevel(opts.QualityLevel)
	}

	if settings.colorStep < 1 {
		settings.colorStep = 1
	}
	if settings.alphaTries == 0 {
		settings.alphaTries = settings.colorTries
	}

	if opts.Refinement != nil {
		ref := opts.Refinement
		if ref.UsePCA != nil {
			settings.usePCA = *ref.UsePCA
		}
		if ref.ColorTries != nil {
			settings.colorTries = clampNonNegative(*ref.ColorTries)
		}
		if ref.AlphaTries != nil {
			settings.alphaTries = clampNonNegative(*ref.AlphaTries)
		}
		if ref.ColorStep != nil {
			step := max(*ref.ColorStep, 1)
			settings.colorStep = step
		}
		if ref.LSQIters != nil {
			settings.lsqIters = clampNonNegative(*ref.LSQIters)
		}
	}

	return settings
}

// qualitySettingsFromLevel maps a 1..10 quality level to concrete search settings.
//
// lsqIters caps the least-squares endpoint polish iterations applied after the grid search;
// it converges and breaks early, so the cap is a safety bound, not a fixed cost.
// Level 1 stays a pure fast path (no refinement, no LSQ).
func qualitySettingsFromLevel(level int) qualitySettings {
	switch level {
	case 1:
		return qualitySettings{usePCA: false, colorTries: 0, colorStep: 1, alphaTries: 0, lsqIters: 0, bc7Partitions: 0}
	case 2:
		return qualitySettings{usePCA: false, colorTries: 8, colorStep: 1, alphaTries: 8, lsqIters: 2, bc7Partitions: 4}
	case 3:
		return qualitySettings{usePCA: false, colorTries: 16, colorStep: 1, alphaTries: 16, lsqIters: 2, bc7Partitions: 4}
	case 4:
		return qualitySettings{usePCA: false, colorTries: 32, colorStep: 1, alphaTries: 32, lsqIters: 2, bc7Partitions: 8}
	case 5:
		return qualitySettings{usePCA: true, colorTries: 32, colorStep: 1, alphaTries: 32, lsqIters: 2, bc7Partitions: 8}
	case 6:
		return qualitySettings{usePCA: true, colorTries: 64, colorStep: 1, alphaTries: 64, lsqIters: 2, bc7Partitions: 8}
	case 7:
		return qualitySettings{usePCA: true, colorTries: 96, colorStep: 1, alphaTries: 96, lsqIters: 2, bc7Partitions: 16}
	case 8:
		return qualitySettings{usePCA: true, colorTries: 256, colorStep: 2, alphaTries: 256, lsqIters: 4, bc7Partitions: 16}
	case 9:
		return qualitySettings{usePCA: true, colorTries: 384, colorStep: 1, alphaTries: 384, lsqIters: 4, bc7Partitions: 32}
	case 10:
		return qualitySettings{usePCA: true, colorTries: 512, colorStep: 1, alphaTries: 512, lsqIters: 4, bc7Partitions: 64}
	default:
		return qualitySettingsFromLevel(QualityLevelBalanced)
	}
}

// normalizeRefinement clamps refinement override fields to valid ranges in place.
func normalizeRefinement(ref *RefinementOptions) {
	if ref.ColorTries != nil {
		*ref.ColorTries = clampNonNegative(*ref.ColorTries)
	}
	if ref.AlphaTries != nil {
		*ref.AlphaTries = clampNonNegative(*ref.AlphaTries)
	}
	if ref.ColorStep != nil && *ref.ColorStep < 1 {
		*ref.ColorStep = 1
	}
	if ref.LSQIters != nil {
		*ref.LSQIters = clampNonNegative(*ref.LSQIters)
	}
}

// clampNonNegative clamps integer values below zero to zero.
func clampNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// getRGBWeightsFP returns fixed-point weights for index selection.
// If opts.RGBWeights is set, uses it;
// else when blockConstantR (e.g. BC3 nohq with R=0) returns Balanced;
// else Default.
func getRGBWeightsFP(opts *EncodeOptions, blockConstantR bool) rgbWeightsFP {
	if opts != nil {
		if opts.weightsFP != nil {
			return *opts.weightsFP
		}

		if opts.RGBWeights != nil {
			// Options that skipped normalizeEncodeOptions: convert on the spot.
			return fixedRGBWeights(opts.RGBWeights.R, opts.RGBWeights.G, opts.RGBWeights.B)
		}
	}

	if blockConstantR {
		return balancedWeightsFP
	}

	return defaultWeightsFP
}
