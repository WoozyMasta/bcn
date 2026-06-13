// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

//go:generate go run ./asm -out kernels_amd64.s -stubs kernels_stubs_amd64.go -pkg bcn

package bcn

import "unsafe"

// findMinMax returns per-channel min and max colors inside a 4x4 block.
func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	if useASM {
		// [16]rgba8 is 64 contiguous bytes (four uint8 fields, no padding).
		v := findMinMaxSSE2((*[64]byte)(unsafe.Pointer(&block)))
		return rgba8FromPacked(uint32(v)), rgba8FromPacked(uint32(v >> 32)) // #nosec G115 -- intentional 32-bit halves.
	}

	return findMinMaxGeneric(block)
}

// rgba8FromPacked splits a packed little-endian RGBA word.
func rgba8FromPacked(v uint32) rgba8 {
	// #nosec G115 -- intentional byte extraction.
	return rgba8{r: uint8(v), g: uint8(v >> 8), b: uint8(v >> 16), a: uint8(v >> 24)}
}
