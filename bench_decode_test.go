package bcn

import (
	"fmt"
	"testing"
)

var benchSinkBlock [64]byte

func BenchmarkDecodeBlockBC1(b *testing.B) {
	src := benchmarkBlockOpaque()
	opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128})
	encoded := encodeBlockBC1WithOptions(src, opts)
	data := encoded[:]

	b.SetBytes(8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkBlock = decodeBlockBC1(data)
	}
}

func BenchmarkDecodeBlockBC3(b *testing.B) {
	src := benchmarkBlockAlpha()
	opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128})
	encoded := encodeBlockBC3WithOptions(src, opts)
	data := encoded[:]

	b.SetBytes(16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkBlock = decodeBlockBC3(data)
	}
}

func BenchmarkDecodeImageBC1(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBAOpaque(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatBC1, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatBC1)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}

func BenchmarkDecodeImageBC3(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBATranslucent(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatBC3, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatBC3)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}
