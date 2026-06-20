package bcn

import (
	"bytes"
	"testing"
)

// TestRoundRatioMatchesMathRoundCases checks the integer LSQ rounding helper
// against the half-away-from-zero behavior previously provided by math.Round.
func TestRoundRatioMatchesMathRoundCases(t *testing.T) {
	for _, tc := range []struct {
		num   int64
		denom int64
		want  int
	}{
		{num: 0, denom: 5, want: 0},
		{num: 2, denom: 5, want: 0},
		{num: 3, denom: 5, want: 1},
		{num: 5, denom: 10, want: 1},
		{num: -2, denom: 5, want: 0},
		{num: -3, denom: 5, want: -1},
		{num: -5, denom: 10, want: -1},
		{num: 2550, denom: 10, want: 255},
		{num: -2550, denom: 10, want: -255},
	} {
		if got := roundRatio(tc.num, tc.denom); got != tc.want {
			t.Fatalf("roundRatio(%d, %d) = %d, want %d", tc.num, tc.denom, got, tc.want)
		}
	}
}

// TestRefinementLSQItersOverride verifies that RefinementOptions.LSQIters is an effective, independent knob:
// disabling it changes the output and never beats the default (which includes LSQ),
// and it can run decoupled from the grid search (ColorTries=0) as a cheap polish-only mode.
func TestRefinementLSQItersOverride(t *testing.T) {
	const w, h = 64, 64
	rgba := goldenImage("opaque")

	encode := func(ref *RefinementOptions) []byte {
		t.Helper()
		opts := &EncodeOptions{QualityLevel: QualityLevelBalanced, AlphaThreshold: 128, Workers: 1, Refinement: ref}
		out, err := encodeBlocksWithOptions(rgba, w, h, FormatDXT1, opts)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		return out
	}

	psnrOf := func(encoded []byte) float64 {
		t.Helper()
		decoded, err := decodeBlocksWithOptions(encoded, w, h, FormatDXT1, &DecodeOptions{Workers: 1})
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return rgbPSNR(rgba, decoded)
	}

	off := 0
	defaultOut := encode(nil)
	offOut := encode(&RefinementOptions{LSQIters: &off})

	if bytes.Equal(defaultOut, offOut) {
		t.Fatal("LSQIters=0 produced identical output to default; override has no effect")
	}

	// The default path includes LSQ, so it must not be worse than LSQ disabled.
	if pDef, pOff := psnrOf(defaultOut), psnrOf(offOut); pDef < pOff-0.05 {
		t.Errorf("default PSNR %.4f dB below LSQ-disabled %.4f dB", pDef, pOff)
	}

	// Decoupled polish-only mode: no grid search, LSQ still refines past q1.
	gridOnly := 0
	lsqOnlyIters := 4
	q1Out, err := encodeBlocksWithOptions(rgba, w, h, FormatDXT1, &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128, Workers: 1})
	if err != nil {
		t.Fatalf("encode q1: %v", err)
	}
	lsqOnly := encode(&RefinementOptions{ColorTries: &gridOnly, LSQIters: &lsqOnlyIters})
	if pLSQ, pQ1 := psnrOf(lsqOnly), psnrOf(q1Out); pLSQ < pQ1 {
		t.Errorf("LSQ-only PSNR %.4f dB below no-refinement q1 %.4f dB", pLSQ, pQ1)
	}
}
