// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"runtime"
	"sync"
)

// decodeBlocks is a convenience wrapper that decodes with default options.
func decodeBlocks(data []byte, width, height int, format Format) ([]byte, error) {
	return decodeBlocksWithOptions(data, width, height, format, nil)
}

// decodeBlocksWithOptions decodes compressed or uncompressed payload into tight RGBA.
func decodeBlocksWithOptions(data []byte, width, height int, format Format, opts *DecodeOptions) ([]byte, error) {
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
	totalBlocks := bx * by

	workers := runtime.GOMAXPROCS(0)
	if opts != nil {
		workers = opts.Workers
		if workers == 0 {
			workers = runtime.GOMAXPROCS(0)
		}
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	parallelMinBlocks := 256 * workers
	if totalBlocks >= parallelMinBlocks && workers > 1 {
		// Split block-space into contiguous ranges to keep writes non-overlapping.
		pool := getDecodePool(workers)
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			start := (totalBlocks * w) / workers
			end := (totalBlocks * (w + 1)) / workers
			pool.jobs <- decodeJob{
				start:     start,
				end:       end,
				bx:        bx,
				width:     width,
				height:    height,
				blockSize: blockSize,
				format:    format,
				data:      data,
				out:       out,
				wg:        &wg,
			}
		}
		wg.Wait()
		return out, nil
	}

	pos := 0
	for y := range by {
		for x := range bx {
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
				for i := range 16 {
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

// encodeBlocksWithOptions encodes tight RGBA pixels into the selected BCn format.
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
	totalBlocks := bx * by

	workers := options.Workers
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	parallelMinBlocks := 256 * workers
	if totalBlocks >= parallelMinBlocks && workers > 1 {
		// Encode worker ranges directly into preallocated output block slices.
		pool := getEncodePool(workers)
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			start := (totalBlocks * w) / workers
			end := (totalBlocks * (w + 1)) / workers
			pool.jobs <- encodeJob{
				start:     start,
				end:       end,
				bx:        bx,
				width:     width,
				height:    height,
				blockSize: blockSize,
				format:    format,
				options:   options,
				rgba:      rgba,
				out:       out,
				wg:        &wg,
			}
		}

		wg.Wait()
		return out, nil
	}

	pos := 0

	for y := range by {
		for x := range bx {
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
