package bcn

func findMinMax(block [16]rgba8) (rgba8, rgba8) {
	minC := rgba8{255, 255, 255, 255}
	maxC := rgba8{0, 0, 0, 0}
	for i := 0; i < 16; i++ {
		c := block[i]
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
		idx := 0
		row := (baseY*width + baseX) * 4
		stride := width * 4
		for y := 0; y < 4; y++ {
			off := row
			for x := 0; x < 4; x++ {
				block[idx] = rgbaFromNRGBA(rgba, off)
				idx++
				off += 4
			}
			row += stride
		}
		return block
	}
	idx := 0

	for y := 0; y < 4; y++ {
		py := by*4 + y
		if py >= height {
			py = height - 1
		}

		for x := 0; x < 4; x++ {
			px := bx*4 + x
			if px >= width {
				px = width - 1
			}
			off := (py*width + px) * 4
			block[idx] = rgbaFromNRGBA(rgba, off)
			idx++
		}
	}

	return block
}

func storeBlock(dst []byte, width, height, bx, by int, block [16]rgba8) {
	baseX := bx * 4
	baseY := by * 4
	if baseX+3 < width && baseY+3 < height {
		idx := 0
		row := (baseY*width + baseX) * 4
		stride := width * 4
		for y := 0; y < 4; y++ {
			off := row
			for x := 0; x < 4; x++ {
				rgbaToNRGBA(dst, off, block[idx])
				idx++
				off += 4
			}
			row += stride
		}
		return
	}
	idx := 0
	for y := 0; y < 4; y++ {
		py := by*4 + y
		for x := 0; x < 4; x++ {
			px := bx*4 + x
			if py < height && px < width {
				off := (py*width + px) * 4
				rgbaToNRGBA(dst, off, block[idx])
			}
			idx++
		}
	}
}
