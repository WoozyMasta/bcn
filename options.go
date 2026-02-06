package bcn

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

// RGBWeights are used when choosing DXT1 palette indices (and in refinement).
// R, G, B are relative weights; they are normalized when used.
// Used to preserve channels that matter (e.g. blue in normal maps).
type RGBWeights struct {
	R, G, B float64
}

// Presets for RGBWeights when encoding DXT1/DXT5 RGB block.
var (
	// DefaultRGBWeights is luminance-oriented (green dominant). Use for typical photos/UI.
	DefaultRGBWeights = RGBWeights{R: 0.3, G: 0.6, B: 0.1}
	// BalancedRGBWeights treats R, G, B equally. Use when all channels matter (e.g. normal maps).
	BalancedRGBWeights = RGBWeights{R: 1.0 / 3.0, G: 1.0 / 3.0, B: 1.0 / 3.0}
)

// EncodeOptions configures block encoding and mipmap generation.
type EncodeOptions struct {
	// RGBWeights overrides weights for DXT1 palette index selection (R, G, B). Nil = default;
	// for DXT5, if nil and block has constant R (e.g. nohq), Balanced is used automatically.
	RGBWeights *RGBWeights
	// Refinement overrides quality behavior when non-nil (applied on top of QualityLevel).
	Refinement *RefinementOptions
	// qualitySettings is an internal cache of quality settings derived from QualityLevel and Refinement.
	qualitySettings *qualitySettings
	// QualityLevel provides a 1..10 quality scale. 0 = default (Balanced).
	// Recommended: 1=fast, 6=balanced, 8=best, 9-10=extreme.
	QualityLevel int
	// Workers controls parallel block encoding. 0 = auto (GOMAXPROCS), 1 = disable parallelism,
	// N > 1 = use N workers. Defaults to 0.
	Workers int
	// GenerateMipmaps enables mipmap generation from the input image.
	GenerateMipmaps bool
	// UseSRGB enables sRGB-aware downscale for mip generation.
	UseSRGB bool
	// AlphaThreshold controls DXT1 1-bit alpha cutout (0..255). Default 128.
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
}

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
	qs := resolveQualitySettings(out)
	out.qualitySettings = &qs

	return out
}

type qualitySettings struct {
	usePCA     bool
	colorTries int
	colorStep  int
	alphaTries int
}

func qualitySettingsForOpts(opts EncodeOptions) qualitySettings {
	if opts.qualitySettings != nil {
		return *opts.qualitySettings
	}
	return resolveQualitySettings(opts)
}

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
			step := *ref.ColorStep
			if step < 1 {
				step = 1
			}
			settings.colorStep = step
		}
	}

	return settings
}

func qualitySettingsFromLevel(level int) qualitySettings {
	switch level {
	case 1:
		return qualitySettings{usePCA: false, colorTries: 0, colorStep: 1, alphaTries: 0}
	case 2:
		return qualitySettings{usePCA: false, colorTries: 8, colorStep: 1, alphaTries: 8}
	case 3:
		return qualitySettings{usePCA: false, colorTries: 16, colorStep: 1, alphaTries: 16}
	case 4:
		return qualitySettings{usePCA: false, colorTries: 32, colorStep: 1, alphaTries: 32}
	case 5:
		return qualitySettings{usePCA: true, colorTries: 32, colorStep: 1, alphaTries: 32}
	case 6:
		return qualitySettings{usePCA: true, colorTries: 64, colorStep: 1, alphaTries: 64}
	case 7:
		return qualitySettings{usePCA: true, colorTries: 96, colorStep: 1, alphaTries: 96}
	case 8:
		return qualitySettings{usePCA: true, colorTries: 256, colorStep: 2, alphaTries: 256}
	case 9:
		return qualitySettings{usePCA: true, colorTries: 384, colorStep: 1, alphaTries: 384}
	case 10:
		return qualitySettings{usePCA: true, colorTries: 512, colorStep: 1, alphaTries: 512}
	default:
		return qualitySettingsFromLevel(QualityLevelBalanced)
	}
}

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
}

func clampNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// getRGBWeights returns (rw, gw, bw) for index selection. If opts.RGBWeights is set, uses it;
// else when blockConstantR (e.g. DXT5 nohq with R=0) returns Balanced; else Default.
func getRGBWeights(opts *EncodeOptions, blockConstantR bool) (rw, gw, bw float64) {
	if opts != nil && opts.RGBWeights != nil {
		w := opts.RGBWeights
		return w.R, w.G, w.B
	}

	if blockConstantR {
		return BalancedRGBWeights.R, BalancedRGBWeights.G, BalancedRGBWeights.B
	}

	return DefaultRGBWeights.R, DefaultRGBWeights.G, DefaultRGBWeights.B
}
