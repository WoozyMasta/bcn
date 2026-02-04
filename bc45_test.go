package bcn

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestBC4RoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	data, _, _, err := EncodeImageWithOptions(img, FormatBC4, &EncodeOptions{Quality: QualityBalanced})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeImage(data, 8, 8, FormatBC4)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(decoded.Pix); i += 4 {
		if absDiffU8(decoded.Pix[i], img.Pix[i]) > 6 {
			t.Fatalf("bc4 mismatch")
		}
	}
}

func TestBC5RoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 10, G: 200, B: 30, A: 255})
	data, _, _, err := EncodeImageWithOptions(img, FormatBC5, &EncodeOptions{Quality: QualityBalanced})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeImage(data, 8, 8, FormatBC5)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(decoded.Pix); i += 4 {
		if absDiffU8(decoded.Pix[i], img.Pix[i]) > 6 || absDiffU8(decoded.Pix[i+1], img.Pix[i+1]) > 6 {
			t.Fatalf("bc5 mismatch")
		}
	}
}

func TestDDSBC4BC5(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
	dds, err := EncodeDDSWithOptions([]image.Image{img}, FormatBC4, &EncodeOptions{Quality: QualityFast})
	if err != nil {
		t.Fatalf("encode dds: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := dds.Write(buf); err != nil {
		t.Fatalf("write dds: %v", err)
	}
	read, err := ReadDDS(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read dds: %v", err)
	}
	if read.Format != FormatBC4 {
		t.Fatalf("expected bc4")
	}
}
