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
