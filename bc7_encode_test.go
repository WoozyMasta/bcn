package bcn

import (
	"bytes"
	"image"
	"testing"
)

// roundTripPSNR encodes and decodes one buffer and returns its RGB PSNR.
func roundTripPSNR(t *testing.T, rgba []byte, w, h int, format Format) float64 {
	t.Helper()
	enc, err := encodeBlocksWithOptions(rgba, w, h, format, &EncodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("%s encode: %v", format, err)
	}
	dec, err := decodeBlocksWithOptions(enc, w, h, format, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("%s decode: %v", format, err)
	}
	return rgbPSNR(rgba, dec)
}

// TestBC7EncodeQuality verifies mode 6 reaches high fidelity
// on a smooth block (where a single subset is sufficient).
func TestBC7EncodeQuality(t *testing.T) {
	var rgba [64]byte
	for i := range 16 {
		rgba[i*4+0] = uint8(i * 4)
		rgba[i*4+1] = uint8(i * 3)
		rgba[i*4+2] = uint8(100 + i*2)
		rgba[i*4+3] = 255
	}
	if p := roundTripPSNR(t, rgba[:], 4, 4, FormatBC7); p < 45 {
		t.Errorf("smooth block PSNR %.2f dB too low", p)
	}
}

// TestBC7EncodeBeatsDXT5 asserts BC7 is at least as good as DXT5 on the same content
// (BC7 mode 6 has 8-bit endpoints and 4-bit indices vs BC1-style color).
func TestBC7EncodeBeatsDXT5(t *testing.T) {
	const w, h = 64, 64
	for _, scenario := range []string{"opaque", "translucent"} {
		rgba := goldenImage(scenario)
		bc7 := roundTripPSNR(t, rgba, w, h, FormatBC7)
		dxt5 := roundTripPSNR(t, rgba, w, h, FormatDXT5)
		if bc7 < dxt5-0.25 {
			t.Errorf("%s: BC7 PSNR %.2f dB below DXT5 %.2f dB", scenario, bc7, dxt5)
		}
	}
}

func TestBC7EncodeSolid(t *testing.T) {
	const w, h = 8, 8
	rgba := make([]byte, w*h*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i+0], rgba[i+1], rgba[i+2], rgba[i+3] = 200, 100, 60, 255
	}
	enc, err := encodeBlocksWithOptions(rgba, w, h, FormatBC7, &EncodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec, err := decodeBlocksWithOptions(enc, w, h, FormatBC7, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := 0; i < len(dec); i += 4 {
		if dec[i] != 200 || dec[i+1] != 100 || dec[i+2] != 60 {
			t.Fatalf("pixel %d = %v, want RGB {200 100 60}", i/4, dec[i:i+3])
		}
	}
}

// TestBC7DDSWriteReadRoundTrip exercises the DDS DX10 BC7 write/read path.
func TestBC7DDSWriteReadRoundTrip(t *testing.T) {
	const w, h = 16, 16
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			img.Pix[i+0] = uint8(x * 16)
			img.Pix[i+1] = uint8(y * 16)
			img.Pix[i+2] = 128
			img.Pix[i+3] = 255
		}
	}

	dds, err := EncodeDDS(img, FormatBC7)
	if err != nil {
		t.Fatalf("EncodeDDS: %v", err)
	}

	var buf bytes.Buffer
	if err := dds.Write(&buf); err != nil {
		t.Fatalf("DDS write: %v", err)
	}

	got, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}
	if got.Format != FormatBC7 || got.Width != w || got.Height != h {
		t.Fatalf("DDS header: fmt=%s %dx%d", got.Format, got.Width, got.Height)
	}
	if !bytes.Equal(got.Faces[0].Mipmaps[0], dds.Faces[0].Mipmaps[0]) {
		t.Fatalf("payload mismatch after DDS round trip")
	}
}
