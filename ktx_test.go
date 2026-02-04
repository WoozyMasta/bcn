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
	ktx, err := EncodeKTXWithOptions([]image.Image{img}, FormatDXT1, &EncodeOptions{Quality: QualityFast})
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
	if read.Width != 8 || read.Height != 8 || read.Format != FormatDXT1 {
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
	ktx, err := EncodeKTXWithOptions(images, FormatDXT1, &EncodeOptions{GenerateMipmaps: true, Quality: QualityFast})
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
	ktx, err := EncodeKTXWithOptions([]image.Image{img}, FormatBC5, &EncodeOptions{Quality: QualityFast})
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

func TestKTXRejectArrays(t *testing.T) {
	header := KTXHeader{
		Identifier:            KTXIdentifier,
		Endianness:            ktxEndianness,
		GlType:                0,
		GlTypeSize:            1,
		GlFormat:              0,
		GlInternalFormat:      KTXGLCompressedRGBAS3TCDXT1,
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
