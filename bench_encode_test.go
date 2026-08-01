package bcn

import (
	"fmt"
	"os"
	"testing"
)

var (
	benchSinkBytes []byte
	benchSink8     [8]byte
	benchSink16    [16]byte
)

func BenchmarkEncodeBlockBC1(b *testing.B) {
	block := benchmarkBlockOpaque()
	for _, level := range benchmarkQualityLevels() {
		level := level
		b.Run(qualityName(level), func(b *testing.B) {
			opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: level, AlphaThreshold: 128})
			b.SetBytes(16 * 4)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSink8 = encodeBlockBC1WithOptions(block, opts)
			}
		})
	}
}

func BenchmarkEncodeBlockBC3(b *testing.B) {
	block := benchmarkBlockAlpha()
	for _, level := range benchmarkQualityLevels() {
		level := level
		b.Run(qualityName(level), func(b *testing.B) {
			opts := normalizeEncodeOptions(&EncodeOptions{QualityLevel: level, AlphaThreshold: 128})
			b.SetBytes(16 * 4)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSink16 = encodeBlockBC3WithOptions(block, opts)
			}
		})
	}
}

func BenchmarkEncodeImageBC1(b *testing.B) {
	sizes := benchmarkSizes()
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBAOpaque(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				level := level
				opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
				b.Run(qualityName(level), func(b *testing.B) {
					b.SetBytes(int64(len(rgba)))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						out, _ := encodeBlocksWithOptions(rgba, size, size, FormatBC1, opts)
						benchSinkBytes = out
					}
				})
			}
		})
	}
}

func BenchmarkEncodeImageBC3(b *testing.B) {
	sizes := benchmarkSizes()
	for _, size := range sizes {
		size := size
		rgba := benchmarkRGBATranslucent(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			for _, level := range benchmarkQualityLevels() {
				level := level
				opts := &EncodeOptions{QualityLevel: level, AlphaThreshold: 128}
				b.Run(qualityName(level), func(b *testing.B) {
					b.SetBytes(int64(len(rgba)))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						out, _ := encodeBlocksWithOptions(rgba, size, size, FormatBC3, opts)
						benchSinkBytes = out
					}
				})
			}
		})
	}
}

func benchmarkQualityLevels() []int {
	return []int{QualityLevelFast, QualityLevelBalanced, QualityLevelBest}
}

func benchmarkSizes() []int {
	sizes := []int{256, 512}
	if os.Getenv("BCN_BENCH_LARGE") != "" {
		sizes = append(sizes, 1024, 2048)
	}
	return sizes
}

func qualityName(level int) string {
	switch level {
	case QualityLevelFast:
		return "fast"
	case QualityLevelBalanced:
		return "balanced"
	case QualityLevelBest:
		return "best"
	default:
		return fmt.Sprintf("q%d", level)
	}
}

func benchmarkBlockOpaque() [16]rgba8 {
	var block [16]rgba8
	for i := 0; i < 16; i++ {
		block[i] = rgba8{
			r: uint8((i*37 + 11) & 0xFF),
			g: uint8((i*17 + 53) & 0xFF),
			b: uint8((i*29 + 97) & 0xFF),
			a: 255,
		}
	}
	return block
}

func benchmarkBlockAlpha() [16]rgba8 {
	var block [16]rgba8
	for i := 0; i < 16; i++ {
		block[i] = rgba8{
			r: uint8((i*23 + 7) & 0xFF),
			g: uint8((i*41 + 19) & 0xFF),
			b: uint8((i*13 + 101) & 0xFF),
			a: uint8((i*31 + 5) & 0xFF),
		}
	}
	return block
}

func benchmarkRGBAOpaque(width, height int) []byte {
	rgba := make([]byte, width*height*4)
	idx := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x*13 + y*7 + 11) & 0xFF)
			g := uint8((x*3 + y*11 + 29) & 0xFF)
			b := uint8((x*17 + y*5 + 71) & 0xFF)
			rgba[idx+0] = r
			rgba[idx+1] = g
			rgba[idx+2] = b
			rgba[idx+3] = 255
			idx += 4
		}
	}
	return rgba
}

func benchmarkRGBATranslucent(width, height int) []byte {
	rgba := make([]byte, width*height*4)
	idx := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x*13 + y*7 + 11) & 0xFF)
			g := uint8((x*3 + y*11 + 29) & 0xFF)
			b := uint8((x*17 + y*5 + 71) & 0xFF)
			a := uint8((x*5 + y*9 + 101) & 0xFF)
			rgba[idx+0] = r
			rgba[idx+1] = g
			rgba[idx+2] = b
			rgba[idx+3] = a
			idx += 4
		}
	}
	return rgba
}
