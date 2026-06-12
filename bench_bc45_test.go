package bcn

import (
	"fmt"
	"testing"
)

func BenchmarkEncodeImageDXT3(b *testing.B) {
	sizes := benchmarkSizes()
	for _, size := range sizes {
		rgba := benchmarkRGBATranslucent(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
				b.Run(qualityName(level), func(b *testing.B) {
					b.SetBytes(int64(len(rgba)))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						out, _ := encodeBlocksWithOptions(rgba, size, size, FormatDXT3, opts)
						benchSinkBytes = out
					}
				})
			}
		})
	}
}

func BenchmarkEncodeImageBC4(b *testing.B) {
	sizes := benchmarkSizes()
	for _, size := range sizes {
		rgba := benchmarkRGBAOpaque(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
				b.Run(qualityName(level), func(b *testing.B) {
					b.SetBytes(int64(len(rgba)))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						out, _ := encodeBlocksWithOptions(rgba, size, size, FormatBC4, opts)
						benchSinkBytes = out
					}
				})
			}
		})
	}
}

func BenchmarkEncodeImageBC5(b *testing.B) {
	sizes := benchmarkSizes()
	for _, size := range sizes {
		rgba := benchmarkRGBAOpaque(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
				b.Run(qualityName(level), func(b *testing.B) {
					b.SetBytes(int64(len(rgba)))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						out, _ := encodeBlocksWithOptions(rgba, size, size, FormatBC5, opts)
						benchSinkBytes = out
					}
				})
			}
		})
	}
}

func BenchmarkDecodeImageDXT3(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		rgba := benchmarkRGBATranslucent(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatDXT3, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatDXT3)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}

func BenchmarkDecodeImageBC4(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		rgba := benchmarkRGBAOpaque(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatBC4, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatBC4)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}

func BenchmarkDecodeImageBC5(b *testing.B) {
	sizes := benchmarkSizes()
	opts := &EncodeOptions{QualityLevel: QualityLevelFast, AlphaThreshold: 128}
	for _, size := range sizes {
		rgba := benchmarkRGBAOpaque(size, size)
		data, err := encodeBlocksWithOptions(rgba, size, size, FormatBC5, opts)
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := decodeBlocks(data, size, size, FormatBC5)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = out
			}
		})
	}
}
