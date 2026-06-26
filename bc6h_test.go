// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"bytes"
	"math"
	"os"
	"testing"
)

// knownHalves lists float32 values paired with their exact IEEE 754 half-float bit patterns
// for spot-checking float32ToFloat16 and float16ToFloat32.
var knownHalves = []struct {
	f32  float32
	bits uint16
}{
	{0.0, 0x0000},
	{math.Float32frombits(0x80000000), 0x8000}, // -0.0
	{1.0, 0x3c00},
	{-1.0, 0xbc00},
	{2.0, 0x4000},
	{0.5, 0x3800},
	{-0.5, 0xb800},
	{65504.0, 0x7bff},  // max finite half
	{-65504.0, 0xfbff}, // min finite half (most negative)
	// smallest positive normal half
	{float32(math.Ldexp(1, -14)), 0x0400},
	// smallest positive subnormal half
	{float32(math.Ldexp(1, -24)), 0x0001},
}

func TestHalfKnownValues(t *testing.T) {
	for _, tc := range knownHalves {
		got := float32ToFloat16(tc.f32)
		if got != tc.bits {
			t.Errorf("float32ToFloat16(%v): got 0x%04x, want 0x%04x", tc.f32, got, tc.bits)
		}
		back := float16ToFloat32(tc.bits)
		if math.Float32bits(back) != math.Float32bits(tc.f32) {
			t.Errorf("float16ToFloat32(0x%04x): got %v (0x%08x), want %v (0x%08x)",
				tc.bits, back, math.Float32bits(back), tc.f32, math.Float32bits(tc.f32))
		}
	}
}

func TestHalfSpecialCases(t *testing.T) {
	// +Inf
	infBits := float32ToFloat16(float32(math.Inf(1)))
	if infBits != 0x7c00 {
		t.Errorf("+Inf: got 0x%04x, want 0x7c00", infBits)
	}
	if !math.IsInf(float64(float16ToFloat32(0x7c00)), 1) {
		t.Error("float16ToFloat32(0x7c00) should be +Inf")
	}

	// -Inf
	ninfBits := float32ToFloat16(float32(math.Inf(-1)))
	if ninfBits != 0xfc00 {
		t.Errorf("-Inf: got 0x%04x, want 0xfc00", ninfBits)
	}
	if !math.IsInf(float64(float16ToFloat32(0xfc00)), -1) {
		t.Error("float16ToFloat32(0xfc00) should be -Inf")
	}

	// NaN -> quiet NaN half
	nanHalf := float32ToFloat16(float32(math.NaN()))
	if nanHalf&0x7c00 != 0x7c00 || nanHalf&0x03ff == 0 {
		t.Errorf("NaN half: got 0x%04x, expected exponent=0x1f and non-zero mantissa", nanHalf)
	}
	if !math.IsNaN(float64(float16ToFloat32(nanHalf))) {
		t.Error("float16ToFloat32 of NaN half should be NaN")
	}

	// overflow -> Inf
	overflowBits := float32ToFloat16(1e10)
	if overflowBits != 0x7c00 {
		t.Errorf("overflow: got 0x%04x, want 0x7c00", overflowBits)
	}

	// underflow below subnormal range -> zero
	underflowBits := float32ToFloat16(float32(math.Ldexp(1, -30)))
	if underflowBits != 0x0000 {
		t.Errorf("underflow: got 0x%04x, want 0x0000", underflowBits)
	}
}

func TestHalfRoundTrip(t *testing.T) {
	// Every representable half-float value must survive a round-trip through float32.
	for h := uint16(0); ; h++ {
		f := float16ToFloat32(h)
		back := float32ToFloat16(f)
		// NaN halves may not round-trip bit-for-bit (quiet vs signaling),
		// but the exponent field must still be 0x1f and mantissa non-zero.
		isNaN := (h>>10)&0x1f == 0x1f && h&0x3ff != 0
		if isNaN {
			if (back>>10)&0x1f != 0x1f || back&0x3ff == 0 {
				t.Errorf("NaN half 0x%04x -> float32 -> 0x%04x (not NaN)", h, back)
			}
		} else if back != h {
			t.Errorf("round-trip failed: 0x%04x -> %v -> 0x%04x", h, f, back)
		}
		if h == 0xffff {
			break
		}
	}
}

const bc6hTestDDS = "ref/bcdec/test_images/lythwood_room_1k_bc6h_signed.dds"

// TestDecodeBC6HNoPanic ensures decodeBlockBC6H never panics on any 16-byte input,
// covering all valid and reserved modes for both signed and unsigned.
func TestDecodeBC6HNoPanic(t *testing.T) {
	// Exercise a variety of 16-byte blocks including reserved mode bit patterns.
	probes := [][16]byte{
		{}, // all zeros
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // all ones
		{0x02}, // mode 3 (spec mode 3)
		{0x06}, // mode 4
		{0x0a}, // mode 5
		{0x0e}, // mode 6
		{0x12}, // mode 7
		{0x16}, // mode 8
		{0x1a}, // mode 9
		{0x1e}, // mode 10 (2-subset non-transformed)
		{0x03}, // mode 11 (1-subset non-transformed)
		{0x07}, // mode 12
		{0x0b}, // mode 13
		{0x0f}, // mode 14
		{0x13}, // reserved
		{0x17}, // reserved
		{0x1b}, // reserved
		{0x1f}, // reserved
	}
	for _, blk := range probes {
		_ = decodeBlockBC6H(blk[:], false)
		_ = decodeBlockBC6H(blk[:], true)
	}
}

// FuzzDecodeBC6HNoPanic is a fuzz target: no 16-byte input must cause a panic.
func FuzzDecodeBC6HNoPanic(f *testing.F) {
	f.Add([]byte{0x03, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{0x0f, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) != 16 {
			return
		}
		_ = decodeBlockBC6H(b, false)
		_ = decodeBlockBC6H(b, true)
	})
}

// TestDecodeBC6HFromFile decodes the signed BC6H DDS test image
// and checks that the output has the right length and that pixel values are finite half-floats
// (no value in [0x7c01..0x7fff] - those are NaN in half-float).
func TestDecodeBC6HFromFile(t *testing.T) {
	f, err := os.Open(bc6hTestDDS)
	if err != nil {
		t.Skipf("test image not found: %v", err)
	}
	defer f.Close()

	dds, err := ReadDDS(f)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}
	if dds.Format != FormatBC6HS {
		t.Fatalf("expected FormatBC6HS, got %v", dds.Format)
	}

	raw := dds.Faces[0].Mipmaps[0]
	w, h := dds.Width, dds.Height

	pixels, err := DecodeBC6H(raw, w, h, true)
	if err != nil {
		t.Fatalf("DecodeBC6H: %v", err)
	}
	if len(pixels) != w*h*3 {
		t.Fatalf("pixel length: got %d, want %d", len(pixels), w*h*3)
	}

	// Verify no NaN half-float values in the output (sign-magnitude half, signed mode).
	// For signed BC6H, values use sign-magnitude layout; NaN would be 0x7c01..0x7ffe.
	nanCount := 0
	for _, v := range pixels {
		exp := (v >> 10) & 0x1f
		mant := v & 0x3ff
		if exp == 0x1f && mant != 0 {
			nanCount++
		}
	}
	if nanCount > 0 {
		t.Errorf("decoded %d NaN half-float values (unexpected for a valid HDR image)", nanCount)
	}
}

// TestDDSBC6HRoundTrip reads the signed BC6H DDS, writes it back, re-reads it,
// and verifies the raw block bytes are identical.
func TestDDSBC6HRoundTrip(t *testing.T) {
	f, err := os.Open(bc6hTestDDS)
	if err != nil {
		t.Skipf("test image not found: %v", err)
	}
	defer f.Close()

	dds, err := ReadDDS(f)
	if err != nil {
		t.Fatalf("ReadDDS: %v", err)
	}

	var buf bytes.Buffer
	if err := dds.Write(&buf); err != nil {
		t.Fatalf("DDS.Write: %v", err)
	}

	dds2, err := ReadDDS(&buf)
	if err != nil {
		t.Fatalf("ReadDDS (round-trip): %v", err)
	}

	if dds2.Format != dds.Format {
		t.Fatalf("format mismatch: got %v, want %v", dds2.Format, dds.Format)
	}
	if dds2.Width != dds.Width || dds2.Height != dds.Height {
		t.Fatalf("dimension mismatch: got %dx%d, want %dx%d", dds2.Width, dds2.Height, dds.Width, dds.Height)
	}
	if len(dds2.Faces) != len(dds.Faces) || len(dds2.Faces[0].Mipmaps) != len(dds.Faces[0].Mipmaps) {
		t.Fatalf("face/mip count mismatch")
	}
	if !bytes.Equal(dds2.Faces[0].Mipmaps[0], dds.Faces[0].Mipmaps[0]) {
		t.Fatal("mip0 block bytes differ after DDS round-trip")
	}
}
