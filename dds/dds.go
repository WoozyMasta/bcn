// Package dds registers the DDS image format with the standard image package.
package dds

import (
	"image"
	"image/color"
	"io"

	"github.com/woozymasta/bcn"
)

const ddsMagic = "\x44\x53\x44\x20" // DDS in little-endian

func init() {
	image.RegisterFormat("dds", ddsMagic, decode, decodeConfig)
}

func decode(r io.Reader) (image.Image, error) {
	_, img, err := bcn.DecodeDDS(r)
	if err != nil {
		return nil, err
	}

	return img, nil
}

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
