package bcn

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// goldenEncodeHashes freezes encoder output byte-exactly across formats and
// quality levels. Output-preserving optimizations must keep these hashes;
// deliberate metric changes regenerate them via BCN_GOLDEN_PRINT=1 together
// with a PSNR comparison documented in the change.
var goldenEncodeHashes = map[string]string{}

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

// TestGoldenEncodeOutput compares encoder+decoder output hashes with the
// frozen reference. Run with BCN_GOLDEN_PRINT=1 to print a fresh map literal.
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
