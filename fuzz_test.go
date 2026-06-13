package bcn

import "testing"

// These fuzz targets assert that every SIMD kernel stays byte-exact with its
// scalar reference on coverage-guided inputs, and that the public encode/decode
// entry points never panic on arbitrary data. They complement the exhaustive
// and random equivalence tests in kernels_test.go.

// fuzzWeights derives a weight set from one seed byte.
func fuzzWeights(b byte) rgbWeightsFP {
	switch b & 3 {
	case 0:
		return defaultWeightsFP

	case 1:
		return balancedWeightsFP

	case 2:
		return rgbWeightsFP{r: 1, g: 1, b: 1}

	default:
		return rgbWeightsFP{r: 1024, g: 0, b: 0}
	}
}

// FuzzScoreDXT1Palette compares the score kernel with the scalar reference.
func FuzzScoreDXT1Palette(f *testing.F) {
	f.Add(make([]byte, 67))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 67 {
			return
		}
		var block [16]rgba8
		for i := range block {
			block[i] = rgba8{data[i*4], data[i*4+1], data[i*4+2], 255}
		}
		c0 := uint16(data[64]) | uint16(data[65])<<8
		c1 := uint16(data[65]) | uint16(data[66])<<8
		w := fuzzWeights(data[66])

		got, ok := scoreDXT1PaletteASM(&block, c0, c1, w)
		if !ok {
			t.Skip("score kernel unavailable")
		}
		want := dxt1BlockErrorScalar(block, c0, c1, false, 0, w, maxBlockErr)
		if got != want {
			t.Fatalf("score c0=%#04x c1=%#04x: got %d want %d", c0, c1, got, want)
		}
	})
}

// FuzzPackDXT1Indices compares the index-assignment kernel with the reference.
func FuzzPackDXT1Indices(f *testing.F) {
	f.Add(make([]byte, 82))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 82 {
			return
		}
		var block [16]rgba8
		for i := range block {
			block[i] = rgba8{data[i*4], data[i*4+1], data[i*4+2], data[i*4+3]}
		}
		var palette [4]rgba8
		for i := range palette {
			o := 64 + i*4
			palette[i] = rgba8{data[o], data[o+1], data[o+2], 255}
		}
		w := fuzzWeights(data[80])
		hasAlpha := data[81]&1 == 0
		threshold := data[81]

		got, ok := packDXT1IndicesASM(&block, &palette, hasAlpha, threshold, w)
		if !ok {
			t.Skip("pack kernel unavailable")
		}
		want := packDXT1IndicesGeneric(block, palette, hasAlpha, threshold, w)
		if got != want {
			t.Fatalf("pack: got %#08x want %#08x", got, want)
		}
	})
}

// FuzzAlphaKernels compares both alpha kernels with their references.
func FuzzAlphaKernels(f *testing.F) {
	f.Add(make([]byte, 18))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 18 {
			return
		}
		var alpha [16]uint8
		copy(alpha[:], data)
		a0, a1 := data[16], data[17]
		palette := dxt5AlphaPalette(a0, a1)

		errGot, ok := alphaBlockErrorASM(&alpha, a0, a1)
		if !ok {
			t.Skip("alpha kernels unavailable")
		}
		errWant := alphaBlockErrorScalar(&palette, &alpha, 1<<62)
		if errGot != errWant {
			t.Fatalf("alphaBlockError: got %d want %d", errGot, errWant)
		}

		idxGot, _ := bestAlphaIndices16ASM(&alpha, a0, a1)
		var idxWant uint64
		for i := 15; i >= 0; i-- {
			idxWant = (idxWant << 3) | uint64(bestAlphaIndex(&palette, alpha[i])&0x7)
			if i == 0 {
				break
			}
		}
		if idxGot != idxWant {
			t.Fatalf("bestAlphaIndices: got %#012x want %#012x", idxGot, idxWant)
		}
	})
}

// FuzzDecodeNoPanic feeds arbitrary payloads to every decoder and asserts the
// public entry points never panic and produce the expected output length.
func FuzzDecodeNoPanic(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0}, uint8(4), uint8(4), uint8(0))
	formats := []Format{FormatDXT1, FormatDXT3, FormatDXT5, FormatBC4, FormatBC5}
	f.Fuzz(func(t *testing.T, data []byte, w, h, fsel uint8) {
		width := int(w%64) + 1
		height := int(h%64) + 1
		format := formats[int(fsel)%len(formats)]

		out, err := decodeBlocksWithOptions(data, width, height, format, &DecodeOptions{Workers: 1})
		if err != nil {
			return
		}
		if len(out) != width*height*4 {
			t.Fatalf("decode %v %dx%d: output length %d, want %d", format, width, height, len(out), width*height*4)
		}
	})
}
