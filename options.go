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

// EncodeOptions configures block encoding and mipmap generation.
type EncodeOptions struct {
	// Quality selects the encoder mode. Defaults to QualityBalanced.
	Quality Quality
	// GenerateMipmaps enables mipmap generation from the input image.
	GenerateMipmaps bool
	// UseSRGB enables sRGB-aware downscale for mip generation.
	UseSRGB bool
	// AlphaThreshold controls DXT1 1-bit alpha cutout (0..255). Default 128.
	AlphaThreshold uint8
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
