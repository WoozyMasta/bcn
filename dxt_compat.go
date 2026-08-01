// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// EncodeDXT1 is a compatibility alias for EncodeBC1.
func EncodeDXT1(rgba []byte, width, height int) ([]byte, error) {
	return EncodeBC1(rgba, width, height)
}

// DecodeDXT1 is a compatibility alias for DecodeBC1.
func DecodeDXT1(data []byte, width, height int) ([]byte, error) {
	return DecodeBC1(data, width, height)
}

// DecodeDXT1WithOptions is a compatibility alias for DecodeBC1WithOptions.
func DecodeDXT1WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return DecodeBC1WithOptions(data, width, height, opts)
}

// EncodeDXT1WithOptions is a compatibility alias for EncodeBC1WithOptions.
func EncodeDXT1WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return EncodeBC1WithOptions(rgba, width, height, opts)
}

// EncodeDXT3 is a compatibility alias for EncodeBC2.
func EncodeDXT3(rgba []byte, width, height int) ([]byte, error) {
	return EncodeBC2(rgba, width, height)
}

// DecodeDXT3 is a compatibility alias for DecodeBC2.
func DecodeDXT3(data []byte, width, height int) ([]byte, error) {
	return DecodeBC2(data, width, height)
}

// DecodeDXT3WithOptions is a compatibility alias for DecodeBC2WithOptions.
func DecodeDXT3WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return DecodeBC2WithOptions(data, width, height, opts)
}

// EncodeDXT3WithOptions is a compatibility alias for EncodeBC2WithOptions.
func EncodeDXT3WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return EncodeBC2WithOptions(rgba, width, height, opts)
}

// EncodeDXT5 is a compatibility alias for EncodeBC3.
func EncodeDXT5(rgba []byte, width, height int) ([]byte, error) {
	return EncodeBC3(rgba, width, height)
}

// DecodeDXT5 is a compatibility alias for DecodeBC3.
func DecodeDXT5(data []byte, width, height int) ([]byte, error) {
	return DecodeBC3(data, width, height)
}

// DecodeDXT5WithOptions is a compatibility alias for DecodeBC3WithOptions.
func DecodeDXT5WithOptions(data []byte, width, height int, opts *DecodeOptions) ([]byte, error) {
	return DecodeBC3WithOptions(data, width, height, opts)
}

// EncodeDXT5WithOptions is a compatibility alias for EncodeBC3WithOptions.
func EncodeDXT5WithOptions(rgba []byte, width, height int, opts *EncodeOptions) ([]byte, error) {
	return EncodeBC3WithOptions(rgba, width, height, opts)
}
