// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn_test

import (
	"bytes"
	"fmt"
	"image"

	"github.com/woozymasta/bcn"
)

func ExampleEncodeImage() {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))

	blocks, width, height, err := bcn.EncodeImage(img, bcn.FormatBC1)
	if err != nil {
		panic(err)
	}
	decoded, err := bcn.DecodeImage(blocks, width, height, bcn.FormatBC1)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(blocks), decoded.Bounds())
	// Output:
	// 8 (0,0)-(4,4)
}

func ExampleEncodeDDSWithOptions() {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	dds, err := bcn.EncodeDDSWithOptions([]image.Image{img}, bcn.FormatBC3, &bcn.EncodeOptions{
		GenerateMipmaps: true,
	})
	if err != nil {
		panic(err)
	}

	var dst bytes.Buffer
	if err := dds.Write(&dst); err != nil {
		panic(err)
	}

	fmt.Println(dds.Format, len(dds.Faces), len(dds.Faces[0].Mipmaps), dst.Len() > 0)
	// Output:
	// BC3 1 3 true
}

func ExampleEncodeBC6H() {
	const width, height = 4, 4

	// 0x3c00 is float16 1.0.
	src := make([]uint16, width*height*3)
	for i := range src {
		src[i] = 0x3c00
	}

	blocks, err := bcn.EncodeBC6H(src, width, height, false)
	if err != nil {
		panic(err)
	}
	decoded, err := bcn.DecodeBC6H(blocks, width, height, false)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(blocks), len(decoded))
	// Output:
	// 16 48
}
