package bcn

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestGenerateMipmapsNLimit(t *testing.T) {
	img := testMipImage(8, 4)

	full := GenerateMipmapsN(img, 0, false)
	if len(full) != 4 {
		t.Fatalf("full mip count: got %d want 4", len(full))
	}
	assertMipSize(t, full[0], 8, 4)
	assertMipSize(t, full[1], 4, 2)
	assertMipSize(t, full[2], 2, 1)
	assertMipSize(t, full[3], 1, 1)

	one := GenerateMipmapsN(img, 1, false)
	if len(one) != 1 {
		t.Fatalf("max 1 mip count: got %d want 1", len(one))
	}
	assertMipSize(t, one[0], 8, 4)

	limited := GenerateMipmapsN(img, 3, false)
	if len(limited) != 3 {
		t.Fatalf("limited mip count: got %d want 3", len(limited))
	}
	assertMipSize(t, limited[2], 2, 1)

	overLimit := GenerateMipmapsN(img, 99, false)
	if len(overLimit) != len(full) {
		t.Fatalf("over-limit mip count: got %d want %d", len(overLimit), len(full))
	}
}

func TestGenerateMipmapsWrapperCompatibility(t *testing.T) {
	img := testMipImage(7, 5)
	old := GenerateMipmaps(img, true)
	next := GenerateMipmapsN(img, 0, true)

	if len(old) != len(next) {
		t.Fatalf("mip count mismatch: got %d want %d", len(old), len(next))
	}
	for i := range old {
		if !old[i].Rect.Eq(next[i].Rect) {
			t.Fatalf("mip %d rect mismatch: got %v want %v", i, old[i].Rect, next[i].Rect)
		}
		if !bytes.Equal(old[i].Pix, next[i].Pix) {
			t.Fatalf("mip %d pixels mismatch", i)
		}
	}
}

func TestGenerateMipmapsNOddAndStrips(t *testing.T) {
	for _, tc := range []struct {
		name  string
		w, h  int
		sizes []image.Point
	}{
		{
			name:  "one",
			w:     1,
			h:     1,
			sizes: []image.Point{{X: 1, Y: 1}},
		},
		{
			name:  "wide-strip",
			w:     5,
			h:     1,
			sizes: []image.Point{{X: 5, Y: 1}, {X: 2, Y: 1}, {X: 1, Y: 1}},
		},
		{
			name:  "tall-strip",
			w:     1,
			h:     5,
			sizes: []image.Point{{X: 1, Y: 5}, {X: 1, Y: 2}, {X: 1, Y: 1}},
		},
		{
			name:  "odd",
			w:     7,
			h:     5,
			sizes: []image.Point{{X: 7, Y: 5}, {X: 3, Y: 2}, {X: 1, Y: 1}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mips := GenerateMipmapsN(testMipImage(tc.w, tc.h), 0, false)
			if len(mips) != len(tc.sizes) {
				t.Fatalf("mip count: got %d want %d", len(mips), len(tc.sizes))
			}
			for i, size := range tc.sizes {
				assertMipSize(t, mips[i], size.X, size.Y)
			}
		})
	}
}

func TestGenerateMipmapsNOddWidthUsesExistingFloorChain(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	img.SetNRGBA(1, 0, color.NRGBA{R: 30, G: 40, B: 50, A: 60})
	img.SetNRGBA(2, 0, color.NRGBA{R: 100, G: 120, B: 140, A: 160})

	mips := GenerateMipmapsN(img, 2, false)
	if len(mips) != 2 {
		t.Fatalf("mip count: got %d want 2", len(mips))
	}

	got := mips[1].NRGBAAt(0, 0)
	want := color.NRGBA{R: 20, G: 30, B: 40, A: 50}
	if got != want {
		t.Fatalf("first downscaled pixel: got %+v want %+v", got, want)
	}
}

func TestGenerateMipmapsNMatchesScalarDownscale(t *testing.T) {
	img := testMipImage(10, 6)
	mips := GenerateMipmapsN(img, 2, false)
	if len(mips) != 2 {
		t.Fatalf("mip count: got %d want 2", len(mips))
	}

	want := scalarDownscaleNRGBAForTest(img, 5, 3)
	if !bytes.Equal(mips[1].Pix, want.Pix) {
		t.Fatalf("downscaled pixels mismatch")
	}
}

func assertMipSize(t *testing.T, img *image.NRGBA, w, h int) {
	t.Helper()
	if img.Rect.Dx() != w || img.Rect.Dy() != h {
		t.Fatalf("mip size: got %dx%d want %dx%d", img.Rect.Dx(), img.Rect.Dy(), w, h)
	}
}

func testMipImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*img.Stride + x*4
			img.Pix[i+0] = uint8((x*17 + y*3 + 5) & 0xFF)
			img.Pix[i+1] = uint8((x*7 + y*13 + 11) & 0xFF)
			img.Pix[i+2] = uint8((x*5 + y*19 + 23) & 0xFF)
			img.Pix[i+3] = uint8((x*11 + y*29 + 31) & 0xFF)
		}
	}
	return img
}

func scalarDownscaleNRGBAForTest(src *image.NRGBA, dstW, dstH int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	srcW := src.Rect.Dx()
	srcH := src.Rect.Dy()
	for y := 0; y < dstH; y++ {
		sy0 := y * 2
		sy1 := sy0 + 1
		if sy1 >= srcH {
			sy1 = srcH - 1
		}
		for x := 0; x < dstW; x++ {
			sx0 := x * 2
			sx1 := sx0 + 1
			if sx1 >= srcW {
				sx1 = srcW - 1
			}
			i00 := src.PixOffset(sx0, sy0)
			i10 := src.PixOffset(sx1, sy0)
			i01 := src.PixOffset(sx0, sy1)
			i11 := src.PixOffset(sx1, sy1)
			o := y*dst.Stride + x*4
			dst.Pix[o+0] = avg4U8(src.Pix[i00+0], src.Pix[i10+0], src.Pix[i01+0], src.Pix[i11+0])
			dst.Pix[o+1] = avg4U8(src.Pix[i00+1], src.Pix[i10+1], src.Pix[i01+1], src.Pix[i11+1])
			dst.Pix[o+2] = avg4U8(src.Pix[i00+2], src.Pix[i10+2], src.Pix[i01+2], src.Pix[i11+2])
			dst.Pix[o+3] = avg4U8(src.Pix[i00+3], src.Pix[i10+3], src.Pix[i01+3], src.Pix[i11+3])
		}
	}
	return dst
}
