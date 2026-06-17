package bcn

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
)

// bc7BitWriter mirrors bptcReader: it appends fields LSB-first into
// a 128-bit little-endian window,
// so hand-built blocks decode with the production reader.
type bc7BitWriter struct {
	lo, hi uint64
	pos    int
}

func (w *bc7BitWriter) put(v uint32, n int) {
	for k := 0; k < n; k++ {
		if v&(1<<uint(k)) != 0 {
			if w.pos < 64 {
				w.lo |= 1 << uint(w.pos)
			} else {
				w.hi |= 1 << uint(w.pos-64)
			}
		}
		w.pos++
	}
}

func (w *bc7BitWriter) block() []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b[0:8], w.lo)
	binary.LittleEndian.PutUint64(b[8:16], w.hi)
	return b
}

// bc7Mode6 builds a single-subset BC7 mode 6 block.
// Each endpoint channel is 7 bits plus a per-endpoint P-bit;
// texel 0 is the anchor (3-bit index), the rest use 4-bit indices.
func bc7Mode6(r0, g0, b0, a0, p0, r1, g1, b1, a1, p1 uint32, idx [16]uint32) []byte {
	var w bc7BitWriter
	w.put(0, 6) // mode 6: six leading zeros ...
	w.put(1, 1) // ... then the set bit
	w.put(r0, 7)
	w.put(r1, 7)
	w.put(g0, 7)
	w.put(g1, 7)
	w.put(b0, 7)
	w.put(b1, 7)
	w.put(a0, 7)
	w.put(a1, 7)
	w.put(p0, 1)
	w.put(p1, 1)
	w.put(idx[0], 3) // anchor texel: one fewer bit
	for i := 1; i < 16; i++ {
		w.put(idx[i], 4)
	}
	return w.block()
}

// px returns the RGBA quad for texel (row, col) of a decoded 4x4 block.
func px(block [64]byte, row, col int) [4]byte {
	o := (row*4 + col) * 4
	return [4]byte{block[o], block[o+1], block[o+2], block[o+3]}
}

func TestBC7DecodeMode6Solid(t *testing.T) {
	// Equal endpoints (p-bit 0): every texel decodes to (v7<<1) per channel.
	var idx [16]uint32
	block := bc7Mode6(100, 50, 30, 120, 0, 100, 50, 30, 120, 0, idx)
	want := [4]byte{200, 100, 60, 240}

	got := decodeBlockBC7(block)
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			if p := px(got, row, col); p != want {
				t.Fatalf("texel (%d,%d) = %v, want %v", row, col, p, want)
			}
		}
	}
}

func TestBC7DecodeMode6Endpoints(t *testing.T) {
	// Distinct endpoints; index 0 selects ep0, index 15 selects ep1.
	idx := [16]uint32{}
	idx[1] = 15
	block := bc7Mode6(100, 50, 30, 120, 0, 20, 110, 10, 60, 0, idx)
	ep0 := [4]byte{200, 100, 60, 240}
	ep1 := [4]byte{40, 220, 20, 120}

	got := decodeBlockBC7(block)
	if p := px(got, 0, 0); p != ep0 {
		t.Errorf("texel (0,0) idx0 = %v, want ep0 %v", p, ep0)
	}
	if p := px(got, 0, 1); p != ep1 {
		t.Errorf("texel (0,1) idx15 = %v, want ep1 %v", p, ep1)
	}
	if p := px(got, 1, 1); p != ep0 {
		t.Errorf("texel (1,1) idx0 = %v, want ep0 %v", p, ep0)
	}
}

func TestBC7DecodeNoPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	dims := [][2]int{{1, 1}, {3, 5}, {4, 4}, {7, 9}, {16, 16}}
	for _, d := range dims {
		w, h := d[0], d[1]
		bx := (w + 3) / 4
		by := (h + 3) / 4
		full := bx * by * 16
		for trial := 0; trial < 64; trial++ {
			n := full
			if trial%4 == 0 && full > 0 {
				n = rng.Intn(full) // truncated payloads must error, not panic
			}
			data := make([]byte, n)
			rng.Read(data)

			out, err := decodeBlocksWithOptions(data, w, h, FormatBC7, &DecodeOptions{Workers: 1})
			if err != nil {
				continue
			}
			if len(out) != w*h*4 {
				t.Fatalf("BC7 %dx%d: output length %d, want %d", w, h, len(out), w*h*4)
			}
		}
	}
}

func TestBC7KTXRoundTripRead(t *testing.T) {
	idx := [16]uint32{}
	block := bc7Mode6(100, 50, 30, 120, 0, 100, 50, 30, 120, 0, idx)
	want := [4]byte{200, 100, 60, 240}

	k := &KTX{Format: FormatBC7, Width: 4, Height: 4, Faces: []Face{{Mipmaps: [][]byte{block}}}}
	var buf bytes.Buffer
	if err := k.Write(&buf); err != nil {
		t.Fatalf("KTX write: %v", err)
	}

	got, err := ReadKTX(&buf)
	if err != nil {
		t.Fatalf("ReadKTX: %v", err)
	}
	if got.Format != FormatBC7 || got.Width != 4 || got.Height != 4 {
		t.Fatalf("KTX header: fmt=%s %dx%d", got.Format, got.Width, got.Height)
	}

	img, err := DecodeImage(got.Faces[0].Mipmaps[0], got.Width, got.Height, got.Format)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p := [4]byte(img.Pix[0:4]); p != want {
		t.Fatalf("pixel 0 = %v, want %v", p, want)
	}
}

func TestBC7DDSDX10Read(t *testing.T) {
	idx := [16]uint32{}
	block := bc7Mode6(100, 50, 30, 120, 0, 100, 50, 30, 120, 0, idx)

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(DDSMagic)); err != nil {
		t.Fatal(err)
	}
	hdr := DDSHeader{
		Size:   DDSHeaderSize,
		Flags:  DDSFlagCaps | DDSFlagHeight | DDSFlagWidth | DDSFlagPixelFormat,
		Width:  4,
		Height: 4,
		Caps:   DDSCapsTexture,
		PixelFormat: DDSPixelFormat{
			Size:   DDSPixelFormatSize,
			Flags:  DDSPFFourCC,
			FourCC: makeFourCC('D', 'X', '1', '0'),
		},
	}
	if err := binary.Write(&buf, binary.LittleEndian, &hdr); err != nil {
		t.Fatal(err)
	}
	dx10 := DDSHeaderDX10{DXGIFormat: 98, ResourceDimension: 3, ArraySize: 1} // BC7_UNORM
	if err := binary.Write(&buf, binary.LittleEndian, &dx10); err != nil {
		t.Fatal(err)
	}
	buf.Write(block)

	d, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}
	if d.Format != FormatBC7 || d.Width != 4 || d.Height != 4 {
		t.Fatalf("DDS header: fmt=%s %dx%d", d.Format, d.Width, d.Height)
	}

	img, err := DecodeImage(d.Faces[0].Mipmaps[0], d.Width, d.Height, d.Format)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p := [4]byte(img.Pix[0:4]); p != [4]byte{200, 100, 60, 240} {
		t.Fatalf("pixel 0 = %v, want {200 100 60 240}", p)
	}
}
