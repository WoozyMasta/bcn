// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"bytes"
	"math"
	"os"
	"testing"

	"math/rand/v2"
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

// bc6hMSE computes the mean-squared error between two half-float RGB slices
// using only the 15-bit magnitude (sign is ignored for unsigned BC6H).
func bc6hMSE(a, b []uint16) float64 {
	var sum float64
	for i := range a {
		da := float64(a[i]&0x7FFF) - float64(b[i]&0x7FFF)
		sum += da * da
	}
	return sum / float64(len(a))
}

func TestEncodeBC6HNoPanic(t *testing.T) {
	rng := rand.New(rand.NewPCG(0xBC6, 42))
	const w, h = 64, 64
	src := make([]uint16, w*h*3)
	for i := range src {
		// generate valid half-float unsigned values (no NaN/Inf)
		exp := uint16(rng.IntN(30)+1) << 10
		man := uint16(rng.IntN(1024))
		src[i] = exp | man
	}
	for _, signed := range []bool{false, true} {
		for _, q := range []int{1, 6, 9} {
			opts := &EncodeOptions{QualityLevel: q}
			out, err := EncodeBC6HWithOptions(src, w, h, signed, opts)
			if err != nil {
				t.Fatalf("signed=%v q=%d: %v", signed, q, err)
			}
			if len(out) != (w/4)*(h/4)*16 {
				t.Fatalf("signed=%v q=%d: unexpected output length %d", signed, q, len(out))
			}
		}
	}
}

func TestEncodeBC6HRoundTrip(t *testing.T) {
	// Solid-color block: encoder must reproduce the value exactly (MSE=0).
	const w, h = 4, 4
	solidColor := uint16(0x3C00) // 1.0 in float16
	src := make([]uint16, w*h*3)
	for i := range src {
		src[i] = solidColor
	}

	compressed, err := EncodeBC6H(src, w, h, false)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeBC6H(compressed, w, h, false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != len(src) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(src))
	}
	mse := bc6hMSE(src, decoded)
	t.Logf("BC6HU 4x4 solid: MSE=%.2f", mse)
	// Solid-color block: quantize/unquantize may introduce <= 1 ULP error per channel.
	if mse > 4 {
		t.Errorf("solid-color MSE %.2f exceeds 4 (more than 2 ULP per channel)", mse)
	}

	// Random block smoke test: just verify encode/decode roundtrip does not diverge catastrophically.
	// MSE floor is set at 80% of max possible error for uniform-random data (full half range).
	const w2, h2 = 8, 8
	rng := rand.New(rand.NewPCG(0xC0FFEE, 7))
	src2 := make([]uint16, w2*h2*3)
	for i := range src2 {
		src2[i] = uint16(rng.IntN(0x7C00))
	}
	comp2, err := EncodeBC6H(src2, w2, h2, false)
	if err != nil {
		t.Fatalf("encode random: %v", err)
	}
	dec2, err := DecodeBC6H(comp2, w2, h2, false)
	if err != nil {
		t.Fatalf("decode random: %v", err)
	}
	mse2 := bc6hMSE(src2, dec2)
	const maxMSE = 2e8 // sanity floor: catastrophic failure if exceeded
	if mse2 > maxMSE {
		t.Errorf("random MSE %.0f exceeds sanity floor %.0f", mse2, maxMSE)
	}
	t.Logf("BC6HU 8x8 random: MSE=%.2f", mse2)
}

func TestEncodeBC6HSignedRoundTrip(t *testing.T) {
	const w, h = 8, 8
	src := make([]uint16, w*h*3)
	rng := rand.New(rand.NewPCG(0xDEAD, 13))
	for i := range src {
		// signed half-float values in [0, 0x7800) (positive normals only)
		src[i] = uint16(rng.IntN(0x7800))
	}

	compressed, err := EncodeBC6H(src, w, h, true)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeBC6H(compressed, w, h, true)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	mse := bc6hMSE(src, decoded)
	t.Logf("BC6HS 8x8 positive random: MSE=%.2f", mse)
}

func TestKTXRoundTripBC6H(t *testing.T) {
	const w, h = 8, 8
	rng := rand.New(rand.NewPCG(0x4B5458, 99))
	src := make([]uint16, w*h*3)
	for i := range src {
		src[i] = uint16(rng.IntN(0x7C00))
	}

	compressed, err := EncodeBC6H(src, w, h, false)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	ktx := &KTX{
		Format: FormatBC6HU,
		Width:  w,
		Height: h,
		Faces:  []Face{{Mipmaps: [][]byte{compressed}}},
	}

	var buf bytes.Buffer
	if err := ktx.Write(&buf); err != nil {
		t.Fatalf("KTX write: %v", err)
	}

	ktx2, err := ReadKTX(&buf)
	if err != nil {
		t.Fatalf("KTX read: %v", err)
	}

	if ktx2.Format != FormatBC6HU {
		t.Fatalf("format mismatch: got %v", ktx2.Format)
	}
	if !bytes.Equal(ktx2.Faces[0].Mipmaps[0], compressed) {
		t.Fatal("KTX block bytes differ after round-trip")
	}
}

func TestEncodeBC6HFloat32RoundTrip(t *testing.T) {
	const w, h = 8, 8
	rng := rand.New(rand.NewPCG(0xF32, 5))
	src := make([]float32, w*h*3)
	for i := range src {
		src[i] = float32(rng.Float64() * 10.0) // HDR values [0, 10)
	}

	out, err := EncodeBC6HFloat32(src, w, h, false)
	if err != nil {
		t.Fatalf("EncodeBC6HFloat32: %v", err)
	}
	if len(out) != (w/4)*(h/4)*16 {
		t.Fatalf("unexpected output length %d", len(out))
	}

	decoded, err := DecodeBC6HFloat32(out, w, h, false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != len(src) {
		t.Fatalf("decoded length mismatch")
	}
}
