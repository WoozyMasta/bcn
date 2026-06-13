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
