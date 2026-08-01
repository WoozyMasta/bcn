// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

const parityDir = "testdata/parity/"

type parityPublicCase struct {
	name      string
	format    Format
	blockSize int
	maxDelta  byte
}

var parityPublicCases = []parityPublicCase{
	{"BC1", FormatDXT1, 8, 1},
	{"BC2", FormatDXT3, 16, 1},
	{"BC3", FormatDXT5, 16, 1},
	{"BC4", FormatBC4, 8, 1},
	{"BC5", FormatBC5, 16, 1},
	{"BC7", FormatBC7, 16, 0},
}

func parityFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(parityDir + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

// parityBytes compares a decoder output to an external reference. Legacy LDR
// formats permit a one-unit component difference from interpolation rounding.
func parityBytes(t *testing.T, format string, block, got, want []byte, index int, maxDelta byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s block %d: len %d, want %d", format, index, len(got), len(want))
	}
	for i := range want {
		delta := byteDelta(got[i], want[i])
		if delta > maxDelta {
			t.Fatalf(
				"%s block %d byte %d: got %#02x, want %#02x (delta %d, max %d); data=%x",
				format, index, i, got[i], want[i], delta, maxDelta, block)
		}
	}
}

func byteDelta(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}

func parityRGBA(t *testing.T, format string, blockSize int, maxDelta byte, decode func([]byte) [64]byte) {
	t.Helper()
	blocks, want := parityFile(t, format+".blocks"), parityFile(t, format+".rgba")
	if len(blocks)%blockSize != 0 || len(want) != len(blocks)/blockSize*64 {
		t.Fatalf("%s: malformed fixture", format)
	}
	for i := 0; i < len(blocks)/blockSize; i++ {
		block := blocks[i*blockSize : (i+1)*blockSize]
		got := decode(block)
		parityBytes(t, format, block, got[:], want[i*64:(i+1)*64], i, maxDelta)
	}
}

func TestDecodeParityFixtures(t *testing.T) {
	t.Run("BC1", func(t *testing.T) { parityRGBA(t, "bc1", 8, 1, decodeBlockDXT1) })
	t.Run("BC2", func(t *testing.T) { parityRGBA(t, "bc2", 16, 1, decodeBlockDXT3) })
	t.Run("BC3", func(t *testing.T) { parityRGBA(t, "bc3", 16, 1, decodeBlockDXT5) })
	t.Run("BC4", parityBC4)
	t.Run("BC5", parityBC5)
	t.Run("BC6H_UF16", func(t *testing.T) { parityBC6H(t, "bc6hu.rgb16le", false) })
	t.Run("BC6H_SF16", func(t *testing.T) { parityBC6H(t, "bc6hs.rgb16le", true) })
	t.Run("BC7", func(t *testing.T) { parityRGBA(t, "bc7", 16, 0, decodeBlockBC7) })
}

func TestDecodeParityPublicAPI(t *testing.T) {
	for _, tc := range parityPublicCases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeParityPublic(t, tc)
			prefix := "bc" + tc.name[2:]
			blocks := parityFile(t, prefix+".blocks")
			want := parityPublicExpected(t, tc.format, prefix, len(blocks)/tc.blockSize)
			parityBytes(t, tc.name, nil, got, want, 0, tc.maxDelta)
		})
	}
}

func TestDecodeParityPublicAPIGolden(t *testing.T) {
	expected := map[string]string{
		"BC1": "b3e4d9549f32b88ff0b00dad06d1f79f91f4bfedc42a51a0cccf96a73febb787",
		"BC2": "62e247a94003562eb1acc3a77698c0a37045de62dcc7d9f8bf00835c18626364",
		"BC3": "a536dca3657ec0eb9f452617b17fd468452e79157e5e89e46d6dc017a4fadf97",
		"BC4": "3af4486abc2fb5f76cf0b1f04042509e7903624d3a3bb0920a7f331d99921ba1",
		"BC5": "10506f5ce2923d2ae7347095785eea143a53c19c2aedb6841f9f0fc183443e91",
		"BC7": "7aa54064f8dab017e054610ce826bb66437059b24f1708ddb0740972aaecc07f",
	}
	for _, tc := range parityPublicCases {
		got := decodeParityPublic(t, tc)
		sum := sha256.Sum256(got)
		actual := fmt.Sprintf("%x", sum)
		if os.Getenv("BCN_GOLDEN_PRINT") != "" {
			t.Logf("%q: %q,", tc.name, actual)
			continue
		}
		if want := expected[tc.name]; actual != want {
			t.Errorf("%s: SHA-256 %s, want %s", tc.name, actual, want)
		}
	}
}

func decodeParityPublic(t *testing.T, tc parityPublicCase) []byte {
	t.Helper()
	prefix := "bc" + tc.name[2:]
	blocks := parityFile(t, prefix+".blocks")
	blockCount := len(blocks) / tc.blockSize
	got, err := decodeBlocksWithOptions(
		blocks, 4, blockCount*4, tc.format, &DecodeOptions{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func parityPublicExpected(t *testing.T, format Format, prefix string, blockCount int) []byte {
	t.Helper()
	switch format {
	case FormatBC4:
		values := parityFile(t, prefix+".r")
		out := make([]byte, blockCount*64)
		for i, value := range values {
			out[i*4] = value
			out[i*4+1] = value
			out[i*4+2] = value
			out[i*4+3] = 255
		}
		return out
	case FormatBC5:
		values := parityFile(t, prefix+".rg")
		out := make([]byte, blockCount*64)
		for i := 0; i < len(values)/2; i++ {
			out[i*4] = values[i*2]
			out[i*4+1] = values[i*2+1]
			out[i*4+3] = 255
		}
		return out
	default:
		return parityFile(t, prefix+".rgba")
	}
}

func parityBC4(t *testing.T) {
	blocks, want := parityFile(t, "bc4.blocks"), parityFile(t, "bc4.r")
	if len(blocks)%8 != 0 || len(want) != len(blocks)/8*16 {
		t.Fatal("BC4: malformed fixture")
	}
	for i := 0; i < len(blocks)/8; i++ {
		block := blocks[i*8 : (i+1)*8]
		got := decodeBlockBC4(block)
		parityBytes(t, "BC4", block, got[:], want[i*16:(i+1)*16], i, 1)
	}
}

func parityBC5(t *testing.T) {
	blocks, want := parityFile(t, "bc5.blocks"), parityFile(t, "bc5.rg")
	if len(blocks)%16 != 0 || len(want) != len(blocks)/16*32 {
		t.Fatal("BC5: malformed fixture")
	}
	for i := 0; i < len(blocks)/16; i++ {
		block := blocks[i*16 : (i+1)*16]
		got := decodeBlockBC5(block)
		for pixel := range 16 {
			parityBytes(t, "BC5", block, got[pixel*4:pixel*4+2], want[i*32+pixel*2:i*32+pixel*2+2], i, 1)
			if got[pixel*4+2] != 0 || got[pixel*4+3] != 255 {
				t.Fatalf("BC5 block %d pixel %d: invalid B/A", i, pixel)
			}
		}
	}
}

func parityBC6H(t *testing.T, fixture string, signed bool) {
	blocks, want := parityFile(t, "bc6h.blocks"), parityFile(t, fixture)
	if len(blocks)%16 != 0 || len(want) != len(blocks)/16*96 {
		t.Fatalf("BC6H: malformed fixture %s", fixture)
	}
	for i := 0; i < len(blocks)/16; i++ {
		block := blocks[i*16 : (i+1)*16]
		got := decodeBlockBC6H(block, signed)
		for component, value := range got {
			expected := binary.LittleEndian.Uint16(want[(i*48+component)*2:])
			if value != expected {
				t.Fatalf(
					"BC6H signed=%v block %d component %d: got %#04x, want %#04x; data=%x",
					signed, i, component, value, expected, block)
			}
		}
	}
}
