package bcn

import (
	"bytes"
	"image"
	"testing"
)

// roundTripPSNR encodes and decodes one buffer and returns its RGB PSNR.
func roundTripPSNR(t *testing.T, rgba []byte, w, h int, format Format) float64 {
	t.Helper()
	enc, err := encodeBlocksWithOptions(rgba, w, h, format, &EncodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("%s encode: %v", format, err)
	}
	dec, err := decodeBlocksWithOptions(enc, w, h, format, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("%s decode: %v", format, err)
	}
	return rgbPSNR(rgba, dec)
}

// TestBC7EncodeQuality verifies mode 6 reaches high fidelity
// on a smooth block (where a single subset is sufficient).
func TestBC7EncodeQuality(t *testing.T) {
	var rgba [64]byte
	for i := range 16 {
		rgba[i*4+0] = uint8(i * 4)
		rgba[i*4+1] = uint8(i * 3)
		rgba[i*4+2] = uint8(100 + i*2)
		rgba[i*4+3] = 255
	}
	if p := roundTripPSNR(t, rgba[:], 4, 4, FormatBC7); p < 45 {
		t.Errorf("smooth block PSNR %.2f dB too low", p)
	}
}

// bc7DecodeSSE decodes a packed block and returns its RGBA error against the source,
// used to confirm an encoder's reported error matches the decoder.
func bc7DecodeSSE(block [16]rgba8, enc [16]byte) int {
	dec := decodeBlockBC7(enc[:])
	total := 0
	for i := range 16 {
		o := i * 4
		total += bc7SSE(block[i], rgba8{dec[o], dec[o+1], dec[o+2], dec[o+3]})
	}
	return total
}

// TestBC7EncoderSelfConsistent checks that each mode encoder's
// reported error equals the error of its own packed block after decoding.
// A mismatch means the packing or quantization disagrees with the decoder.
func TestBC7EncoderSelfConsistent(t *testing.T) {
	var grad, region, alpha [16]rgba8
	for i := range 16 {
		grad[i] = rgba8{uint8(i * 16), uint8(i * 8), uint8(100 + i*4), 255}
		region[i] = rgba8{50, 50, 200, 255}
		if i%4 < 2 {
			region[i] = rgba8{200, 50, 50, 255}
		}
		alpha[i] = rgba8{uint8(i * 12), 80, uint8(200 - i*10), uint8(i * 16)}
	}
	blocks := map[string][16]rgba8{"gradient": grad, "region": region, "alpha": alpha}

	for name, blk := range blocks {
		rank2Order := bc7Rank2SubsetN(&blk, 16)

		if b, e := encodeBC7Mode6(blk); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode6: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e := encodeBC7Mode5(blk, 4); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode5: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode1WithOrder(blk, rank2Order, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode1: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode7(blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode7: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode02(bc7Mode0Params, blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode0: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode02(bc7Mode2Params, blk, 64); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode2: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode3WithOrder(blk, rank2Order, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode3: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e := encodeBC7Mode4(blk, 4); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode4: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
	}
}

// TestBC7Mode6IndicesASMMatchesScalar verifies that
// the AVX2 mode 6 nearest-index kernel preserves scalar tie-breaking
// and total-error semantics.
func TestBC7Mode6IndicesASMMatchesScalar(t *testing.T) {
	block := benchmarkBlockAlpha()
	q0, p0 := bc7QuantizeMode6(rgba8{r: 8, g: 32, b: 80, a: 16})
	q1, p1 := bc7QuantizeMode6(rgba8{r: 240, g: 210, b: 170, a: 230})
	pal := bc7Mode6Palette(bc7ExpandMode6(q0, p0), bc7ExpandMode6(q1, p1))

	wantIdx, wantErr := bc7Mode6IndicesScalarForTest(block, &pal)
	gotIdx, gotErr, ok := bc7Mode6IndicesASM(&block, &pal)
	if !ok {
		t.Skip("BC7 mode 6 AVX2 kernel unavailable")
	}

	if gotErr != wantErr {
		t.Fatalf("total error: got %d want %d", gotErr, wantErr)
	}
	if gotIdx != wantIdx {
		t.Fatalf("indices: got %v want %v", gotIdx, wantIdx)
	}
}

// bc7Mode6IndicesScalarForTest mirrors the scalar mode 6 assignment loop
// so the assembly wrapper can be tested without calling the accelerated path.
func bc7Mode6IndicesScalarForTest(block [16]rgba8, pal *[16]rgba8) ([16]uint8, int) {
	var idx [16]uint8
	total := 0
	for i, px := range block {
		best := 0
		bestErr := bc7SSE(px, pal[0])
		for k := 1; k < 16; k++ {
			if e := bc7SSE(px, pal[k]); e < bestErr {
				bestErr = e
				best = k
			}
		}
		idx[i] = uint8(best) // #nosec G115 -- best is in [0,15].
		total += bestErr
	}
	return idx, total
}

// TestBC7Color4LSQASMMatchesScalar verifies
// the shared 4-entry RGB LSQ wrapper used by BC7 mode 5 color fitting.
func TestBC7Color4LSQASMMatchesScalar(t *testing.T) {
	block := benchmarkBlockAlpha()
	pal := bc7Color5Palette(
		bc7ExpandColor5(bc7QuantColor5(rgba8{r: 8, g: 32, b: 80})),
		bc7ExpandColor5(bc7QuantColor5(rgba8{r: 240, g: 210, b: 170})),
	)

	want := bc7Color4LSQScalarSumsForTest(&block, &pal)
	got, ok := bc7Color4LSQASM(&block, &pal)
	if !ok {
		t.Skip("BC7 color4 AVX2 LSQ kernel unavailable")
	}

	if got != want {
		t.Fatalf("LSQ sums: got %+v want %+v", got, want)
	}
}

// bc7Color4LSQScalarSumsForTest mirrors BC7 mode 5 color LSQ accumulation
// so the assembly wrapper can be checked independently from the production path.
func bc7Color4LSQScalarSumsForTest(block *[16]rgba8, pal *[4]rgba8) lsqColorSums {
	var sums lsqColorSums
	for i := range 16 {
		idx, _ := bc7Color5Nearest(block[i], pal)
		b := int(bc7Weight2[idx])
		a := 64 - b
		sums.saa += a * a
		sums.sbb += b * b
		sums.sab += a * b
		sums.sapR += a * int(block[i].r)
		sums.sapG += a * int(block[i].g)
		sums.sapB += a * int(block[i].b)
		sums.sbpR += b * int(block[i].r)
		sums.sbpG += b * int(block[i].g)
		sums.sbpB += b * int(block[i].b)
	}
	return sums
}

// TestBC7SubsetEvalASMMatchesScalar verifies the shared partition-mode kernel:
// for each subset it must reproduce the scalar least-squares sums
// and the total nearest-entry error,
// for both an 8-entry (mode 1/0) and a 4-entry (mode 3/2) palette
// (the latter exercises the entry-0 padding path).
func TestBC7SubsetEvalASMMatchesScalar(t *testing.T) {
	block := benchmarkBlockOpaque()
	pal8 := bc7Mode1Palette(rgba8{r: 20, g: 40, b: 60, a: 255}, rgba8{r: 200, g: 180, b: 150, a: 255})
	pal4 := bc7Mode3Palette(rgba8{r: 30, g: 70, b: 110, a: 255}, rgba8{r: 210, g: 160, b: 120, a: 255})
	cases := []struct {
		name    string
		pal     []rgba8
		weights []int32
	}{
		{name: "8-entry", pal: pal8[:], weights: bc7Weight3[:]},
		{name: "4-entry", pal: pal4[:], weights: bc7Weight2[:]},
	}

	for _, p := range []int{0, 13, 34} {
		part := bc7PartitionSets[0][p]
		for _, tc := range cases {
			for _, subset := range []uint8{0, 1} {
				wantSums, wantErr := bc7SubsetEvalScalarForTest(&block, &part, subset, tc.pal, tc.weights)
				gotSums, gotErr, ok := bc7SubsetEvalASM(&block, &part, subset, tc.pal, tc.weights)
				if !ok {
					t.Skip("BC7 subset-eval AVX2 kernel unavailable")
				}

				if gotSums != wantSums {
					t.Fatalf("p=%d %s subset=%d sums: got %+v want %+v", p, tc.name, subset, gotSums, wantSums)
				}
				if gotErr != wantErr {
					t.Fatalf("p=%d %s subset=%d error: got %d want %d", p, tc.name, subset, gotErr, wantErr)
				}
			}
		}
	}
}

// bc7SubsetEvalScalarForTest mirrors the partition-mode kernel:
// masked nearest-of-N RGB search with least-squares accumulation and error sum.
func bc7SubsetEvalScalarForTest(block *[16]rgba8, part *[16]uint8, subset uint8, pal []rgba8, weights []int32) (lsqColorSums, int) {
	var s lsqColorSums
	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7RGBErr(block[i], pal[0])
		for k := 1; k < len(pal); k++ {
			if e := bc7RGBErr(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

		b := 0
		if idx < len(weights) {
			b = int(weights[idx])
		}
		a := 64 - b
		s.saa += a * a
		s.sbb += b * b
		s.sab += a * b
		s.sapR += a * int(block[i].r)
		s.sapG += a * int(block[i].g)
		s.sapB += a * int(block[i].b)
		s.sbpR += b * int(block[i].r)
		s.sbpG += b * int(block[i].g)
		s.sbpB += b * int(block[i].b)
		total += bestErr
	}

	return s, total
}

// TestBC7Mode7SubsetEvalASMMatchesScalar verifies the RGBA partition-mode kernel
// reproduces the scalar mode 7 least-squares sums and total error per subset.
func TestBC7Mode7SubsetEvalASMMatchesScalar(t *testing.T) {
	block := benchmarkBlockAlpha()
	pal := bc7Mode7Palette(rgba8{r: 20, g: 40, b: 60, a: 30}, rgba8{r: 200, g: 180, b: 150, a: 220})

	for _, p := range []int{0, 13, 34} {
		part := bc7PartitionSets[0][p]
		for _, subset := range []uint8{0, 1} {
			wantSums, wantErr := bc7Mode7SubsetEvalScalarForTest(&block, &part, subset, &pal)
			gotSums, gotErr, ok := bc7Mode7SubsetEvalASM(&block, &part, subset, &pal)
			if !ok {
				t.Skip("BC7 mode 7 subset-eval AVX2 kernel unavailable")
			}

			if gotSums != wantSums {
				t.Fatalf("p=%d subset=%d sums: got %+v want %+v", p, subset, gotSums, wantSums)
			}
			if gotErr != wantErr {
				t.Fatalf("p=%d subset=%d error: got %d want %d", p, subset, gotErr, wantErr)
			}
		}
	}
}

// bc7Mode7SubsetEvalScalarForTest mirrors the mode 7 kernel:
// masked nearest-of-4 RGBA search with full RGBA least-squares accumulation and error sum.
func bc7Mode7SubsetEvalScalarForTest(block *[16]rgba8, part *[16]uint8, subset uint8, pal *[4]rgba8) (bc7Mode7Sums, int) {
	var s bc7Mode7Sums
	total := 0
	for i := range 16 {
		if part[i]&0x03 != subset {
			continue
		}

		idx := 0
		bestErr := bc7SSE(block[i], pal[0])
		for k := 1; k < 4; k++ {
			if e := bc7SSE(block[i], pal[k]); e < bestErr {
				bestErr, idx = e, k
			}
		}

		b := int(bc7Weight2[idx])
		a := 64 - b
		s.saa += a * a
		s.sbb += b * b
		s.sab += a * b
		ch := [4]int{int(block[i].r), int(block[i].g), int(block[i].b), int(block[i].a)}
		for c := range 4 {
			s.sap[c] += a * ch[c]
			s.sbp[c] += b * ch[c]
		}
		total += bestErr
	}

	return s, total
}

// TestBC7Rank2SubsetNMatchesFullPrefix verifies that bounded two-subset ranking
// preserves the exact candidate order used by the full sort.
func TestBC7Rank2SubsetNMatchesFullPrefix(t *testing.T) {
	block := benchmarkBlockAlpha()
	full := bc7Rank2SubsetN(&block, 64)

	for _, n := range []int{0, 1, 4, 8, 16, 32, 64, 128} {
		got := bc7Rank2SubsetN(&block, n)
		limit := min(max(n, 0), 64)
		for i := range limit {
			if got[i] != full[i] {
				t.Fatalf("n=%d index=%d: got partition %d want %d", n, i, got[i], full[i])
			}
		}
	}
}

// TestBC7Rank3SubsetNMatchesFullPrefix verifies that bounded three-subset ranking
// preserves the exact candidate order for both mode 0 and mode 2 tables.
func TestBC7Rank3SubsetNMatchesFullPrefix(t *testing.T) {
	block := benchmarkBlockOpaque()

	for _, numParts := range []int{16, 64} {
		full := bc7Rank3SubsetN(&block, numParts, numParts)
		for _, n := range []int{0, 1, 4, 8, 16, 32, 64, 128} {
			got := bc7Rank3SubsetN(&block, numParts, n)
			limit := min(min(max(n, 0), numParts), 64)
			for i := range limit {
				if got[i] != full[i] {
					t.Fatalf("numParts=%d n=%d index=%d: got partition %d want %d", numParts, n, i, got[i], full[i])
				}
			}
		}
	}
}

// TestBC7EncodeTwoRegions checks that the partition modes fit
// a block with two distinct flat color regions (which a single subset cannot),
// reaching near-lossless quality.
func TestBC7EncodeTwoRegions(t *testing.T) {
	const w, h = 4, 4
	rgba := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
			if x < 2 {
				rgba[i+0], rgba[i+1], rgba[i+2] = 200, 50, 50
			} else {
				rgba[i+0], rgba[i+1], rgba[i+2] = 50, 50, 200
			}
			rgba[i+3] = 255
		}
	}
	if p := roundTripPSNR(t, rgba, w, h, FormatBC7); p < 45 {
		t.Errorf("two-region block PSNR %.2f dB too low (partition mode not selected?)", p)
	}
}

// TestBC7EncodeBeatsBC3 asserts BC7 is at least as good as BC3 on the same content
// (BC7 mode 6 has 8-bit endpoints and 4-bit indices vs BC1-style color).
func TestBC7EncodeBeatsBC3(t *testing.T) {
	const w, h = 64, 64
	for _, scenario := range []string{"opaque", "translucent"} {
		rgba := goldenImage(scenario)
		bc7 := roundTripPSNR(t, rgba, w, h, FormatBC7)
		bc3 := roundTripPSNR(t, rgba, w, h, FormatBC3)
		if bc7 < bc3-0.25 {
			t.Errorf("%s: BC7 PSNR %.2f dB below BC3 %.2f dB", scenario, bc7, bc3)
		}
	}
}

func TestBC7EncodeSolid(t *testing.T) {
	const w, h = 8, 8
	rgba := make([]byte, w*h*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i+0], rgba[i+1], rgba[i+2], rgba[i+3] = 200, 100, 60, 255
	}
	enc, err := encodeBlocksWithOptions(rgba, w, h, FormatBC7, &EncodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec, err := decodeBlocksWithOptions(enc, w, h, FormatBC7, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(dec); i += 4 {
		if dec[i] != 200 || dec[i+1] != 100 || dec[i+2] != 60 {
			t.Fatalf("pixel %d = %v, want RGB {200 100 60}", i/4, dec[i:i+3])
		}
	}
}

// TestBC7DDSWriteReadRoundTrip exercises the DDS DX10 BC7 write/read path.
func TestBC7DDSWriteReadRoundTrip(t *testing.T) {
	const w, h = 16, 16
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			img.Pix[i+0] = uint8(x * 16)
			img.Pix[i+1] = uint8(y * 16)
			img.Pix[i+2] = 128
			img.Pix[i+3] = 255
		}
	}

	dds, err := EncodeDDS(img, FormatBC7)
	if err != nil {
		t.Fatalf("EncodeDDS: %v", err)
	}

	var buf bytes.Buffer
	if err := dds.Write(&buf); err != nil {
		t.Fatalf("DDS write: %v", err)
	}

	got, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}
	if got.Format != FormatBC7 || got.Width != w || got.Height != h {
		t.Fatalf("DDS header: fmt=%s %dx%d", got.Format, got.Width, got.Height)
	}
	if !bytes.Equal(got.Faces[0].Mipmaps[0], dds.Faces[0].Mipmaps[0]) {
		t.Fatalf("payload mismatch after DDS round trip")
	}
}
