// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build !amd64 || purego

package bcn

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	return findMinMaxGeneric(block)
}
