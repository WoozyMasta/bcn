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

func TestDDSUncompressedBGRX(t *testing.T) {
	img := SolidImage(1, 1, color.NRGBA{R: 5, G: 15, B: 25, A: 40})
	ds, err := EncodeDDS(img, FormatBGRX8)
	if err != nil {
		t.Fatalf("encode dds: %v", err)
	}
	if got := ds.Faces[0].Mipmaps[0]; !bytes.Equal(got, []byte{25, 15, 5, 255}) {
		t.Fatalf("encoded pixel = %v, want [25 15 5 255]", got)
	}

	// Write must also normalize caller-provided BGRX payloads.
	ds.Faces[0].Mipmaps[0][3] = 1
	var buf bytes.Buffer
	if err := ds.Write(&buf); err != nil {
		t.Fatalf("write dds: %v", err)
	}

	read, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("read dds: %v", err)
	}
	if read.Format != FormatBGRX8 {
		t.Fatalf("format = %s, want BGRX8", read.Format)
	}
	if got := read.Faces[0].Mipmaps[0]; !bytes.Equal(got, []byte{25, 15, 5, 255}) {
		t.Fatalf("stored pixel = %v, want [25 15 5 255]", got)
	}
}

func TestDDSUncompressedR8RG8(t *testing.T) {
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
			ds, err := EncodeDDS(img, tt.format)
			if err != nil {
				t.Fatalf("EncodeDDS: %v", err)
			}
			if got := ds.Faces[0].Mipmaps[0]; !bytes.Equal(got, tt.data) {
				t.Fatalf("encoded data = %v, want %v", got, tt.data)
			}

			var buf bytes.Buffer
			if err := ds.Write(&buf); err != nil {
				t.Fatalf("Write: %v", err)
			}
			read, err := ReadDDS(&buf)
			if err != nil {
				t.Fatalf("ReadDDS: %v", err)
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

func TestDDSUncompressedR8SRG8S(t *testing.T) {
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
			ds, err := EncodeDDS(img, tt.format)
			if err != nil {
				t.Fatalf("EncodeDDS: %v", err)
			}
			if got := ds.Faces[0].Mipmaps[0]; !bytes.Equal(got, tt.data) {
				t.Fatalf("encoded data = %v, want %v", got, tt.data)
			}

			var buf bytes.Buffer
			if err := ds.Write(&buf); err != nil {
				t.Fatalf("Write: %v", err)
			}
			read, err := ReadDDS(&buf)
			if err != nil {
				t.Fatalf("ReadDDS: %v", err)
			}
			if read.Format != tt.format {
				t.Fatalf("format = %s, want %s", read.Format, tt.format)
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

func TestDDSUncompressedA8(t *testing.T) {
	img := SolidImage(3, 2, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	ds, err := EncodeDDS(img, FormatA8)
	if err != nil {
		t.Fatalf("EncodeDDS: %v", err)
	}
	wantData := []byte{40, 40, 40, 40, 40, 40}
	if got := ds.Faces[0].Mipmaps[0]; !bytes.Equal(got, wantData) {
		t.Fatalf("encoded data = %v, want %v", got, wantData)
	}

	var buf bytes.Buffer
	if err := ds.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	read, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}
	if read.Format != FormatA8 {
		t.Fatalf("format = %s, want A8", read.Format)
	}
	decoded, err := DecodeImage(read.Faces[0].Mipmaps[0], read.Width, read.Height, read.Format)
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if got := [4]byte(decoded.Pix[:4]); got != [4]byte{0, 0, 0, 40} {
		t.Fatalf("pixel = %v, want [0 0 0 40]", got)
	}
}

func TestRGB10A2EncodeDecode(t *testing.T) {
	const packed = uint32(0xA02003FF) // R=1023, G=0, B=514, A=2.
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, packed)

	decoded, err := DecodeImage(data, 1, 1, FormatRGB10A2)
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if got := [4]byte(decoded.Pix[:4]); got != [4]byte{255, 0, 128, 170} {
		t.Fatalf("decoded pixel = %v, want [255 0 128 170]", got)
	}

	encoded, _, _, err := EncodeImage(SolidImage(1, 1, color.NRGBA{R: 255, G: 0, B: 128, A: 170}), FormatRGB10A2)
	if err != nil {
		t.Fatalf("EncodeImage: %v", err)
	}
	if got := binary.LittleEndian.Uint32(encoded); got != packed {
		t.Fatalf("encoded pixel = %#08x, want %#08x", got, packed)
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
		{"BGRX8", 88, []byte{30, 20, 10, 40}, FormatBGRX8},
		{"BGRX8SRGB", 93, []byte{30, 20, 10, 40}, FormatBGRX8},
		{"R8", 61, []byte{10}, FormatR8},
		{"RG8", 49, []byte{10, 20}, FormatRG8},
		{"R8SNORM", 63, []byte{0x80}, FormatR8S},
		{"RG8SNORM", 51, []byte{0x80, 0x7f}, FormatRG8S},
		{"A8", 65, []byte{40}, FormatA8},
		{"RGB10A2", 24, []byte{0xff, 0x03, 0x20, 0xa0}, FormatRGB10A2},
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
			want := [4]byte{10, 20, 30, 40}
			switch tt.format {
			case FormatBGRX8:
				want[3] = 255
				if got := d.Faces[0].Mipmaps[0][3]; got != 255 {
					t.Fatalf("stored X = %d, want 255", got)
				}
			case FormatR8:
				want = [4]byte{10, 10, 10, 255}
			case FormatRG8:
				want = [4]byte{10, 20, 0, 255}
			case FormatRGB10A2:
				want = [4]byte{255, 0, 128, 170}
			case FormatR8S:
				want = [4]byte{0, 0, 0, 255}
			case FormatRG8S:
				want = [4]byte{0, 255, 0, 255}
			case FormatA8:
				want = [4]byte{0, 0, 0, 40}
			}
			if got := [4]byte(img.Pix[:4]); got != want {
				t.Fatalf("pixel = %v, want %v", got, want)
			}
		})
	}
}
