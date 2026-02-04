package bcn

import (
	"bytes"
	"image/color"
	"testing"
)

func TestDDSUncompressedRGBA(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	ds, err := EncodeDDS(img, FormatRGBA8)
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
	if read.Format != FormatRGBA8 {
		t.Fatalf("expected rgba8")
	}
	decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(decoded.Pix); i += 4 {
		if decoded.Pix[i] != 10 || decoded.Pix[i+1] != 20 || decoded.Pix[i+2] != 30 || decoded.Pix[i+3] != 40 {
			t.Fatalf("pixel mismatch")
		}
	}
}

func TestDDSUncompressedBGRA(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 5, G: 15, B: 25, A: 255})
	ds, err := EncodeDDS(img, FormatBGRA8)
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
	if read.Format != FormatBGRA8 {
		t.Fatalf("expected bgra8")
	}
	decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(decoded.Pix); i += 4 {
		if decoded.Pix[i] != 5 || decoded.Pix[i+1] != 15 || decoded.Pix[i+2] != 25 || decoded.Pix[i+3] != 255 {
			t.Fatalf("pixel mismatch")
		}
	}
}
