package bcn

// Quality controls the encoder speed/quality tradeoff.
//
// Fast uses a simple bounding-box fit.
// Balanced uses PCA + limited refinement.
// Best adds additional refinement for lower error.
type Quality int

const (
	// QualityFast prioritizes speed over quality.
	QualityFast Quality = iota
	// QualityBalanced is the default, balancing speed and quality.
	QualityBalanced
	// QualityBest prioritizes quality and can be slower.
	QualityBest
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
	// Quality selects the encoder mode. Defaults to QualityBalanced.
	Quality Quality
	// GenerateMipmaps enables mipmap generation from the input image.
	GenerateMipmaps bool
	// UseSRGB enables sRGB-aware downscale for mip generation.
	UseSRGB bool
	// AlphaThreshold controls DXT1 1-bit alpha cutout (0..255). Default 128.
	AlphaThreshold uint8
	// Workers controls parallel block encoding. 0 = auto (GOMAXPROCS), 1 = disable parallelism,
	// N > 1 = use N workers. Defaults to 0.
	Workers int
}

func normalizeEncodeOptions(opts *EncodeOptions) EncodeOptions {
	if opts == nil {
		return EncodeOptions{Quality: QualityBalanced, AlphaThreshold: 128}
	}

	out := *opts
	if out.AlphaThreshold == 0 {
		out.AlphaThreshold = 128
	}
	if out.Quality < QualityFast || out.Quality > QualityBest {
		out.Quality = QualityBalanced
	}

	return out
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
