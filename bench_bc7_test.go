package bcn

import (
	"fmt"
	"testing"
)

// BenchmarkEncodeBlockBC7 measures single-block BC7 encoding for an opaque
// block (modes 6/1/3/0/2) and an alpha block (modes 6/5/4/7).
func BenchmarkEncodeBlockBC7(b *testing.B) {
	blocks := []struct {
		name  string
		block [16]rgba8
	}{
		{"opaque", benchmarkBlockOpaque()},
		{"alpha", benchmarkBlockAlpha()},
	}
	for _, bc := range blocks {
		b.Run(bc.name, func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				b.Run(qualityName(level), func(b *testing.B) {
					opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: level, AlphaThreshold: 128})
					b.SetBytes(16 * 4)
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						benchSink16 = encodeBlockBC7(bc.block, opts)
					}
				})
			}
		})
	}
}

// BenchmarkEncodeImageBC7 measures full-image BC7 encoding for opaque and
// alpha content across the fast/balanced/best quality levels.
func BenchmarkEncodeImageBC7(b *testing.B) {
	scenarios := []struct {
		name string
		gen  func(width, height int) []byte
	}{
		{"opaque", benchmarkRGBAOpaque},
		{"alpha", benchmarkRGBATranslucent},
	}
	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			for _, size := range benchmarkSizes() {
				rgba := sc.gen(size, size)
				b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
					for _, level := range benchmarkQualityLevels() {
						opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
						b.Run(qualityName(level), func(b *testing.B) {
							b.SetBytes(int64(len(rgba)))
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								out, _ := encodeBlocksWithOptions(rgba, size, size, FormatBC7, opts)
								benchSinkBytes = out
							}
						})
					}
				})
			}
		})
	}
}

// BenchmarkDecodeImageBC7 measures full-image BC7 decoding (alpha content
// exercises the rotation/separate-alpha modes).
func BenchmarkDecodeImageBC7(b *testing.B) {
	opts := &EncodeOptions{QualityLevel: QualityLevelBalanced, AlphaThreshold: 128}
	for _, size := range benchmarkSizes() {
		rgba := benchmarkRGBATranslucent(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatBC7, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				out, err := decodeBlocks(data, size, size, FormatBC7)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}
