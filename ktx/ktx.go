// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

// Package ktx registers the KTX image format with the standard image package.
package ktx

import (
	"image"
	"image/color"
	"io"

	"github.com/woozymasta/bcn"
)

// init registers KTX decoder hooks in the standard image package.
func init() {
	image.RegisterFormat("ktx", string(bcn.KTXIdentifier[:]), decode, decodeConfig)
}

// decode reads full KTX payload and returns the top-level image.
func decode(r io.Reader) (image.Image, error) {
	_, img, err := bcn.DecodeKTX(r)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// decodeConfig reads dimensions from KTX header without full payload decode.
func decodeConfig(r io.Reader) (image.Config, error) {
	h, err := bcn.ReadKTXHeader(r)
	if err != nil {
		return image.Config{}, err
	}

	return image.Config{
		Width:      int(h.PixelWidth),
		Height:     int(h.PixelHeight),
		ColorModel: color.NRGBAModel,
	}, nil
}
