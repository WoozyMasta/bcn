package bcn

import "testing"

// kernelTestBlocks yields edge-case and pseudo-random blocks for equivalence tests.
func kernelTestBlocks() [][16]rgba8 {
	var blocks [][16]rgba8

	var b [16]rgba8
	blocks = append(blocks, b) // all zero

	for i := range b {
		b[i] = rgba8{255, 255, 255, 255}
	}
	blocks = append(blocks, b) // all max

	for i := range b {
		v := uint8(i * 17)
		b[i] = rgba8{v, 255 - v, v / 2, v | 0x80}
	}
	blocks = append(blocks, b) // gradient

	for k := range 16 {
		b = [16]rgba8{}
		b[k] = rgba8{255, 1, 254, 2}
		blocks = append(blocks, b) // single-pixel extremes at every position
	}

	state := uint32(0xC0FFEE01)
	next := func() uint8 {
		state = state*1664525 + 1013904223
		return uint8(state >> 24)
	}
	for range 100_000 {
		for i := range b {
			b[i] = rgba8{next(), next(), next(), next()}
		}
		blocks = append(blocks, b)
	}

	return blocks
}

// TestFindMinMaxEquivalence verifies the dispatched kernel against the
// pure-Go reference byte-exactly over edge and random blocks.
func TestFindMinMaxEquivalence(t *testing.T) {
	for _, b := range kernelTestBlocks() {
		gotMin, gotMax := findMinMax(b)
		wantMin, wantMax := findMinMaxGeneric(b)
		if gotMin != wantMin || gotMax != wantMax {
			t.Fatalf("findMinMax(%v) = %v, %v; want %v, %v", b, gotMin, gotMax, wantMin, wantMax)
		}
	}
}

// decodePayloadBC1 builds a deterministic payload covering endpoint orderings,
// per-channel endpoint sweeps and pseudo-random blocks.
func decodePayloadBC1(blocks int) []byte {
	data := make([]byte, blocks*8)
	state := uint32(0xDEC0DE01)
	next := func() byte {
		state = state*1664525 + 1013904223
		return byte(state >> 24)
	}

	for b := 0; b < blocks; b++ {
		off := b * 8
		switch {
		case b < 1024: // per-channel endpoint sweeps (r/g pairs), both modes
			c0 := uint16(b%32) << 11
			c1 := uint16(b/32) << 11
			if b%2 == 0 {
				c0 |= uint16(b) & 0x7E0
				c1 |= uint16(b/2) & 0x7E0
			}
			data[off] = byte(c0)
			data[off+1] = byte(c0 >> 8)
			data[off+2] = byte(c1)
			data[off+3] = byte(c1 >> 8)
			for i := 4; i < 8; i++ {
				data[off+i] = next()
			}
		default: // random endpoints and indices
			for i := range 8 {
				data[off+i] = next()
			}
		}
	}

	return data
}

// TestDecodeRangeBC1Equivalence verifies the AVX2 row kernel against the
// scalar block decoder byte-exactly, including partial start/end ranges.
func TestDecodeRangeBC1Equivalence(t *testing.T) {
	const width, height = 64, 64 // 16x16 blocks, all interior
	bx := width / 4
	blocks := bx * (height / 4)
	data := decodePayloadBC1(blocks)

	want := make([]byte, width*height*4)
	for idx := range blocks {
		block := decodeBlockBC1(data[idx*8 : idx*8+8])
		storeBlock(want, width, height, idx%bx, idx/bx, &block)
	}

	ranges := [][2]int{{0, blocks}, {3, blocks - 5}, {bx - 1, bx + 1}, {7, 9}}
	for _, r := range ranges {
		got := make([]byte, width*height*4)
		if !decodeRangeBC1ASM(data, got, width, height, bx, r[0], r[1]) {
			t.Skip("ASM decode kernel unavailable on this platform")
		}

		for idx := r[0]; idx < r[1]; idx++ {
			x, y := idx%bx, idx/bx
			for row := range 4 {
				off := ((y*4+row)*width + x*4) * 4
				for i := range 16 {
					if got[off+i] != want[off+i] {
						t.Fatalf("range %v block %d row %d byte %d: got %d, want %d",
							r, idx, row, i, got[off+i], want[off+i])
					}
				}
			}
		}
	}
}

// checkDecodeRangeASM runs an ASM range decoder against the scalar reference
// over a 64x64 image built from payload and fails on any byte difference.
func checkDecodeRangeASM(t *testing.T, name string, payload []byte, blockSize int,
	asmFn func(data, out []byte, width, height, bx, start, end int) bool,
	scalarFn func(data []byte) [64]byte,
) {
	t.Helper()
	const width, height = 64, 64
	bx := width / 4
	blocks := bx * (height / 4)

	want := make([]byte, width*height*4)
	for idx := range blocks {
		block := scalarFn(payload[idx*blockSize : (idx+1)*blockSize])
		storeBlock(want, width, height, idx%bx, idx/bx, &block)
	}

	got := make([]byte, width*height*4)
	if !asmFn(payload, got, width, height, bx, 0, blocks) {
		t.Skipf("%s: ASM kernel unavailable on this platform", name)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: byte %d (block %d): got %d, want %d", name, i, i/64, got[i], want[i])
		}
	}
}

// alphaPayload fills 8-byte alpha blocks: full sweep of (a0, a1) endpoint
// pairs across calls plus pseudo-random index bits.
func alphaPayload(blocks, seed int) []byte {
	data := make([]byte, blocks*8)
	state := uint32(seed)
	next := func() byte {
		state = state*1664525 + 1013904223
		return byte(state >> 24)
	}
	for b := range blocks {
		off := b * 8
		pair := seed*blocks + b
		data[off] = byte(pair >> 8)
		data[off+1] = byte(pair)
		for i := 2; i < 8; i++ {
			data[off+i] = next()
		}
	}
	return data
}

// TestDecodeRangeBC2Equivalence sweeps explicit 4-bit alpha and color blocks
// (both palette modes) through the BC2 kernel.
func TestDecodeRangeBC2Equivalence(t *testing.T) {
	const blocks = 256
	state := uint32(0xD37C0DE3)
	next := func() byte {
		state = state*1664525 + 1013904223
		return byte(state >> 24)
	}
	for round := range 256 {
		color := decodePayloadBC1(blocks)
		payload := make([]byte, blocks*16)
		for b := range blocks {
			// 8 bytes explicit alpha (all 16 nibble values covered across rounds).
			for i := range 8 {
				payload[b*16+i] = byte(round) + next()
			}
			// Reuse BC1 color sub-block (c0,c1,indices) at offset 8.
			copy(payload[b*16+8:b*16+16], color[b*8:(b+1)*8])
		}
		checkDecodeRangeASM(t, "BC2", payload, 16, decodeRangeBC2ASM, decodeBlockBC2)
	}
}

// TestDecodeRangeBC3Equivalence sweeps all 65536 alpha endpoint pairs plus
// random color blocks through the BC3 kernel.
func TestDecodeRangeBC3Equivalence(t *testing.T) {
	const blocks = 256
	for round := range 256 {
		alpha := alphaPayload(blocks, round)
		color := decodePayloadBC1(blocks)
		payload := make([]byte, blocks*16)
		for b := range blocks {
			copy(payload[b*16:b*16+8], alpha[b*8:(b+1)*8])
			copy(payload[b*16+8:b*16+16], color[b*8:(b+1)*8])
		}
		checkDecodeRangeASM(t, "BC3", payload, 16, decodeRangeBC3ASM, decodeBlockBC3)
	}
}

// TestDecodeRangeBC4Equivalence sweeps all alpha endpoint pairs through BC4.
func TestDecodeRangeBC4Equivalence(t *testing.T) {
	const blocks = 256
	for round := range 256 {
		payload := alphaPayload(blocks, round)
		checkDecodeRangeASM(t, "BC4", payload, 8, decodeRangeBC4ASM, func(data []byte) [64]byte {
			alpha := decodeBlockBC4(data)
			var block [64]byte
			expandBC4Block(&block, &alpha)
			return block
		})
	}
}

// TestDecodeRangeBC5Equivalence covers endpoint sweeps in both channels.
func TestDecodeRangeBC5Equivalence(t *testing.T) {
	const blocks = 256
	for round := range 128 {
		r := alphaPayload(blocks, round)
		g := alphaPayload(blocks, 255-round)
		payload := make([]byte, blocks*16)
		for b := range blocks {
			copy(payload[b*16:b*16+8], r[b*8:(b+1)*8])
			copy(payload[b*16+8:b*16+16], g[b*8:(b+1)*8])
		}
		checkDecodeRangeASM(t, "BC5", payload, 16, decodeRangeBC5ASM, decodeBlockBC5)
	}
}

// TestPackBC1IndicesEquivalence verifies the AVX2 index-assignment kernel
// against the scalar reference across opaque and alpha modes, random palettes,
// weights and blocks (including sub-threshold alpha pixels and ties).
func TestPackBC1IndicesEquivalence(t *testing.T) {
	state := uint32(0x9E3779B1)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}
	nextByte := func() uint8 { return uint8(next() >> 24) }

	weights := []rgbWeightsFP{
		defaultWeightsFP,
		balancedWeightsFP,
		{r: 1, g: 1, b: 1},
		{r: 1024, g: 0, b: 0},
	}

	for iter := 0; iter < 200_000; iter++ {
		var block [16]rgba8
		for i := range block {
			block[i] = rgba8{nextByte(), nextByte(), nextByte(), nextByte()}
		}

		var palette [4]rgba8
		for i := range palette {
			palette[i] = rgba8{nextByte(), nextByte(), nextByte(), 255}
		}

		w := weights[next()%uint32(len(weights))]
		hasAlpha := next()&1 == 0
		threshold := uint8(next() % 256)

		got, ok := packBC1IndicesASM(&block, &palette, hasAlpha, threshold, w)
		if !ok {
			t.Skip("pack kernel unavailable on this platform")
		}
		want := packBC1IndicesGeneric(block, palette, hasAlpha, threshold, w)
		if got != want {
			t.Fatalf("iter %d hasAlpha=%v thr=%d w=%v: got %#08x, want %#08x\nblock=%v\npalette=%v",
				iter, hasAlpha, threshold, w, got, want, block, palette)
		}
	}
}

// TestScoreBC1PaletteEquivalence verifies the AVX2 block-error kernel against
// the scalar opaque-mode reference over random blocks, endpoint pairs (both
// 3-color and 4-color palette modes) and weights.
func TestScoreBC1PaletteEquivalence(t *testing.T) {
	state := uint32(0x5DEECE66)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}
	nextByte := func() uint8 { return uint8(next() >> 24) }

	weights := []rgbWeightsFP{
		defaultWeightsFP,
		balancedWeightsFP,
		{r: 1, g: 1, b: 1},
		{r: 1024, g: 0, b: 0},
		{r: 0, g: 0, b: 1024},
	}

	for iter := 0; iter < 300_000; iter++ {
		var block [16]rgba8
		for i := range block {
			block[i] = rgba8{nextByte(), nextByte(), nextByte(), 255}
		}

		c0 := uint16(next())
		c1 := uint16(next())
		// Half the cases force c0 == c1 (3-color palette with black entry 3).
		if next()&3 == 0 {
			c1 = c0
		}
		w := weights[next()%uint32(len(weights))]

		got, ok := scoreBC1PaletteASM(&block, c0, c1, w)
		if !ok {
			t.Skip("score kernel unavailable on this platform")
		}
		want := bc1BlockErrorScalar(block, c0, c1, false, 0, w, maxBlockErr)
		if got != want {
			t.Fatalf("iter %d c0=%#04x c1=%#04x w=%v: got %d, want %d\nblock=%v",
				iter, c0, c1, w, got, want, block)
		}
	}
}

// TestAlphaBlockErrorEquivalence verifies the AVX2 alpha-scoring kernel against
// the scalar reference over random samples and all-pair endpoint palettes,
// including degenerate (a0==a1) and swapped (mode5/mode7) palettes.
func TestAlphaBlockErrorEquivalence(t *testing.T) {
	state := uint32(0xA1FA0001)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}

	for iter := 0; iter < 300_000; iter++ {
		var alpha [16]uint8
		for i := range alpha {
			alpha[i] = uint8(next() >> 24)
		}
		a0 := uint8(next() >> 24)
		a1 := uint8(next() >> 24)
		palette := bc3AlphaPalette(a0, a1)

		got, ok := alphaBlockErrorASM(&alpha, a0, a1)
		if !ok {
			t.Skip("alpha score kernel unavailable on this platform")
		}
		want := alphaBlockErrorScalar(&palette, &alpha, 1<<62)
		if got != want {
			t.Fatalf("iter %d a0=%d a1=%d: got %d, want %d\nalpha=%v\npalette=%v",
				iter, a0, a1, got, want, alpha, palette)
		}
	}
}

// TestBestAlphaIndicesEquivalence verifies the AVX2 alpha index-packing kernel
// against the scalar reference, including ties (lowest index wins).
func TestBestAlphaIndicesEquivalence(t *testing.T) {
	state := uint32(0xB2FB0002)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}

	scalarPack := func(palette *[8]uint8, alpha *[16]uint8) uint64 {
		var idx uint64
		for i := 15; i >= 0; i-- {
			best := bestAlphaIndex(palette, alpha[i])
			idx = (idx << 3) | uint64(best&0x7)
			if i == 0 {
				break
			}
		}
		return idx
	}

	for iter := 0; iter < 300_000; iter++ {
		var alpha [16]uint8
		for i := range alpha {
			alpha[i] = uint8(next() >> 24)
		}
		// Mix in palettes with repeated values to exercise tie-breaks.
		a0 := uint8(next() >> 24)
		a1 := a0
		if next()&3 != 0 {
			a1 = uint8(next() >> 24)
		}
		palette := bc3AlphaPalette(a0, a1)

		got, ok := bestAlphaIndices16ASM(&alpha, a0, a1)
		if !ok {
			t.Skip("alpha index kernel unavailable on this platform")
		}
		want := scalarPack(&palette, &alpha)
		if got != want {
			t.Fatalf("iter %d a0=%d a1=%d: got %#012x, want %#012x\nalpha=%v\npalette=%v",
				iter, a0, a1, got, want, alpha, palette)
		}
	}
}

// TestLSQColorAccumulateEquivalence verifies the LSQ assign+accumulate hook
// against the scalar reference for opaque and alpha-mode BC1 palettes.
func TestLSQColorAccumulateEquivalence(t *testing.T) {
	state := uint32(0x15A11C01)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}
	nextByte := func() uint8 { return uint8(next() >> 24) }

	weights := []rgbWeightsFP{
		defaultWeightsFP,
		balancedWeightsFP,
		{r: 1, g: 1, b: 1},
		{r: 0, g: 1024, b: 0},
	}

	for iter := 0; iter < 200_000; iter++ {
		var block [16]rgba8
		for i := range block {
			block[i] = rgba8{nextByte(), nextByte(), nextByte(), nextByte()}
		}

		c0, c1 := uint16(next()), uint16(next())
		palette := bc1Palette(c0, c1)
		hasAlpha := next()&1 == 0
		threshold := uint8(next() >> 24)
		w := weights[next()%uint32(len(weights))]
		d := 3
		betaNum := &colorBetaNum4
		limit := 4
		if hasAlpha {
			d = 2
			betaNum = &colorBetaNum3
			limit = 3
		}

		got, ok := lsqColorAccumulateASM(&block, &palette, hasAlpha, threshold, w, d, betaNum)
		if !ok {
			t.Skip("LSQ color accumulate kernel unavailable on this platform")
		}
		want := lsqColorAccumulateGeneric(block, &palette, hasAlpha, threshold, w, d, betaNum, limit)
		if got != want {
			t.Fatalf("iter %d hasAlpha=%v thr=%d w=%v: got %+v, want %+v\nblock=%v\npalette=%v",
				iter, hasAlpha, threshold, w, got, want, block, palette)
		}
	}
}

// TestLSQAlphaAccumulateEquivalence verifies the LSQ alpha assign+accumulate hook
// against the scalar reference over random samples and endpoint pairs.
func TestLSQAlphaAccumulateEquivalence(t *testing.T) {
	state := uint32(0x15A11C02)
	next := func() uint32 {
		state = state*1664525 + 1013904223
		return state
	}

	for iter := 0; iter < 200_000; iter++ {
		var alpha [16]uint8
		for i := range alpha {
			alpha[i] = uint8(next() >> 24)
		}
		a0 := uint8(next() >> 24)
		a1 := uint8(next() >> 24)
		palette := bc3AlphaPalette(a0, a1)

		got, ok := lsqAlphaAccumulateASM(&alpha, a0, a1)
		if !ok {
			t.Skip("LSQ alpha accumulate kernel unavailable on this platform")
		}
		want := lsqAlphaAccumulateGeneric(alpha, &palette)
		if got != want {
			t.Fatalf("iter %d a0=%d a1=%d: got %+v, want %+v\nalpha=%v\npalette=%v",
				iter, a0, a1, got, want, alpha, palette)
		}
	}
}

func BenchmarkFindMinMax(b *testing.B) {
	block := benchmarkBlockAlpha()
	b.Run("dispatch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchSinkMin, benchSinkMax = findMinMax(block)
		}
	})
	b.Run("generic", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchSinkMin, benchSinkMax = findMinMaxGeneric(block)
		}
	})
}

var benchSinkMin, benchSinkMax rgba8
