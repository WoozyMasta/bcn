// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"math"
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
