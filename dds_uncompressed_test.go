package bcn

import (
	"bytes"
	"encoding/binary"
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

func TestDDSDX10UncompressedRead(t *testing.T) {
	tests := []struct {
		name    string
		dxgi    uint32
		payload []byte
		format  Format
	}{
		{"RGBA8", 28, []byte{10, 20, 30, 40}, FormatRGBA8},
		{"RGBA8SRGB", 29, []byte{10, 20, 30, 40}, FormatRGBA8},
		{"BGRA8", 87, []byte{30, 20, 10, 40}, FormatBGRA8},
		{"BGRA8SRGB", 91, []byte{30, 20, 10, 40}, FormatBGRA8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			for _, value := range []any{
				uint32(DDSMagic),
				&DDSHeader{
					Size:              DDSHeaderSize,
					Flags:             DDSFlagCaps | DDSFlagHeight | DDSFlagWidth | DDSFlagPitch | DDSFlagPixelFormat,
					Width:             1,
					Height:            1,
					PitchOrLinearSize: 4,
					Caps:              DDSCapsTexture,
					PixelFormat: DDSPixelFormat{
						Size:   DDSPixelFormatSize,
						Flags:  DDSPFFourCC,
						FourCC: DDSFourCCDX10,
					},
				},
				&DDSHeaderDX10{DXGIFormat: tt.dxgi, ResourceDimension: 3, ArraySize: 1},
			} {
				if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
					t.Fatal(err)
				}
			}
			buf.Write(tt.payload)

			d, err := ReadDDS(&buf)
			if err != nil {
				t.Fatalf("ReadDDS: %v", err)
			}
			if d.Format != tt.format {
				t.Fatalf("format = %s, want %s", d.Format, tt.format)
			}

			img, err := DecodeImage(d.Faces[0].Mipmaps[0], d.Width, d.Height, d.Format)
			if err != nil {
				t.Fatalf("DecodeImage: %v", err)
			}
			if got := [4]byte(img.Pix[:4]); got != [4]byte{10, 20, 30, 40} {
				t.Fatalf("pixel = %v, want {10 20 30 40}", got)
			}
		})
	}
}
