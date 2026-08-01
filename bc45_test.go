package bcn

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestBC4RoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	data, _, _, err := EncodeImageWithOptions(img, FormatBC4, &EncodeOptions{QualityLevel: QualityLevelBalanced})
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
	data, _, _, err := EncodeImageWithOptions(img, FormatBC5, &EncodeOptions{QualityLevel: QualityLevelBalanced})
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
	dds, err := EncodeDDSWithOptions([]image.Image{img}, FormatBC4, &EncodeOptions{QualityLevel: QualityLevelFast})
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

func TestBC4SDecodeSpecialPalette(t *testing.T) {
	var block [8]byte
	block[0] = 0xC0 // int8(-64)
	block[1] = 0x40 // int8(64)
	var indices uint64
	for i := range 16 {
		indices |= uint64(i%8) << (3 * i)
	}
	putAlphaIndices(block[2:], indices)

	got := decodeSignedAlphaBlock(block[:])
	want := [16]int{-64, 64, -38, -13, 13, 38, -127, 127, -64, 64, -38, -13, 13, 38, -127, 127}
	if got != want {
		t.Fatalf("signed palette = %v, want %v", got, want)
	}

	rgba, err := DecodeBC4S(block[:], 4, 4)
	if err != nil {
		t.Fatalf("DecodeBC4S: %v", err)
	}
	for i, value := range [8]byte{63, 192, 89, 114, 141, 166, 0, 255} {
		pixel := rgba[i*4 : i*4+4]
		if got := [4]byte(pixel); got != [4]byte{value, value, value, 255} {
			t.Fatalf("pixel %d = %v, want {%d %d %d 255}", i, got, value, value, value)
		}
	}
}

func TestBC4SDecodeClampsNegativeOneEndpoint(t *testing.T) {
	block := [8]byte{0x80, 0x7F} // int8(-128), int8(127)
	if got := decodeSignedAlphaBlock(block[:]); got[0] != -127 {
		t.Fatalf("endpoint = %d, want -127", got[0])
	}
}

func TestBC4SBC5SRoundTrip(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = 128
		img.Pix[i+1] = 128
		img.Pix[i+3] = 255
	}

	for _, format := range []Format{FormatBC4S, FormatBC5S} {
		data, _, _, err := EncodeImageWithOptions(img, format, &EncodeOptions{QualityLevel: QualityLevelBalanced})
		if err != nil {
			t.Fatalf("encode %s: %v", format, err)
		}
		decoded, err := DecodeImage(data, 8, 8, format)
		if err != nil {
			t.Fatalf("decode %s: %v", format, err)
		}
		for i := 0; i < len(decoded.Pix); i += 4 {
			if decoded.Pix[i] != 128 || (format == FormatBC5S && decoded.Pix[i+1] != 128) {
				t.Fatalf("%s pixel %d = %v, want normalized zero", format, i/4, decoded.Pix[i:i+4])
			}
		}
	}
}

func TestDDSBC4SBC5S(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 128, G: 128, A: 255})
	for _, format := range []Format{FormatBC4S, FormatBC5S} {
		dds, err := EncodeDDS(img, format)
		if err != nil {
			t.Fatalf("encode %s: %v", format, err)
		}
		var buf bytes.Buffer
		if err := dds.Write(&buf); err != nil {
			t.Fatalf("write %s: %v", format, err)
		}
		read, err := ReadDDS(&buf)
		if err != nil {
			t.Fatalf("read %s: %v", format, err)
		}
		if read.Format != format {
			t.Fatalf("format = %s, want %s", read.Format, format)
		}
	}
}
