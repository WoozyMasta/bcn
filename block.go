// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

func findMinMax(block [16]rgba8) (rgba8, rgba8) {
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

func storeBlock(dst []byte, width, height, bx, by int, block [16]rgba8) {
	baseX := bx * 4
	baseY := by * 4
	if baseX+3 < width && baseY+3 < height {
		row := (baseY*width + baseX) * 4
		stride := width * 4

		for y := range 4 {
			off := row + y*stride
			for x := range 4 {
				rgbaToNRGBA(dst, off+x*4, block[y*4+x])
			}
		}

		return
	}

	for y := range 4 {
		py := by*4 + y
		for x := range 4 {
			px := bx*4 + x
			if py < height && px < width {
				off := (py*width + px) * 4
				rgbaToNRGBA(dst, off, block[y*4+x])
			}
		}
	}
}
