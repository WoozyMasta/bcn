package bcn

func decodeBlocks(data []byte, width, height int, format Format) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	blockSize := format.blockSize()
	if blockSize == 0 {
		return nil, ErrUnsupportedFormat
	}
	if !format.isCompressed() {
		expected := width * height * blockSize
		if len(data) < expected {
			return nil, ErrInsufficientData
		}

		switch format {
		case FormatRGBA8:
			out := make([]byte, expected)
			copy(out, data[:expected])
			return out, nil
		case FormatBGRA8:
			out := make([]byte, expected)
			for i := 0; i < expected; i += 4 {
				out[i] = data[i+2]
				out[i+1] = data[i+1]
				out[i+2] = data[i]
				out[i+3] = data[i+3]
			}
			return out, nil
		default:
			return nil, ErrUnsupportedUncompressedFormat
		}
	}

	bx := (width + 3) / 4
	by := (height + 3) / 4
	if len(data) < bx*by*blockSize {
		return nil, ErrInsufficientData
	}

	out := make([]byte, width*height*4)
	pos := 0
	for y := 0; y < by; y++ {
		for x := 0; x < bx; x++ {
			var block [16]rgba8

			switch format {
			case FormatDXT1:
				block = decodeBlockDXT1(data[pos : pos+8])
				pos += 8
			case FormatDXT3:
				block = decodeBlockDXT3(data[pos : pos+16])
				pos += 16
			case FormatDXT5:
				block = decodeBlockDXT5(data[pos : pos+16])
				pos += 16
			case FormatBC4:
				alpha := decodeBlockBC4(data[pos : pos+8])
				pos += 8
				for i := 0; i < 16; i++ {
					block[i] = rgba8{r: alpha[i], g: alpha[i], b: alpha[i], a: 255}
				}
			case FormatBC5:
				block = decodeBlockBC5(data[pos : pos+16])
				pos += 16
			default:
				return nil, ErrUnsupportedFormat
			}

			storeBlock(out, width, height, x, y, block)
		}
	}

	return out, nil
}

func encodeBlocksWithOptions(rgba []byte, width, height int, format Format, opts *EncodeOptions) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	if len(rgba) != width*height*4 {
		return nil, ErrInvalidRGBALength
	}

	blockSize := format.blockSize()
	if blockSize == 0 {
		return nil, ErrUnsupportedFormat
	}

	if !format.isCompressed() {
		switch format {
		case FormatRGBA8:
			out := make([]byte, len(rgba))
			copy(out, rgba)
			return out, nil
		case FormatBGRA8:
			out := make([]byte, len(rgba))
			for i := 0; i < len(rgba); i += 4 {
				out[i] = rgba[i+2]
				out[i+1] = rgba[i+1]
				out[i+2] = rgba[i]
				out[i+3] = rgba[i+3]
			}
			return out, nil
		default:
			return nil, ErrUnsupportedUncompressedFormat
		}
	}

	options := normalizeEncodeOptions(opts)
	bx := (width + 3) / 4
	by := (height + 3) / 4
	out := make([]byte, bx*by*blockSize)
	pos := 0

	for y := 0; y < by; y++ {
		for x := 0; x < bx; x++ {
			block := extractBlock(rgba, width, height, x, y)
			switch format {
			case FormatDXT1:
				b := encodeBlockDXT1WithOptions(block, options)
				copy(out[pos:pos+8], b[:])
				pos += 8
			case FormatDXT3:
				b := encodeBlockDXT3WithOptions(block, options)
				copy(out[pos:pos+16], b[:])
				pos += 16
			case FormatDXT5:
				b := encodeBlockDXT5WithOptions(block, options)
				copy(out[pos:pos+16], b[:])
				pos += 16
			case FormatBC4:
				b := encodeBlockBC4(block, options, func(c rgba8) uint8 { return c.r })
				copy(out[pos:pos+8], b[:])
				pos += 8
			case FormatBC5:
				b := encodeBlockBC5(block, options)
				copy(out[pos:pos+16], b[:])
				pos += 16
			default:
				return nil, ErrUnsupportedFormat
			}
		}
	}

	return out, nil
}
