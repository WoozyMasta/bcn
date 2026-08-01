package bcn

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestKTXRoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 120, G: 80, B: 200, A: 255})
	ktx, err := EncodeKTXWithOptions([]image.Image{img}, FormatBC1, &EncodeOptions{QualityLevel: QualityLevelFast})
	if err != nil {
		t.Fatalf("encode ktx: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := ktx.Write(buf); err != nil {
		t.Fatalf("write ktx: %v", err)
	}
	read, err := ReadKTX(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read ktx: %v", err)
	}
	if read.Width != 8 || read.Height != 8 || read.Format != FormatBC1 {
		t.Fatalf("ktx header mismatch")
	}
	if len(read.Faces) != 1 || len(read.Faces[0].Mipmaps) != 1 {
		t.Fatalf("expected 1 face/1 mip")
	}
}

func TestKTXCubemapMipmaps(t *testing.T) {
	images := []image.Image{
		SolidImage(8, 8, color.NRGBA{R: 255, G: 0, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 255, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 0, B: 255, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 255, G: 255, B: 0, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 255, G: 0, B: 255, A: 255}),
		SolidImage(8, 8, color.NRGBA{R: 0, G: 255, B: 255, A: 255}),
	}
	ktx, err := EncodeKTXWithOptions(images, FormatBC1, &EncodeOptions{GenerateMipmaps: true, QualityLevel: QualityLevelFast})
	if err != nil {
		t.Fatalf("encode cubemap: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := ktx.Write(buf); err != nil {
		t.Fatalf("write ktx: %v", err)
	}
	read, err := ReadKTX(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read ktx: %v", err)
	}
	if !read.IsCubemap() {
		t.Fatalf("expected cubemap")
	}
	if len(read.Faces[0].Mipmaps) != len(ktx.Faces[0].Mipmaps) {
		t.Fatalf("mipmap count mismatch")
	}
}

func TestKTXBC5RoundTrip(t *testing.T) {
	img := SolidImage(8, 8, color.NRGBA{R: 40, G: 210, B: 10, A: 255})
	ktx, err := EncodeKTXWithOptions([]image.Image{img}, FormatBC5, &EncodeOptions{QualityLevel: QualityLevelFast})
	if err != nil {
		t.Fatalf("encode ktx: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := ktx.Write(buf); err != nil {
		t.Fatalf("write ktx: %v", err)
	}
	read, err := ReadKTX(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read ktx: %v", err)
	}
	if read.Format != FormatBC5 {
		t.Fatalf("expected bc5")
	}
}

func TestKTXBC4SBC5SRoundTrip(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 128, G: 128, A: 255})
	for _, format := range []Format{FormatBC4S, FormatBC5S} {
		ktx, err := EncodeKTX(img, format)
		if err != nil {
			t.Fatalf("encode %s: %v", format, err)
		}
		var buf bytes.Buffer
		if err := ktx.Write(&buf); err != nil {
			t.Fatalf("write %s: %v", format, err)
		}
		read, err := ReadKTX(&buf)
		if err != nil {
			t.Fatalf("read %s: %v", format, err)
		}
		if read.Format != format {
			t.Fatalf("format = %s, want %s", read.Format, format)
		}
	}
}

func TestKTXUncompressedRoundTrip(t *testing.T) {
	img := SolidImage(4, 4, color.NRGBA{R: 255, G: 128, B: 64, A: 255})
	for _, format := range []Format{FormatRGBA8, FormatBGRA8, FormatRGB10A2} {
		ktx, err := EncodeKTXWithOptions([]image.Image{img}, format, nil)
		if err != nil {
			t.Fatalf("encode ktx %s: %v", format, err)
		}
		buf := &bytes.Buffer{}
		if err := ktx.Write(buf); err != nil {
			t.Fatalf("write ktx %s: %v", format, err)
		}
		read, err := ReadKTX(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("read ktx %s: %v", format, err)
		}
		if read.Format != format || read.Width != 4 || read.Height != 4 {
			t.Fatalf("ktx %s header mismatch: got format %v %dx%d", format, read.Format, read.Width, read.Height)
		}
		_, decoded, err := DecodeKTX(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("decode ktx %s: %v", format, err)
		}
		if decoded == nil || decoded.Bounds().Dx() != 4 || decoded.Bounds().Dy() != 4 {
			t.Fatalf("ktx %s decode size mismatch", format)
		}
	}
}

func TestKTXUncompressedR8RG8(t *testing.T) {
	img := SolidImage(3, 2, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	tests := []struct {
		format Format
		data   []byte
		pixel  [4]byte
	}{
		{FormatR8, []byte{10, 10, 10, 10, 10, 10}, [4]byte{10, 10, 10, 255}},
		{FormatRG8, []byte{10, 20, 10, 20, 10, 20, 10, 20, 10, 20, 10, 20}, [4]byte{10, 20, 0, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			ktx, err := EncodeKTX(img, tt.format)
			if err != nil {
				t.Fatalf("EncodeKTX: %v", err)
			}
			var buf bytes.Buffer
			if err := ktx.Write(&buf); err != nil {
				t.Fatalf("Write: %v", err)
			}
			read, err := ReadKTX(&buf)
			if err != nil {
				t.Fatalf("ReadKTX: %v", err)
			}
			if read.Format != tt.format {
				t.Fatalf("format = %s, want %s", read.Format, tt.format)
			}
			if got := read.Faces[0].Mipmaps[0]; !bytes.Equal(got, tt.data) {
				t.Fatalf("stored data = %v, want %v", got, tt.data)
			}
			decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
			if err != nil {
				t.Fatalf("DecodeImage: %v", err)
			}
			if got := [4]byte(decoded.Pix[:4]); got != tt.pixel {
				t.Fatalf("pixel = %v, want %v", got, tt.pixel)
			}
		})
	}
}

func TestKTXUncompressedR8SRG8S(t *testing.T) {
	img := SolidImage(3, 2, color.NRGBA{R: 0, G: 128, B: 30, A: 40})
	tests := []struct {
		format Format
		data   []byte
		pixel  [4]byte
	}{
		{FormatR8S, []byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x81}, [4]byte{0, 0, 0, 255}},
		{FormatRG8S, []byte{0x81, 0, 0x81, 0, 0x81, 0, 0x81, 0, 0x81, 0, 0x81, 0}, [4]byte{0, 128, 0, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			ktx, err := EncodeKTX(img, tt.format)
			if err != nil {
				t.Fatalf("EncodeKTX: %v", err)
			}
			var buf bytes.Buffer
			if err := ktx.Write(&buf); err != nil {
				t.Fatalf("Write: %v", err)
			}
			read, err := ReadKTX(&buf)
			if err != nil {
				t.Fatalf("ReadKTX: %v", err)
			}
			if read.Format != tt.format {
				t.Fatalf("format = %s, want %s", read.Format, tt.format)
			}
			if got := read.Faces[0].Mipmaps[0]; !bytes.Equal(got, tt.data) {
				t.Fatalf("stored data = %v, want %v", got, tt.data)
			}
			decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
			if err != nil {
				t.Fatalf("DecodeImage: %v", err)
			}
			if got := [4]byte(decoded.Pix[:4]); got != tt.pixel {
				t.Fatalf("pixel = %v, want %v", got, tt.pixel)
			}
		})
	}
}

func TestKTXUncompressedA8(t *testing.T) {
	img := SolidImage(3, 2, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	ktx, err := EncodeKTX(img, FormatA8)
	if err != nil {
		t.Fatalf("EncodeKTX: %v", err)
	}
	var buf bytes.Buffer
	if err := ktx.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	read, err := ReadKTX(&buf)
	if err != nil {
		t.Fatalf("ReadKTX: %v", err)
	}
	if read.Format != FormatA8 {
		t.Fatalf("format = %s, want A8", read.Format)
	}
	if got := read.Faces[0].Mipmaps[0]; !bytes.Equal(got, []byte{40, 40, 40, 40, 40, 40}) {
		t.Fatalf("stored data = %v, want [40 40 40 40 40 40]", got)
	}
	decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if got := [4]byte(decoded.Pix[:4]); got != [4]byte{0, 0, 0, 40} {
		t.Fatalf("pixel = %v, want [0 0 0 40]", got)
	}
}

func TestKTXRejectArrays(t *testing.T) {
	header := KTXHeader{
		Identifier:            KTXIdentifier,
		Endianness:            ktxEndianness,
		GlType:                0,
		GlTypeSize:            1,
		GlFormat:              0,
		GlInternalFormat:      KTXGLCompressedRGBAS3TCBC1,
		GlBaseInternalFormat:  KTXGLRGBA,
		PixelWidth:            4,
		PixelHeight:           4,
		NumberOfArrayElements: 2,
		NumberOfFaces:         1,
		NumberOfMipmapLevels:  1,
		BytesOfKeyValueData:   0,
	}
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, &header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := ReadKTX(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("expected error for array KTX")
	}
}
