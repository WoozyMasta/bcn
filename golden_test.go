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
// deliberate metric changes regenerate them via BCN_GOLDEN_PRINT=1 together
// with a PSNR comparison documented in the change.
var goldenEncodeHashes = map[string]string{
	"DXT1/opaque/q1":       "08785ac09658d578",
	"DXT1/opaque/q2":       "307b4f355dac3fcb",
	"DXT1/opaque/q3":       "39f24c12352d9e9c",
	"DXT1/opaque/q4":       "7956fd1f4dbcd382",
	"DXT1/opaque/q5":       "dece114917a6b381",
	"DXT1/opaque/q6":       "d354ca144fcbfa88",
	"DXT1/opaque/q7":       "60d851ff8a1d8385",
	"DXT1/opaque/q8":       "b340e6caea81142a",
	"DXT1/opaque/q9":       "51e5c0f8ec22cf55",
	"DXT1/opaque/q10":      "e06bfebc092e647d",
	"DXT1/translucent/q1":  "0f2d56329e5adbaa",
	"DXT1/translucent/q2":  "5f2d57e6d928b916",
	"DXT1/translucent/q3":  "bc1dbf80ddab3d5d",
	"DXT1/translucent/q4":  "e592d18a2f4b417f",
	"DXT1/translucent/q5":  "1cd74cb24b191695",
	"DXT1/translucent/q6":  "5b9abc828799863f",
	"DXT1/translucent/q7":  "14182f2f814b13c4",
	"DXT1/translucent/q8":  "6728c9d0faeefced",
	"DXT1/translucent/q9":  "87e97ffcbfa77bd9",
	"DXT1/translucent/q10": "9e8b12a47c75a60f",
	"DXT3/translucent/q1":  "be323b6bbbe01aca",
	"DXT3/translucent/q2":  "7e8a5d158a15f086",
	"DXT3/translucent/q3":  "fbba24dba9f5ad8b",
	"DXT3/translucent/q4":  "a4dbe7f7b97237e2",
	"DXT3/translucent/q5":  "4d2794c4f202efc2",
	"DXT3/translucent/q6":  "0715858cbc683edf",
	"DXT3/translucent/q7":  "383586948c8182ac",
	"DXT3/translucent/q8":  "7ddfe27f99f3205a",
	"DXT3/translucent/q9":  "542bb61135ae2516",
	"DXT3/translucent/q10": "39532e68b5485e4c",
	"DXT5/translucent/q1":  "1ad1a03a5fa75a20",
	"DXT5/translucent/q2":  "2285024823843a80",
	"DXT5/translucent/q3":  "95e7c275f8066a10",
	"DXT5/translucent/q4":  "e22e07b639d65f80",
	"DXT5/translucent/q5":  "8376349fcca4de84",
	"DXT5/translucent/q6":  "0c1c4f40542062cf",
	"DXT5/translucent/q7":  "f9fd454daa6adb8e",
	"DXT5/translucent/q8":  "757ff3b7fd5fb59e",
	"DXT5/translucent/q9":  "ff54afe557cd78ce",
	"DXT5/translucent/q10": "7a3c2c3c97ce46c4",
	"DXT5/nohq/q1":         "8b732676827b3617",
	"DXT5/nohq/q2":         "22084c59121a08aa",
	"DXT5/nohq/q3":         "a38abcaff7f9fa17",
	"DXT5/nohq/q4":         "e7d7a016b7707b7b",
	"DXT5/nohq/q5":         "082af446ae42f3b4",
	"DXT5/nohq/q6":         "70e738da9e554037",
	"DXT5/nohq/q7":         "a76818cd5cdb5951",
	"DXT5/nohq/q8":         "b075299a75581482",
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
