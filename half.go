// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "math"

// float16ToFloat32 converts an IEEE 754 half-precision float to float32.
// Subnormals, infinities, and NaNs are handled correctly.
func float16ToFloat32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32((h >> 10) & 0x1f)
	mant := uint32(h & 0x3ff)

	switch exp {
	case 0:
		if mant == 0 {
			// +/-zero
			return math.Float32frombits(sign)
		}

		// Subnormal half -> normalized float32.
		// Shift mantissa left until the implicit leading 1 is in place.
		exp = 1
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		mant &= 0x3ff

		return math.Float32frombits(sign | ((exp + (127 - 15)) << 23) | (mant << 13))

	case 31:
		// Infinity or NaN.
		return math.Float32frombits(sign | 0x7f800000 | (mant << 13))

	default:
		return math.Float32frombits(sign | ((exp + (127 - 15)) << 23) | (mant << 13))
	}
}

// float32ToFloat16 converts a float32 to IEEE 754 half-precision.
// Round-to-nearest-even. Overflow produces +/-Inf half.
// NaN inputs produce a quiet NaN half. Subnormals are supported.
func float32ToFloat16(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16(bits>>31) << 15
	exp := int32((bits>>23)&0xff) - 127 + 15 // half-biased exponent
	mant := bits & 0x7fffff

	// Inf or NaN.
	if (bits & 0x7fffffff) >= 0x7f800000 {
		if mant != 0 {
			return sign | 0x7e00 // quiet NaN
		}
		return sign | 0x7c00 // Inf
	}

	if exp >= 31 {
		// Overflow -> Inf.
		return sign | 0x7c00
	}

	if exp <= 0 {
		// Result is a half subnormal or flushes to zero.
		// Correct shift: the 24-bit significand (implicit 1 + 23 mantissa bits)
		// must be right-shifted by (14 - exp) positions to land in the 10-bit half mantissa.
		shift := uint32(14 - exp)
		if shift > 24 {
			return sign // too small even to round up to 0x0001
		}

		significand := mant | 0x800000 // include implicit leading 1
		halfMant := significand >> shift
		// Round half-to-even using the discarded bits.
		roundBit := uint32(1) << (shift - 1)
		sticky := significand & (roundBit - 1)
		if significand&roundBit != 0 && (sticky != 0 || halfMant&1 != 0) {
			halfMant++
		}

		return sign | uint16(halfMant)
	}

	// Normal half.
	// Round the 23-bit mantissa to 10 bits (round-to-nearest-even).
	// Bit 12 is the first discarded bit.
	const roundBit = 1 << 12
	halfMant := uint16(mant >> 13)
	roundUp := mant&roundBit != 0 && (mant&(roundBit-1) != 0 || halfMant&1 != 0)
	if roundUp {
		halfMant++
		if halfMant == 0x400 {
			// Mantissa overflow: carry into exponent.
			exp++
			halfMant = 0
			if exp >= 31 {
				return sign | 0x7c00 // overflow to Inf
			}
		}
	}

	return sign | uint16(exp)<<10 | halfMant
}
