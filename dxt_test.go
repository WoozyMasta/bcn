package bcn

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestDXT1SolidBlock(t *testing.T) {
	c := color.NRGBA{R: 248, G: 252, B: 248, A: 255}
	img := SolidImage(4, 4, c)
	data, _, _, err := EncodeImage(img, FormatDXT1)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeImage(data, 4, 4, FormatDXT1)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := rgba8{c.R, c.G, c.B, c.A}
	for i := 0; i < len(decoded.Pix); i += 4 {
		got := rgba8{decoded.Pix[i], decoded.Pix[i+1], decoded.Pix[i+2], decoded.Pix[i+3]}
		if absDiffU8(got.r, want.r) > 12 || absDiffU8(got.g, want.g) > 12 || absDiffU8(got.b, want.b) > 12 || got.a != want.a {
			t.Fatalf("pixel mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestDXT3AlphaQuantization(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 100, G: 150, B: 200, A: 0})
	for i := 0; i < 16; i++ {
		img.Pix[i*4+3] = uint8(i * 17)
	}
	data, _, _, err := EncodeImage(img, FormatDXT3)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeImage(data, 4, 4, FormatDXT3)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < 16; i++ {
		orig := img.Pix[i*4+3]
		want := uint8((int(orig)+8)/17) * 17
		got := decoded.Pix[i*4+3]
		if got != want {
			t.Fatalf("alpha mismatch at %d: got=%d want=%d", i, got, want)
		}
	}
}

func TestDXT5AlphaConstant(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 200})
	data, _, _, err := EncodeImage(img, FormatDXT5)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeImage(data, 4, 4, FormatDXT5)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < 16; i++ {
		got := decoded.Pix[i*4+3]
		if got != 200 {
			t.Fatalf("alpha mismatch: got=%d want=200", got)
		}
	}
}

func TestDDSRoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 180, G: 60, B: 220, A: 255})
	ds, err := EncodeDDS(img, FormatDXT1)
	if err != nil {
		t.Fatalf("encode dds: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := ds.Write(buf); err != nil {
		t.Fatalf("write dds: %v", err)
	}
	read, err := ReadDDS(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read dds: %v", err)
	}
	if read.Width != 8 || read.Height != 8 || read.Format != FormatDXT1 {
		t.Fatalf("dds header mismatch")
	}
	if len(read.Faces) != 1 || len(read.Faces[0].Mipmaps) != 1 {
		t.Fatalf("expected 1 mipmap")
	}
	if len(read.Faces[0].Mipmaps[0]) != len(ds.Faces[0].Mipmaps[0]) {
		t.Fatalf("mipmap size mismatch")
	}
}

func TestDDSCubemapMipmaps(t *testing.T) {
	images := []image.Image{
		SolidImage(8, 8, color.NRGBA{R: 255, G: 0, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 255, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 0, B: 255, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 255, G: 255, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 255, G: 0, B: 255, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 255, B: 255, A: 255}),
	}
	opts := &EncodeOptions{GenerateMipmaps: true, Quality: QualityFast}
	ds, err := EncodeDDSWithOptions(images, FormatDXT1, opts)
	if err != nil {
		t.Fatalf("encode cubemap: %v", err)
	}
	if len(ds.Faces) != 6 {
		t.Fatalf("expected 6 faces")
	}
	if len(ds.Faces[0].Mipmaps) < 2 {
		t.Fatalf("expected mipmaps")
	}
	buf := &bytes.Buffer{}
	if err := ds.Write(buf); err != nil {
		t.Fatalf("write dds: %v", err)
	}
	read, err := ReadDDS(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read dds: %v", err)
	}
	if !read.IsCubemap() {
		t.Fatalf("expected cubemap")
	}
	if len(read.Faces[0].Mipmaps) != len(ds.Faces[0].Mipmaps) {
		t.Fatalf("mipmap count mismatch")
	}
}

func absDiffU8(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
