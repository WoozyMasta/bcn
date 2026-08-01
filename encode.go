// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"encoding/binary"
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

	if format == FormatBC6HU || format == FormatBC6HS {
		return nil, ErrBC6HUsesHDRAPI
	}

	out := make([]byte, width*height*4)
	if err := decodeBlocksInto(out, data, width, height, format, opts); err != nil {
		return nil, err
	}

	return out, nil
}

// decodeBlocksInto decodes data into dst, which must be a tight width*height*4 RGBA buffer.
// It lets callers (e.g. image.NRGBA.Pix) skip a second allocation and copy.
// The decoded bytes are identical to decodeBlocksWithOptions.
func decodeBlocksInto(dst, data []byte, width, height int, format Format, opts *DecodeOptions) error {
	if width <= 0 || height <= 0 {
		return ErrInvalidDimensions
	}

	blockSize := format.blockSize()
	if blockSize == 0 {
		return ErrUnsupportedFormat
	}

	if !format.isCompressed() {
		expected := width * height * blockSize
		if len(data) < expected {
			return ErrInsufficientData
		}

		switch format {
		case FormatRGBA8:
			copy(dst, data[:expected])
			return nil

		case FormatBGRA8:
			for i := 0; i < expected; i += 4 {
				dst[i] = data[i+2]
				dst[i+1] = data[i+1]
				dst[i+2] = data[i]
				dst[i+3] = data[i+3]
			}
			return nil

		case FormatBGRX8:
			for i := 0; i < expected; i += 4 {
				dst[i] = data[i+2]
				dst[i+1] = data[i+1]
				dst[i+2] = data[i]
				dst[i+3] = 255
			}
			return nil

		case FormatR8:
			for i, value := range data[:expected] {
				out := i * 4
				dst[out] = value
				dst[out+1] = value
				dst[out+2] = value
				dst[out+3] = 255
			}
			return nil

		case FormatRG8:
			for i := 0; i < expected; i += 2 {
				out := i * 2
				dst[out] = data[i]
				dst[out+1] = data[i+1]
				dst[out+2] = 0
				dst[out+3] = 255
			}
			return nil

		case FormatRGB10A2:
			for src, outOffset := 0, 0; src < expected; src, outOffset = src+4, outOffset+4 {
				pixel := binary.LittleEndian.Uint32(data[src:])
				dst[outOffset] = unorm10ToU8(uint16(pixel & 0x3ff))
				dst[outOffset+1] = unorm10ToU8(uint16((pixel >> 10) & 0x3ff))
				dst[outOffset+2] = unorm10ToU8(uint16((pixel >> 20) & 0x3ff))
				dst[outOffset+3] = unorm2ToU8(uint8(pixel >> 30))
			}
			return nil

		default:
			return ErrUnsupportedUncompressedFormat
		}
	}

	bx := (width + 3) / 4
	by := (height + 3) / 4
	if len(data) < bx*by*blockSize {
		return ErrInsufficientData
	}

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
				out:    dst,
				wg:     &wg,
			}
		}

		wg.Wait()
		return nil
	}

	return decodeBlockRange(format, data, dst, width, height, bx, 0, totalBlocks)
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
	if format == FormatBC6HU || format == FormatBC6HS {
		return nil, ErrBC6HUsesHDRAPI
	}

	n, err := encodedBlocksSize(rgba, width, height, format)
	if err != nil {
		return nil, err
	}

	out := make([]byte, n)
	if err := encodeBlocksInto(out, rgba, width, height, format, opts); err != nil {
		return nil, err
	}

	return out, nil
}

// encodedBlocksSize validates the inputs
// and returns the encoded byte length for the format
// (block payload size when compressed, len(rgba) when uncompressed).
func encodedBlocksSize(rgba []byte, width, height int, format Format) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, ErrInvalidDimensions
	}
	if len(rgba) != width*height*4 {
		return 0, ErrInvalidRGBALength
	}

	blockSize := format.blockSize()
	if blockSize == 0 {
		return 0, ErrUnsupportedFormat
	}

	if !format.isCompressed() {
		return width * height * blockSize, nil
	}

	return ((width + 3) / 4) * ((height + 3) / 4) * blockSize, nil
}

// encodeBlocksInto encodes rgba into dst,
// which must be at least encodedBlocksSize bytes.
// It lets callers reuse a buffer across encodes
// (the bytes are identical to encodeBlocksWithOptions).
func encodeBlocksInto(dst, rgba []byte, width, height int, format Format, opts *EncodeOptions) error {
	n, err := encodedBlocksSize(rgba, width, height, format)
	if err != nil {
		return err
	}
	if len(dst) < n {
		return ErrBufferTooSmall
	}
	out := dst[:n]

	if !format.isCompressed() {
		switch format {
		case FormatRGBA8:
			copy(out, rgba)
			return nil

		case FormatBGRA8:
			for i := 0; i < len(rgba); i += 4 {
				out[i] = rgba[i+2]
				out[i+1] = rgba[i+1]
				out[i+2] = rgba[i]
				out[i+3] = rgba[i+3]
			}
			return nil

		case FormatBGRX8:
			for i := 0; i < len(rgba); i += 4 {
				out[i] = rgba[i+2]
				out[i+1] = rgba[i+1]
				out[i+2] = rgba[i]
				out[i+3] = 255
			}
			return nil

		case FormatR8:
			for src, dst := 0, 0; src < len(rgba); src, dst = src+4, dst+1 {
				out[dst] = rgba[src]
			}
			return nil

		case FormatRG8:
			for src, dst := 0, 0; src < len(rgba); src, dst = src+4, dst+2 {
				out[dst] = rgba[src]
				out[dst+1] = rgba[src+1]
			}
			return nil

		case FormatRGB10A2:
			for src, dst := 0, 0; src < len(rgba); src, dst = src+4, dst+4 {
				pixel := uint32(u8ToUNORM10(rgba[src])) |
					uint32(u8ToUNORM10(rgba[src+1]))<<10 |
					uint32(u8ToUNORM10(rgba[src+2]))<<20 |
					uint32(u8ToUNORM2(rgba[src+3]))<<30
				binary.LittleEndian.PutUint32(out[dst:], pixel)
			}
			return nil

		default:
			return ErrUnsupportedUncompressedFormat
		}
	}

	options := normalizeEncodeOptions(opts)
	bx := (width + 3) / 4
	by := (height + 3) / 4
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
		return nil
	}

	return encodeBlockRange(format, rgba, out, width, height, bx, 0, totalBlocks, options)
}

// u8ToUNORM10 maps an 8-bit normalized value to 10 bits with nearest rounding.
func u8ToUNORM10(v byte) uint16 {
	return uint16((uint32(v)*1023 + 127) / 255) // #nosec G115 -- result is in [0,1023].
}

// unorm10ToU8 maps a 10-bit normalized value to 8 bits with nearest rounding.
func unorm10ToU8(v uint16) byte {
	return byte((uint32(v&0x3ff)*255 + 511) / 1023) // #nosec G115 -- result is in [0,255].
}

// u8ToUNORM2 maps an 8-bit normalized value to 2 bits with nearest rounding.
func u8ToUNORM2(v byte) uint8 {
	return uint8((uint16(v)*3 + 127) / 255) // #nosec G115 -- result is in [0,3].
}

// unorm2ToU8 maps a 2-bit normalized value to 8 bits with nearest rounding.
func unorm2ToU8(v uint8) byte {
	return byte((uint16(v&3)*255 + 1) / 3) // #nosec G115 -- result is in [0,255].
}
