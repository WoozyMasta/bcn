package bcn

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// goldenEncodeHashes freezes encoder output byte-exactly across formats and quality levels.
// Output-preserving optimizations must keep these hashes;
// deliberate metric changes regenerate them via BCN_GOLDEN_PRINT=1
// together with a PSNR comparison documented in the change.
var goldenEncodeHashes = map[string]string{
	"DXT1/opaque/q1":       "645bebcb02564fba",
	"DXT1/opaque/q2":       "308ba92f46ad7ce5",
	"DXT1/opaque/q3":       "e0cb4204f6ac0720",
	"DXT1/opaque/q4":       "8e4efd57718e18ec",
	"DXT1/opaque/q5":       "8847106356687114",
	"DXT1/opaque/q6":       "d3f4d0b3d044a63d",
	"DXT1/opaque/q7":       "442920d090bd3457",
	"DXT1/opaque/q8":       "9c00bf351ded34fb",
	"DXT1/opaque/q9":       "704b7f86ba926c7a",
	"DXT1/opaque/q10":      "e1be00d260684f6e",
	"DXT1/translucent/q1":  "0f2d56329e5adbaa",
	"DXT1/translucent/q2":  "fd8c89ed385e6125",
	"DXT1/translucent/q3":  "658af55a0ea7a30f",
	"DXT1/translucent/q4":  "032bd84d726c0920",
	"DXT1/translucent/q5":  "1703fa9b9f4b2510",
	"DXT1/translucent/q6":  "ef0ad38a2f90be79",
	"DXT1/translucent/q7":  "977ec053827960dc",
	"DXT1/translucent/q8":  "6728c9d0faeefced",
	"DXT1/translucent/q9":  "da3e487da956c336",
	"DXT1/translucent/q10": "6cae4059027db7e9",
	"DXT3/translucent/q1":  "b0b375a13743fd27",
	"DXT3/translucent/q2":  "a8440443142956fc",
	"DXT3/translucent/q3":  "5d44a6dc171aa665",
	"DXT3/translucent/q4":  "2c30d822b7c4f4af",
	"DXT3/translucent/q5":  "2c33efecbef73dd6",
	"DXT3/translucent/q6":  "ec1ad9dd0fa7b0db",
	"DXT3/translucent/q7":  "a68567c5ccfc3c6b",
	"DXT3/translucent/q8":  "02c390c7346ee007",
	"DXT3/translucent/q9":  "374bd676ddd26d58",
	"DXT3/translucent/q10": "de96b15f60dbba00",
	"DXT5/translucent/q1":  "c2af7a143698e692",
	"DXT5/translucent/q2":  "52a97fef3332b912",
	"DXT5/translucent/q3":  "739de7475dde47bd",
	"DXT5/translucent/q4":  "b214d3abf81be023",
	"DXT5/translucent/q5":  "f39dba76ec694d83",
	"DXT5/translucent/q6":  "ef658f9ea6e17601",
	"DXT5/translucent/q7":  "35b95394347dae37",
	"DXT5/translucent/q8":  "5c17086a7ff6d3e5",
	"DXT5/translucent/q9":  "cf750aa80ddd8864",
	"DXT5/translucent/q10": "26556c5c246ee487",
	"DXT5/nohq/q1":         "8b732676827b3617",
	"DXT5/nohq/q2":         "7a6fd08f473c3343",
	"DXT5/nohq/q3":         "a38abcaff7f9fa17",
	"DXT5/nohq/q4":         "e7d7a016b7707b7b",
	"DXT5/nohq/q5":         "082af446ae42f3b4",
	"DXT5/nohq/q6":         "70e738da9e554037",
	"DXT5/nohq/q7":         "a76818cd5cdb5951",
	"DXT5/nohq/q8":         "b1dd2afec71488a6",
	"DXT5/nohq/q9":         "e7cc8af4a7ddd414",
	"DXT5/nohq/q10":        "e7cc8af4a7ddd414",
	"BC4/opaque/q1":        "b63e66185a881fd6",
	"BC4/opaque/q2":        "f7d7823e1ae78ae5",
	"BC4/opaque/q3":        "f7d7823e1ae78ae5",
	"BC4/opaque/q4":        "f7d7823e1ae78ae5",
	"BC4/opaque/q5":        "f7d7823e1ae78ae5",
	"BC4/opaque/q6":        "f7d7823e1ae78ae5",
	"BC4/opaque/q7":        "f7d7823e1ae78ae5",
	"BC4/opaque/q8":        "f7d7823e1ae78ae5",
	"BC4/opaque/q9":        "f7d7823e1ae78ae5",
	"BC4/opaque/q10":       "f7d7823e1ae78ae5",
	"BC5/opaque/q1":        "51bfd7a4de1a48c9",
	"BC5/opaque/q2":        "51f4eb7f9f962550",
	"BC5/opaque/q3":        "51f4eb7f9f962550",
	"BC5/opaque/q4":        "51f4eb7f9f962550",
	"BC5/opaque/q5":        "51f4eb7f9f962550",
	"BC5/opaque/q6":        "51f4eb7f9f962550",
	"BC5/opaque/q7":        "51f4eb7f9f962550",
	"BC5/opaque/q8":        "51f4eb7f9f962550",
	"BC5/opaque/q9":        "51f4eb7f9f962550",
	"BC5/opaque/q10":       "51f4eb7f9f962550",
}

// goldenImage returns a deterministic 64x64 test image per scenario.
func goldenImage(scenario string) []byte {
	const w, h = 64, 64
	rgba := make([]byte, w*h*4)
	idx := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch scenario {
			case "opaque":
				rgba[idx+0] = uint8((x*13 + y*7 + 11) & 0xFF)
				rgba[idx+1] = uint8((x*3 + y*11 + 29) & 0xFF)
				rgba[idx+2] = uint8((x*17 + y*5 + 71) & 0xFF)
				rgba[idx+3] = 255

			case "translucent":
				rgba[idx+0] = uint8((x*13 + y*7 + 11) & 0xFF)
				rgba[idx+1] = uint8((x*3 + y*11 + 29) & 0xFF)
				rgba[idx+2] = uint8((x*17 + y*5 + 71) & 0xFF)
				rgba[idx+3] = uint8((x*5 + y*9 + 101) & 0xFF)

			case "nohq":
				// Constant red channel: triggers BalancedRGBWeights path.
				rgba[idx+0] = 0
				rgba[idx+1] = uint8((x*3 + y*11 + 29) & 0xFF)
				rgba[idx+2] = uint8((x*17 + y*5 + 71) & 0xFF)
				rgba[idx+3] = uint8((x*5 + y*9 + 101) & 0xFF)
			}
			idx += 4
		}
	}

	return rgba
}

func goldenCases() []struct {
	name     string
	format   Format
	scenario string
	quality  int
} {
	formats := []struct {
		format   Format
		scenario string
	}{
		{FormatDXT1, "opaque"},
		{FormatDXT1, "translucent"},
		{FormatDXT3, "translucent"},
		{FormatDXT5, "translucent"},
		{FormatDXT5, "nohq"},
		{FormatBC4, "opaque"},
		{FormatBC5, "opaque"},
	}

	var cases []struct {
		name     string
		format   Format
		scenario string
		quality  int
	}

	for _, f := range formats {
		for q := 1; q <= 10; q++ {
			cases = append(cases, struct {
				name     string
				format   Format
				scenario string
				quality  int
			}{
				name:     fmt.Sprintf("%s/%s/q%d", f.format, f.scenario, q),
				format:   f.format,
				scenario: f.scenario,
				quality:  q,
			})
		}
	}

	return cases
}

func goldenHash(t *testing.T, format Format, scenario string, quality int) string {
	t.Helper()
	rgba := goldenImage(scenario)
	opts := &EncodeOptions{QualityLevel: quality, AlphaThreshold: 128, Workers: 1}
	encoded, err := encodeBlocksWithOptions(rgba, 64, 64, format, opts)
	if err != nil {
		t.Fatalf("encode %s/%s/q%d: %v", format, scenario, quality, err)
	}

	decoded, err := decodeBlocksWithOptions(encoded, 64, 64, format, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("decode %s/%s/q%d: %v", format, scenario, quality, err)
	}

	sum := sha256.Sum256(append(encoded, decoded...))
	return hex.EncodeToString(sum[:8])
}

// TestGoldenEncodeOutput compares encoder+decoder output hashes with the frozen reference.
// Run with BCN_GOLDEN_PRINT=1 to print a fresh map literal.
func TestGoldenEncodeOutput(t *testing.T) {
	if os.Getenv("BCN_GOLDEN_PRINT") != "" {
		for _, c := range goldenCases() {
			fmt.Printf("\t%q: %q,\n", c.name, goldenHash(t, c.format, c.scenario, c.quality))
		}
		t.Skip("printed golden hashes")
	}

	if len(goldenEncodeHashes) == 0 {
		t.Skip("golden hashes not recorded yet")
	}

	for _, c := range goldenCases() {
		got := goldenHash(t, c.format, c.scenario, c.quality)
		want, ok := goldenEncodeHashes[c.name]
		if !ok {
			t.Errorf("%s: no golden hash recorded", c.name)
			continue
		}
		if got != want {
			t.Errorf("%s: output hash %s, want %s", c.name, got, want)
		}
	}
}
