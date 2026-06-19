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

// bc7DecodeSSE decodes a packed block and returns its RGBA error against the source,
// used to confirm an encoder's reported error matches the decoder.
func bc7DecodeSSE(block [16]rgba8, enc [16]byte) int {
	dec := decodeBlockBC7(enc[:])
	total := 0
	for i := range 16 {
		o := i * 4
		total += bc7SSE(block[i], rgba8{dec[o], dec[o+1], dec[o+2], dec[o+3]})
	}
	return total
}

// TestBC7EncoderSelfConsistent checks that each mode encoder's
// reported error equals the error of its own packed block after decoding.
// A mismatch means the packing or quantization disagrees with the decoder.
func TestBC7EncoderSelfConsistent(t *testing.T) {
	var grad, region, alpha [16]rgba8
	for i := range 16 {
		grad[i] = rgba8{uint8(i * 16), uint8(i * 8), uint8(100 + i*4), 255}
		region[i] = rgba8{50, 50, 200, 255}
		if i%4 < 2 {
			region[i] = rgba8{200, 50, 50, 255}
		}
		alpha[i] = rgba8{uint8(i * 12), 80, uint8(200 - i*10), uint8(i * 16)}
	}
	blocks := map[string][16]rgba8{"gradient": grad, "region": region, "alpha": alpha}

	for name, blk := range blocks {
		if b, e := encodeBC7Mode6(blk); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode6: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e := encodeBC7Mode5(blk, 4); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode5: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode1(blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode1: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode7(blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode7: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode02(bc7Mode0Params, blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode0: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode02(bc7Mode2Params, blk, 64); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode2: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e, ok := encodeBC7Mode3(blk, 16); ok && bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode3: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
		if b, e := encodeBC7Mode4(blk, 4); bc7DecodeSSE(blk, b) != e {
			t.Errorf("%s mode4: reported %d, decoded %d", name, e, bc7DecodeSSE(blk, b))
		}
	}
}

// TestBC7EncodeTwoRegions checks that the partition modes fit
// a block with two distinct flat color regions (which a single subset cannot),
// reaching near-lossless quality.
func TestBC7EncodeTwoRegions(t *testing.T) {
	const w, h = 4, 4
	rgba := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
			if x < 2 {
				rgba[i+0], rgba[i+1], rgba[i+2] = 200, 50, 50
			} else {
				rgba[i+0], rgba[i+1], rgba[i+2] = 50, 50, 200
			}
			rgba[i+3] = 255
		}
	}
	if p := roundTripPSNR(t, rgba, w, h, FormatBC7); p < 45 {
		t.Errorf("two-region block PSNR %.2f dB too low (partition mode not selected?)", p)
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
