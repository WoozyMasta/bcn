// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"fmt"
	"testing"
)

// benchmarkHDRBlock returns a deterministic 4x4 RGB half-float block (48 uint16).
func benchmarkHDRBlock() [48]uint16 {
	var b [48]uint16
	for i := range 16 {
		b[i*3+0] = float32ToFloat16(float32(i*37+11) / 255.0)
		b[i*3+1] = float32ToFloat16(float32(i*17+53) / 255.0)
		b[i*3+2] = float32ToFloat16(float32(i%8) * 1.5) // some HDR values
	}
	return b
}

// benchmarkHDR returns a deterministic HDR image as a flat []uint16 (width*height*3).
func benchmarkHDR(width, height int) []uint16 {
	src := make([]uint16, width*height*3)
	for y := range height {
		for x := range width {
			off := (y*width + x) * 3
			src[off+0] = float32ToFloat16(float32(x*13+y*7+11) / 255.0)
			src[off+1] = float32ToFloat16(float32(x*3+y*11+29) / 255.0)
			src[off+2] = float32ToFloat16(float32((x+y)%8) * 1.5)
		}
	}
	return src
}

// BenchmarkEncodeBlockBC6H measures single 4x4 BC6H block encoding across quality levels.
func BenchmarkEncodeBlockBC6H(b *testing.B) {
	block := benchmarkHDRBlock()
	for _, level := range benchmarkQualityLevels() {
		b.Run(qualityName(level), func(b *testing.B) {
			b.SetBytes(48 * 2) // 16 texels * 3 channels * 2 bytes per half-float
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSink16 = encodeBlockBC6H(block, false, level)
			}
		})
	}
}

// BenchmarkEncodeImageBC6H measures full-image BC6H encoding for unsigned and signed
// variants across fast/balanced/best quality levels and standard image sizes.
func BenchmarkEncodeImageBC6H(b *testing.B) {
	scenarios := []struct {
		name   string
		signed bool
	}{
		{"UF16", false},
		{"SF16", true},
	}
	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			for _, size := range benchmarkSizes() {
				src := benchmarkHDR(size, size)
				b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
					for _, level := range benchmarkQualityLevels() {
						opts := &EncodeOptions{QualityLevel: level}
						b.Run(qualityName(level), func(b *testing.B) {
							b.SetBytes(int64(len(src)) * 2)
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								out, _ := EncodeBC6HWithOptions(src, size, size, sc.signed, opts)
								benchSinkBytes = out
							}
						})
					}
				})
			}
		})
	}
}

// BenchmarkDecodeBlockBC6H measures single 4x4 BC6H block decoding.
func BenchmarkDecodeBlockBC6H(b *testing.B) {
	block := benchmarkHDRBlock()
	encoded := encodeBlockBC6H(block, false, QualityLevelFast)
	b.SetBytes(16) // one compressed block is 16 bytes
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		out := decodeBlockBC6H(encoded[:], false)
		_ = out
	}
}

// BenchmarkDecodeImageBC6H measures full-image BC6H decoding across standard image sizes.
func BenchmarkDecodeImageBC6H(b *testing.B) {
	for _, size := range benchmarkSizes() {
		src := benchmarkHDR(size, size)
		data, err := EncodeBC6HWithOptions(src, size, size, false,
			&EncodeOptions{QualityLevel: QualityLevelFast})
		if err != nil {
			b.Fatalf("encode setup: %v", err)
		}

		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				out, err := DecodeBC6H(data, size, size, false)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				benchSinkBytes = make([]byte, len(out)*2)
			}
		})
	}
}
