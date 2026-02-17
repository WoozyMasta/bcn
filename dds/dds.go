// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

// Package dds registers the DDS image format with the standard image package.
package dds

import (
	"image"
	"image/color"
	"io"

	"github.com/woozymasta/bcn"
)

const ddsMagic = "\x44\x53\x44\x20" // DDS in little-endian

// init registers DDS decoder hooks in the standard image package.
func init() {
	image.RegisterFormat("dds", ddsMagic, decode, decodeConfig)
}

// decode reads full DDS payload and returns the top-level image.
func decode(r io.Reader) (image.Image, error) {
	_, img, err := bcn.DecodeDDS(r)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// decodeConfig reads dimensions from DDS header without full payload decode.
func decodeConfig(r io.Reader) (image.Config, error) {
	h, err := bcn.ReadDDSHeader(r)
	if err != nil {
		return image.Config{}, err
	}

	return image.Config{
		Width:      int(h.Width),
		Height:     int(h.Height),
		ColorModel: color.NRGBAModel,
	}, nil
}
