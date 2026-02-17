// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "math"

// u32 converts int to uint32 and panics on overflow to keep callers strict.
func u32(v int) uint32 {
	if v < 0 || v > math.MaxUint32 {
		panic("int out of uint32 range")
	}
	// #nosec G115 -- bounds checked above.
	return uint32(v)
}

// u32len is a semantic alias for converting non-negative lengths to uint32.
func u32len(v int) uint32 {
	return u32(v)
}
