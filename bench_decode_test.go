package bcn

import (
	"fmt"
	"testing"
)

var benchSinkBlock [16]rgba8

func BenchmarkDecodeBlockDXT1(b *testing.B) {
	src := benchmarkBlockOpaque()
	opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128})
	encoded := encodeBlockDXT1WithOptions(src, opts)
	data := encoded[:]

	b.SetBytes(8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkBlock = decodeBlockDXT1(data)
	}
}

func BenchmarkDecodeBlockDXT5(b *testing.B) {
	src := benchmarkBlockAlpha()
	opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128})
	encoded := encodeBlockDXT5WithOptions(src, opts)
	data := encoded[:]

	b.SetBytes(16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkBlock = decodeBlockDXT5(data)
	}
}

func BenchmarkDecodeImageDXT1(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBAOpaque(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatDXT1, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatDXT1)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}

func BenchmarkDecodeImageDXT5(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBATranslucent(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatDXT5, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatDXT5)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}
