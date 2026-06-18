package bcn

import (
	"fmt"
	"math"
	"os"
	"testing"
)

// psnrFloorsDB freezes RGB PSNR (dB) of encode->decode round trips measured on
// the float64 metric baseline (master). The fixed-point metric and any later
// optimization must not drop quality below these floors minus psnrToleranceDB.
// Regenerate with BCN_PSNR_PRINT=1 when the metric changes deliberately.
var psnrFloorsDB = map[string]float64{
	"DXT1/opaque/q1":      17.8834,
	"DXT1/opaque/q6":      20.4653,
	"DXT1/opaque/q8":      20.6061,
	"DXT1/translucent/q1": 7.4972,
	"DXT1/translucent/q6": 7.5854,
	"DXT1/translucent/q8": 7.5890,
	"DXT3/translucent/q1": 17.8834,
	"DXT3/translucent/q6": 20.4653,
	"DXT3/translucent/q8": 20.6061,
	"DXT5/translucent/q1": 17.8834,
	"DXT5/translucent/q6": 20.4653,
	"DXT5/translucent/q8": 20.6061,
	"DXT5/nohq/q1":        23.1445,
	"DXT5/nohq/q6":        25.4902,
	"DXT5/nohq/q8":        25.7045,
	"BC7/opaque/q1":       23.6610,
	"BC7/opaque/q6":       31.0990,
	"BC7/opaque/q8":       31.6984,
	"BC7/translucent/q1":  22.6344,
	"BC7/translucent/q6":  23.5320,
	"BC7/translucent/q8":  23.5320,
}

const psnrToleranceDB = 0.05

// psnrCases returns the encoder quality scenarios tracked for PSNR.
func psnrCases() []struct {
	format   Format
	scenario string
	quality  int
} {
	var cases []struct {
		format   Format
		scenario string
		quality  int
	}
	add := func(format Format, scenario string) {
		for _, q := range []int{QualityLevelFast, QualityLevelBalanced, QualityLevelBest} {
			cases = append(cases, struct {
				format   Format
				scenario string
				quality  int
			}{format, scenario, q})
		}
	}

	add(FormatDXT1, "opaque")
	add(FormatDXT1, "translucent")
	add(FormatDXT3, "translucent")
	add(FormatDXT5, "translucent")
	add(FormatDXT5, "nohq")
	add(FormatBC7, "opaque")
	add(FormatBC7, "translucent")

	return cases
}

// rgbPSNR computes PSNR over RGB bytes (alpha excluded) of two RGBA buffers.
func rgbPSNR(a, b []byte) float64 {
	var sse, n int64
	for i := 0; i < len(a); i += 4 {
		for c := range 3 {
			d := int64(a[i+c]) - int64(b[i+c])
			sse += d * d
			n++
		}
	}

	if sse == 0 {
		return math.Inf(1)
	}

	mse := float64(sse) / float64(n)
	return 10 * math.Log10(255*255/mse)
}

// TestEncodePSNR guards encode quality against the recorded floors.
// Run with BCN_PSNR_PRINT=1 to print a fresh map literal.
func TestEncodePSNR(t *testing.T) {
	printMode := os.Getenv("BCN_PSNR_PRINT") != ""
	if !printMode && len(psnrFloorsDB) == 0 {
		t.Skip("PSNR floors not recorded yet")
	}

	for _, c := range psnrCases() {
		name := fmt.Sprintf("%s/%s/q%d", c.format, c.scenario, c.quality)
		rgba := goldenImage(c.scenario)
		opts := &EncodeOptions{QualityLevel: c.quality, AlphaThreshold: 128, Workers: 1}

		encoded, err := encodeBlocksWithOptions(rgba, 64, 64, c.format, opts)
		if err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
		decoded, err := decodeBlocksWithOptions(encoded, 64, 64, c.format, &DecodeOptions{Workers: 1})
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}

		got := rgbPSNR(rgba, decoded)
		if printMode {
			fmt.Printf("\t%q: %.4f,\n", name, got)
			continue
		}

		want, ok := psnrFloorsDB[name]
		if !ok {
			t.Errorf("%s: no PSNR floor recorded", name)
			continue
		}
		if got < want-psnrToleranceDB {
			t.Errorf("%s: PSNR %.4f dB below floor %.4f dB", name, got, want)
		}
	}

	if printMode {
		t.Skip("printed PSNR values")
	}
}
