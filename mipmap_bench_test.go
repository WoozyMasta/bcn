package bcn

import (
	"fmt"
	"image"
	"testing"
)

var benchSinkMips []*image.NRGBA

func BenchmarkGenerateMipmapsFull1024NRGBA(b *testing.B) {
	img := benchmarkMipNRGBA(1024, 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkMips = GenerateMipmaps(img, false)
	}
}

func BenchmarkGenerateMipmapsLimited1024NRGBA(b *testing.B) {
	img := benchmarkMipNRGBA(1024, 1024)
	for _, maxMipmaps := range []int{1, 4} {
		maxMipmaps := maxMipmaps
		b.Run(fmt.Sprintf("max%d", maxMipmaps), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkMips = GenerateMipmapsN(img, maxMipmaps, false)
			}
		})
	}
}

func BenchmarkGenerateMipmapsOddNRGBA(b *testing.B) {
	img := benchmarkMipNRGBA(1023, 777)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkMips = GenerateMipmaps(img, false)
	}
}

func benchmarkMipNRGBA(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*img.Stride + x*4
			img.Pix[i+0] = uint8((x*13 + y*7 + 11) & 0xFF)
			img.Pix[i+1] = uint8((x*3 + y*11 + 29) & 0xFF)
			img.Pix[i+2] = uint8((x*17 + y*5 + 71) & 0xFF)
			img.Pix[i+3] = uint8((x*5 + y*9 + 101) & 0xFF)
		}
	}
	return img
}
