// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// BC7 (BPTC unorm) encoding.
//
// Each block is encoded by every applicable mode and the lowest-error result wins.
// Opaque blocks use modes 6, 1, 3 and the three-subset modes 0/2;
// alpha blocks use modes 6, 5, 4 and 7.
// Each mode lives in its own bc7_mode*.go file;
// shared endpoint/quantization/ranking helpers live in bc7_common.go.

// EncodeBC7 encodes an RGBA image (NRGBA layout) into BC7 blocks.
func EncodeBC7(rgba []byte, width, height int) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC7, nil)
}

// EncodeBC7WithOptions encodes with explicit options.
// QualityLevel controls the endpoint refinement budget.
func EncodeBC7WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return encodeBlocksWithOptions(rgba, width, height, FormatBC7, opts)
}

// encodeBlockBC7 encodes one 4x4 block, trying the applicable modes
// and keeping the one with the lowest reconstruction error.
// Mode 6 always applies.
// Opaque blocks also try mode 1 (2-subset RGB) and, at higher quality,
// the 3-subset modes 0 and 2;
// alpha blocks try mode 5 (separate color/alpha) and mode 7 (2-subset RGBA).
// The extra modes run only when the quality level enables them.
func encodeBlockBC7(block [16]rgba8, opts EncodeOptions) [16]byte {
	best, bestErr := encodeBC7Mode6(block)
	consider := func(b [16]byte, err int, ok bool) {
		if ok && err < bestErr {
			best, bestErr = b, err
		}
	}

	n := qualitySettingsForOpts(opts).bc7Partitions
	if n == 0 {
		return best
	}

	if bc7BlockHasAlpha(block) {
		rotations := 1
		if n >= 8 { // channel-rotation search is reserved for higher quality
			rotations = 4
		}
		b5, e5 := encodeBC7Mode5(block, rotations)
		consider(b5, e5, true)
		b4, e4 := encodeBC7Mode4(block, rotations)
		consider(b4, e4, true)
		consider(encodeBC7Mode7(block, n))
	} else {
		consider(encodeBC7Mode1(block, n))
		consider(encodeBC7Mode3(block, n))
		if n >= 8 { // 3-subset search is reserved for the higher quality levels
			consider(encodeBC7Mode02(bc7Mode0Params, block, n))
			consider(encodeBC7Mode02(bc7Mode2Params, block, n))
		}
	}

	return best
}

// bc7BlockHasAlpha reports whether any texel is not fully opaque.
func bc7BlockHasAlpha(block [16]rgba8) bool {
	for _, px := range block {
		if px.a != 255 {
			return true
		}
	}

	return false
}
