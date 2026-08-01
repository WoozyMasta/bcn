// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDDSDX10BCSRGBRead(t *testing.T) {
	tests := []struct {
		name   string
		dxgi   uint32
		format Format
		size   int
	}{
		{"BC1", 72, FormatBC1, 8},
		{"BC2", 75, FormatBC2, 16},
		{"BC3", 78, FormatBC3, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			for _, value := range []any{
				uint32(DDSMagic),
				&DDSHeader{
					Size:              DDSHeaderSize,
					Flags:             DDSFlagCaps | DDSFlagHeight | DDSFlagWidth | DDSFlagLinearSize | DDSFlagPixelFormat,
					Width:             4,
					Height:            4,
					PitchOrLinearSize: uint32(tt.size),
					Caps:              DDSCapsTexture,
					PixelFormat: DDSPixelFormat{
						Size:   DDSPixelFormatSize,
						Flags:  DDSPFFourCC,
						FourCC: DDSFourCCDX10,
					},
				},
				&DDSHeaderDX10{DXGIFormat: tt.dxgi, ResourceDimension: 3, ArraySize: 1},
			} {
				if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
					t.Fatal(err)
				}
			}
			buf.Write(make([]byte, tt.size))

			d, err := ReadDDS(&buf)
			if err != nil {
				t.Fatalf("ReadDDS: %v", err)
			}
			if d.Format != tt.format {
				t.Fatalf("format = %s, want %s", d.Format, tt.format)
			}
		})
	}
}
