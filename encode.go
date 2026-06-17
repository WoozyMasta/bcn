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
				start:  start,
				end:    end,
				bx:     bx,
				width:  width,
				height: height,
				format: format,
				data:   data,
				out:    out,
				wg:     &wg,
			}
		}

		wg.Wait()
		return out, nil
	}

	if err := decodeBlockRange(format, data, out, width, height, bx, 0, totalBlocks); err != nil {
		return nil, err
	}

	return out, nil
}

// expandBC4Block replicates 16 scalar samples into NRGBA gray pixels.
func expandBC4Block(block *[64]byte, alpha *[16]uint8) {
	for i := range 16 {
		block[i*4+0] = alpha[i]
		block[i*4+1] = alpha[i]
		block[i*4+2] = alpha[i]
		block[i*4+3] = 255
	}
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
				start:   start,
				end:     end,
				bx:      bx,
				width:   width,
				height:  height,
				format:  format,
				options: options,
				rgba:    rgba,
				out:     out,
				wg:      &wg,
			}
		}

		wg.Wait()
		return out, nil
	}

	if err := encodeBlockRange(format, rgba, out, width, height, bx, 0, totalBlocks, options); err != nil {
		return nil, err
	}

	return out, nil
}
