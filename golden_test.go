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
	"DXT1/opaque/q2":       "49022b4011bc1ddd",
	"DXT1/opaque/q3":       "122400b9c696028a",
	"DXT1/opaque/q4":       "d765ea758656b179",
	"DXT1/opaque/q5":       "04abe32437d913d3",
	"DXT1/opaque/q6":       "086c9a5a46b258b7",
	"DXT1/opaque/q7":       "be3fb6ee9f0e8706",
	"DXT1/opaque/q8":       "3f5d21d232f1592f",
	"DXT1/opaque/q9":       "13cdb6ac1be82601",
	"DXT1/opaque/q10":      "c29052c9f02e1bcf",
	"DXT1/translucent/q1":  "0f2d56329e5adbaa",
	"DXT1/translucent/q2":  "f2cdc3180f61b389",
	"DXT1/translucent/q3":  "052c7b2e6d349c03",
	"DXT1/translucent/q4":  "59fc241c2e771f12",
	"DXT1/translucent/q5":  "90f13979ea874bdf",
	"DXT1/translucent/q6":  "1d064a543929425c",
	"DXT1/translucent/q7":  "b101a8ec39975c66",
	"DXT1/translucent/q8":  "7cafcda90ce77b87",
	"DXT1/translucent/q9":  "0632379ec630dff2",
	"DXT1/translucent/q10": "aec3701cdf5677ff",
	"DXT3/translucent/q1":  "b0b375a13743fd27",
	"DXT3/translucent/q2":  "c1b8b0061a98f19e",
	"DXT3/translucent/q3":  "e8bb7e07a2d0d946",
	"DXT3/translucent/q4":  "475793c0dfc0b713",
	"DXT3/translucent/q5":  "f585fe1ef5d219f7",
	"DXT3/translucent/q6":  "a9d917310f873da0",
	"DXT3/translucent/q7":  "40f10292f8497676",
	"DXT3/translucent/q8":  "a9cfa8e20b8418d2",
	"DXT3/translucent/q9":  "ab9643b36f91f24b",
	"DXT3/translucent/q10": "8e77c95ad95b244a",
	"DXT5/translucent/q1":  "c2af7a143698e692",
	"DXT5/translucent/q2":  "359843aeb5d1cd4c",
	"DXT5/translucent/q3":  "d2f61ad341876a40",
	"DXT5/translucent/q4":  "727da9ef9495f675",
	"DXT5/translucent/q5":  "e7963ee34b751b03",
	"DXT5/translucent/q6":  "70b3b8b6f8f29e15",
	"DXT5/translucent/q7":  "7272b3ab002aca55",
	"DXT5/translucent/q8":  "8863e14a635c2798",
	"DXT5/translucent/q9":  "35f00ab0bb2149d5",
	"DXT5/translucent/q10": "123cfc9ff11bde4c",
	"DXT5/nohq/q1":         "8b732676827b3617",
	"DXT5/nohq/q2":         "8372d844da2ec237",
	"DXT5/nohq/q3":         "fbde30eb8d508e1b",
	"DXT5/nohq/q4":         "53298e682d2921f3",
	"DXT5/nohq/q5":         "820c18adeb4c8df8",
	"DXT5/nohq/q6":         "a1de3e30e4a22a2a",
	"DXT5/nohq/q7":         "53af96172ee42de9",
	"DXT5/nohq/q8":         "e9505a3f2731bd7a",
	"DXT5/nohq/q9":         "016eb16aad2b1565",
	"DXT5/nohq/q10":        "016eb16aad2b1565",
	"BC4/opaque/q1":        "b63e66185a881fd6",
	"BC4/opaque/q2":        "4f4e3f564d05246c",
	"BC4/opaque/q3":        "4f4e3f564d05246c",
	"BC4/opaque/q4":        "4f4e3f564d05246c",
	"BC4/opaque/q5":        "4f4e3f564d05246c",
	"BC4/opaque/q6":        "4f4e3f564d05246c",
	"BC4/opaque/q7":        "4f4e3f564d05246c",
	"BC4/opaque/q8":        "778a4651318576fe",
	"BC4/opaque/q9":        "778a4651318576fe",
	"BC4/opaque/q10":       "778a4651318576fe",
	"BC5/opaque/q1":        "51bfd7a4de1a48c9",
	"BC5/opaque/q2":        "b735707000d9e609",
	"BC5/opaque/q3":        "b735707000d9e609",
	"BC5/opaque/q4":        "b735707000d9e609",
	"BC5/opaque/q5":        "b735707000d9e609",
	"BC5/opaque/q6":        "b735707000d9e609",
	"BC5/opaque/q7":        "b735707000d9e609",
	"BC5/opaque/q8":        "330bcfc40f2875fc",
	"BC5/opaque/q9":        "330bcfc40f2875fc",
	"BC5/opaque/q10":       "330bcfc40f2875fc",
	"BC7/opaque/q1":        "676bc14c0e70df41",
	"BC7/opaque/q2":        "b1ad2b4841e37807",
	"BC7/opaque/q3":        "b1ad2b4841e37807",
	"BC7/opaque/q4":        "409f6a3c7c8b2014",
	"BC7/opaque/q5":        "409f6a3c7c8b2014",
	"BC7/opaque/q6":        "409f6a3c7c8b2014",
	"BC7/opaque/q7":        "7805df4f16c372d7",
	"BC7/opaque/q8":        "7805df4f16c372d7",
	"BC7/opaque/q9":        "5ad74b5bca019cbd",
	"BC7/opaque/q10":       "bcd50e8476268b1f",
	"BC7/translucent/q1":   "80f1cd8d9ca86319",
	"BC7/translucent/q2":   "ae9d77f67fd841cc",
	"BC7/translucent/q3":   "ae9d77f67fd841cc",
	"BC7/translucent/q4":   "26ba2779f17248a6",
	"BC7/translucent/q5":   "26ba2779f17248a6",
	"BC7/translucent/q6":   "26ba2779f17248a6",
	"BC7/translucent/q7":   "5b5765fe46034ec5",
	"BC7/translucent/q8":   "5b5765fe46034ec5",
	"BC7/translucent/q9":   "6022f0f208fdd6f6",
	"BC7/translucent/q10":  "50d0ad8a1c111653",
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
		{FormatBC7, "opaque"},
		{FormatBC7, "translucent"},
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
