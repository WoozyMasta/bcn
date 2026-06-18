// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// intoGradient builds a small deterministic NRGBA test image.
func intoGradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 7),
				G: uint8(y * 5),
				B: uint8((x + y) * 3),
				A: uint8(x ^ y),
			})
		}
	}
	return img
}

func TestEncodeImageIntoMatches(t *testing.T) {
	// 18x14 is not a multiple of 4, exercising edge-block replication.
	img := intoGradient(18, 14)
	opts := &EncodeOptions{QualityLevel: QualityLevelFast}

	for _, format := range []Format{FormatDXT1, FormatDXT5} {
		want, w, h, err := EncodeImageWithOptions(img, format, opts)
		if err != nil {
			t.Fatalf("EncodeImageWithOptions(%v): %v", format, err)
		}

		got, gw, gh, err := EncodeImageInto(nil, img, format, opts)
		if err != nil {
			t.Fatalf("EncodeImageInto(%v): %v", format, err)
		}
		if gw != w || gh != h {
			t.Fatalf("dims %dx%d != %dx%d", gw, gh, w, h)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("format %v: EncodeImageInto output differs from EncodeImageWithOptions", format)
		}
	}
}

func TestEncodeImageIntoReusesBuffer(t *testing.T) {
	big := intoGradient(64, 64)
	small := intoGradient(16, 16)

	buf, _, _, err := EncodeImageInto(nil, big, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCap := cap(buf)

	got, _, _, err := EncodeImageInto(buf, small, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(got) != wantCap {
		t.Fatalf("expected buffer reuse, cap %d != %d", cap(got), wantCap)
	}

	want, _, _, err := EncodeImageWithOptions(small, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("reused encode output differs from EncodeImageWithOptions")
	}
}

func TestDecodeImageIntoMatchesAndReuses(t *testing.T) {
	img := intoGradient(32, 24)
	data, w, h, err := EncodeImageWithOptions(img, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}

	want, err := DecodeImageWithOptions(data, w, h, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeImageInto(nil, data, w, h, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("DecodeImageInto output differs from DecodeImageWithOptions")
	}

	// Reuse the larger image's Pix for a smaller decode: no reallocation expected.
	small := intoGradient(8, 8)
	sdata, sw, sh, err := EncodeImageWithOptions(small, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCap := cap(got.Pix)

	got2, err := DecodeImageInto(got, sdata, sw, sh, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(got2.Pix) != wantCap {
		t.Fatalf("expected Pix reuse, cap %d != %d", cap(got2.Pix), wantCap)
	}

	swant, err := DecodeImageWithOptions(sdata, sw, sh, FormatDXT5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2.Pix, swant.Pix) {
		t.Fatal("reused decode output differs from DecodeImageWithOptions")
	}
}
