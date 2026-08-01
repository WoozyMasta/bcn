// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build !amd64 || purego

package bcn

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	return findMinMaxGeneric(block)
}

// decodeRangeBC1ASM is unavailable without assembly support.
func decodeRangeBC1ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeBC2ASM is unavailable without assembly support.
func decodeRangeBC2ASM(_, _ []byte, _, _, _, _, _ int) bool {
	return false
}

// decodeRangeBC3ASM is unavailable without assembly support.
func decodeRangeBC3ASM(_, _ []byte, _, _, _, _, _ int) bool {
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

// packBC1IndicesASM is unavailable without assembly support.
func packBC1IndicesASM(_ *[16]rgba8, _ *[4]rgba8, _ bool, _ uint8, _ rgbWeightsFP) (uint32, bool) {
	return 0, false
}

// scoreBC1PaletteASM is unavailable without assembly support.
func scoreBC1PaletteASM(_ *[16]rgba8, _, _ uint16, _ rgbWeightsFP) (int64, bool) {
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

// bc7SubsetEvalASM is unavailable without assembly support.
func bc7SubsetEvalASM(_ *[16]rgba8, _ *[16]uint8, _ uint8, _ []rgba8, _ []int32) (lsqColorSums, int, bool) {
	return lsqColorSums{}, 0, false
}

// bc7Mode7SubsetEvalASM is unavailable without assembly support.
func bc7Mode7SubsetEvalASM(_ *[16]rgba8, _ *[16]uint8, _ uint8, _ *[4]rgba8) (bc7Mode7Sums, int, bool) {
	return bc7Mode7Sums{}, 0, false
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

// bc6hFindIdx1ASM is unavailable without assembly support.
func bc6hFindIdx1ASM(_ *[48]int32, _, _ [3]int) ([16]byte, bool) {
	return [16]byte{}, false
}

// bc6hFindIdx2ASM is unavailable without assembly support.
func bc6hFindIdx2ASM(_ *[48]int32, _, _ [3]int, _, _ int) ([16]byte, bool) {
	return [16]byte{}, false
}
