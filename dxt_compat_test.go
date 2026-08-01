// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "testing"

func TestDXTCompatibilityAliases(t *testing.T) {
	if FormatDXT1 != FormatBC1 || FormatDXT3 != FormatBC2 || FormatDXT5 != FormatBC3 {
		t.Fatal("DXT format aliases do not match BC formats")
	}

	var (
		_ func([]byte, int, int) ([]byte, error)                 = EncodeDXT1
		_ func([]byte, int, int) ([]byte, error)                 = DecodeDXT1
		_ func([]byte, int, int, *EncodeOptions) ([]byte, error) = EncodeDXT1WithOptions
		_ func([]byte, int, int, *DecodeOptions) ([]byte, error) = DecodeDXT1WithOptions
		_ func([]byte, int, int) ([]byte, error)                 = EncodeDXT3
		_ func([]byte, int, int) ([]byte, error)                 = DecodeDXT3
		_ func([]byte, int, int, *EncodeOptions) ([]byte, error) = EncodeDXT3WithOptions
		_ func([]byte, int, int, *DecodeOptions) ([]byte, error) = DecodeDXT3WithOptions
		_ func([]byte, int, int) ([]byte, error)                 = EncodeDXT5
		_ func([]byte, int, int) ([]byte, error)                 = DecodeDXT5
		_ func([]byte, int, int, *EncodeOptions) ([]byte, error) = EncodeDXT5WithOptions
		_ func([]byte, int, int, *DecodeOptions) ([]byte, error) = DecodeDXT5WithOptions
	)

	if KTXGLCompressedRGBS3TCDXT1 != KTXGLCompressedRGBS3TCBC1 ||
		KTXGLCompressedRGBAS3TCDXT1 != KTXGLCompressedRGBAS3TCBC1 ||
		KTXGLCompressedRGBAS3TCDXT3 != KTXGLCompressedRGBAS3TCBC2 ||
		KTXGLCompressedRGBAS3TCDXT5 != KTXGLCompressedRGBAS3TCBC3 {
		t.Fatal("KTX DXT aliases do not match BC constants")
	}
}
