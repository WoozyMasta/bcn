// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build !amd64 || purego

package bcn

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	return findMinMaxGeneric(block)
}

// decodeRangeDXT1ASM is unavailable without assembly support.
func decodeRangeDXT1ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeDXT3ASM is unavailable without assembly support.
func decodeRangeDXT3ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeDXT5ASM is unavailable without assembly support.
func decodeRangeDXT5ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeBC4ASM is unavailable without assembly support.
func decodeRangeBC4ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeBC5ASM is unavailable without assembly support.
func decodeRangeBC5ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// packDXT1IndicesASM is unavailable without assembly support.
func packDXT1IndicesASM(_ *[16]rgba8, _ *[4]rgba8, _ bool, _ uint8, _ rgbWeightsFP) (uint32, bool) {
	return 0, false
}

// scoreDXT1PaletteASM is unavailable without assembly support.
func scoreDXT1PaletteASM(_ *[16]rgba8, _, _ uint16, _ rgbWeightsFP) (int64, bool) {
	return 0, false
}

// bc7Mode6IndicesASM is unavailable without assembly support.
func bc7Mode6IndicesASM(_ *[16]rgba8, _ *[16]rgba8) ([16]uint8, int, bool) {
	return [16]uint8{}, 0, false
}

// bc7Color4LSQASM is unavailable without assembly support.
func bc7Color4LSQASM(_ *[16]rgba8, _ *[4]rgba8) (lsqColorSums, bool) {
	return lsqColorSums{}, false
}

// alphaBlockErrorASM is unavailable without assembly support.
func alphaBlockErrorASM(_ *[16]uint8, _, _ uint8) (int, bool) {
	return 0, false
}

// bestAlphaIndices16ASM is unavailable without assembly support.
func bestAlphaIndices16ASM(_ *[16]uint8, _, _ uint8) (uint64, bool) {
	return 0, false
}

// lsqColorAccumulateASM is unavailable without assembly support.
func lsqColorAccumulateASM(_ *[16]rgba8, _ *[4]rgba8, _ bool, _ uint8, _ rgbWeightsFP, _ int, _ *[4]int) (lsqColorSums, bool) {
	return lsqColorSums{}, false
}

// lsqAlphaAccumulateASM is unavailable without assembly support.
func lsqAlphaAccumulateASM(_ *[16]uint8, _, _ uint8) (lsqAlphaSums, bool) {
	return lsqAlphaSums{}, false
}

// downscaleNRGBARow2xASM is unavailable without assembly support.
func downscaleNRGBARow2xASM(_, _, _ []byte, _ int) bool {
	return false
}
