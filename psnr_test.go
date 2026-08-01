package bcn

import (
	"fmt"
	"math"
	"os"
	"testing"
)

// psnrFloorsDB freezes RGB PSNR (dB) of encode->decode round trips
// measured on the float64 metric baseline (master).
// The fixed-point metric and any later optimization must not drop quality
// below these floors minus psnrToleranceDB.
// Regenerate with BCN_PSNR_PRINT=1 when the metric changes deliberately.
var psnrFloorsDB = map[string]float64{
	"BC1/opaque/q1":      17.8834,
	"BC1/opaque/q6":      20.4653,
	"BC1/opaque/q8":      20.6061,
	"BC1/translucent/q1": 7.4972,
	"BC1/translucent/q6": 7.5854,
	"BC1/translucent/q8": 7.5890,
	"BC2/translucent/q1": 17.8834,
	"BC2/translucent/q6": 20.4653,
	"BC2/translucent/q8": 20.6061,
	"BC3/translucent/q1": 17.8834,
	"BC3/translucent/q6": 20.4653,
	"BC3/translucent/q8": 20.6061,
	"BC3/nohq/q1":        23.1445,
	"BC3/nohq/q6":        25.4902,
	"BC3/nohq/q8":        25.7045,
	"BC7/opaque/q1":      23.6610,
	"BC7/opaque/q6":      34.6708,
	"BC7/opaque/q8":      35.2756,
	"BC7/translucent/q1": 22.6344,
	"BC7/translucent/q6": 30.5639,
	"BC7/translucent/q8": 30.4949,
	"BC6HU/UF16/q1":      43.0175,
	"BC6HU/UF16/q6":      55.6059,
	"BC6HU/UF16/q8":      55.9995,
	"BC6HS/SF16/q1":      42.9792,
	"BC6HS/SF16/q6":      56.3256,
	"BC6HS/SF16/q8":      56.7639,
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

	add(FormatBC1, "opaque")
	add(FormatBC1, "translucent")
	add(FormatBC2, "translucent")
	add(FormatBC3, "translucent")
	add(FormatBC3, "nohq")
	add(FormatBC7, "opaque")
	add(FormatBC7, "translucent")
	add(FormatBC6HU, "UF16")
	add(FormatBC6HS, "SF16")

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
		opts := &EncodeOptions{QualityLevel: c.quality, AlphaThreshold: 128, Workers: 1}

		var got float64
		if c.format == FormatBC6HU || c.format == FormatBC6HS {
			signed := c.format == FormatBC6HS
			src := goldenHDRImage()
			encoded, err := EncodeBC6HWithOptions(src, 64, 64, signed, opts)
			if err != nil {
				t.Fatalf("encode %s: %v", name, err)
			}
			decoded, err := DecodeBC6H(encoded, 64, 64, signed)
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			got = bc6hPSNR(src, decoded, signed)
		} else {
			rgba := goldenImage(c.scenario)
			encoded, err := encodeBlocksWithOptions(rgba, 64, 64, c.format, opts)
			if err != nil {
				t.Fatalf("encode %s: %v", name, err)
			}
			decoded, err := decodeBlocksWithOptions(encoded, 64, 64, c.format, &DecodeOptions{Workers: 1})
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			got = rgbPSNR(rgba, decoded)
		}
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
