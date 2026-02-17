// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "image"

// encodeFacesWithOptions validates face dimensions and produces encoded face mip payloads.
// It is shared by DDS and KTX container encoders to keep container behavior identical.
func encodeFacesWithOptions(images []image.Image, format Format, opts *EncodeOptions) ([]Face, int, int, error) {
	if len(images) != 1 && len(images) != 6 {
		return nil, 0, 0, ErrExpectedOneOrSixImages
	}

	options := normalizeEncodeOptions(opts)
	width := images[0].Bounds().Dx()
	height := images[0].Bounds().Dy()

	for _, img := range images[1:] {
		if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
			return nil, 0, 0, ErrFacesDifferentDimensions
		}
	}

	faces := make([]Face, len(images))

	for i, img := range images {
		if options.GenerateMipmaps {
			mips := GenerateMipmaps(img, options.UseSRGB)
			mipData := make([][]byte, len(mips))

			for level, mip := range mips {
				data, _, _, err := EncodeImageWithOptions(mip, format, &options)
				if err != nil {
					return nil, 0, 0, err
				}

				mipData[level] = data
			}

			faces[i] = Face{Mipmaps: mipData}
			continue
		}

		data, _, _, err := EncodeImageWithOptions(img, format, &options)
		if err != nil {
			return nil, 0, 0, err
		}

		faces[i] = Face{Mipmaps: [][]byte{data}}
	}

	return faces, width, height, nil
}
