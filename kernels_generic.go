// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// Pure-Go reference implementations of the SIMD kernel contracts. They serve
// as the fallback on platforms without assembly and as the byte-exact oracle
// in kernel equivalence tests.

// findMinMaxGeneric returns per-channel min and max colors inside a 4x4 block.
func findMinMaxGeneric(block [16]rgba8) (rgba8, rgba8) {
	minC := rgba8{255, 255, 255, 255}
	maxC := rgba8{0, 0, 0, 0}
	for _, c := range block {
		if c.r < minC.r {
			minC.r = c.r
		}
		if c.g < minC.g {
			minC.g = c.g
		}
		if c.b < minC.b {
			minC.b = c.b
		}
		if c.a < minC.a {
			minC.a = c.a
		}
		if c.r > maxC.r {
			maxC.r = c.r
		}
		if c.g > maxC.g {
			maxC.g = c.g
		}
		if c.b > maxC.b {
			maxC.b = c.b
		}
		if c.a > maxC.a {
			maxC.a = c.a
		}
	}

	return minC, maxC
}
