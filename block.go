// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// extractBlock reads a 4x4 block from linear RGBA data with edge replication.
func extractBlock(rgba []byte, width, height, bx, by int) [16]rgba8 {
	var block [16]rgba8
	baseX := bx * 4
	baseY := by * 4
	if baseX+3 < width && baseY+3 < height {
		row := (baseY*width + baseX) * 4
		stride := width * 4

		for y := range 4 {
			off := row + y*stride
			for x := range 4 {
				block[y*4+x] = rgbaFromNRGBA(rgba, off+x*4)
			}
		}

		return block
	}

	for y := range 4 {
		py := by*4 + y
		if py >= height {
			py = height - 1
		}

		for x := range 4 {
			px := bx*4 + x
			if px >= width {
				px = width - 1
			}
			off := (py*width + px) * 4
			block[y*4+x] = rgbaFromNRGBA(rgba, off)
		}
	}

	return block
}

// storeBlock writes a decoded 4x4 NRGBA block (4 rows of 16 bytes) back to
// destination RGBA data.
func storeBlock(dst []byte, width, height, bx, by int, block *[64]byte) {
	baseX := bx * 4
	baseY := by * 4
	if baseX+3 < width && baseY+3 < height {
		row := (baseY*width + baseX) * 4
		stride := width * 4
		copy(dst[row:row+16], block[0:16])
		row += stride
		copy(dst[row:row+16], block[16:32])
		row += stride
		copy(dst[row:row+16], block[32:48])
		row += stride
		copy(dst[row:row+16], block[48:64])

		return
	}

	for y := range 4 {
		py := by*4 + y
		if py >= height {
			break
		}

		for x := range 4 {
			px := bx*4 + x
			if px >= width {
				break
			}
			off := (py*width + px) * 4
			i := (y*4 + x) * 4
			copy(dst[off:off+4], block[i:i+4])
		}
	}
}
