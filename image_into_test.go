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

	for _, format := range []Format{FormatBC1, FormatBC3} {
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

	buf, _, _, err := EncodeImageInto(nil, big, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCap := cap(buf)

	got, _, _, err := EncodeImageInto(buf, small, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(got) != wantCap {
		t.Fatalf("expected buffer reuse, cap %d != %d", cap(got), wantCap)
	}

	want, _, _, err := EncodeImageWithOptions(small, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("reused encode output differs from EncodeImageWithOptions")
	}
}

func TestDecodeImageIntoMatchesAndReuses(t *testing.T) {
	img := intoGradient(32, 24)
	data, w, h, err := EncodeImageWithOptions(img, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}

	want, err := DecodeImageWithOptions(data, w, h, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeImageInto(nil, data, w, h, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("DecodeImageInto output differs from DecodeImageWithOptions")
	}

	// Reuse the larger image's Pix for a smaller decode: no reallocation expected.
	small := intoGradient(8, 8)
	sdata, sw, sh, err := EncodeImageWithOptions(small, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCap := cap(got.Pix)

	got2, err := DecodeImageInto(got, sdata, sw, sh, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(got2.Pix) != wantCap {
		t.Fatalf("expected Pix reuse, cap %d != %d", cap(got2.Pix), wantCap)
	}

	swant, err := DecodeImageWithOptions(sdata, sw, sh, FormatBC3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2.Pix, swant.Pix) {
		t.Fatal("reused decode output differs from DecodeImageWithOptions")
	}
}

func TestGenerateMipmapsIntoMatches(t *testing.T) {
	// Non-power-of-two so the chain exercises edge replication.
	img := intoGradient(40, 24)
	want := GenerateMipmapsN(img, 0, false)
	got := GenerateMipmapsInto(nil, img, 0, false)

	if len(got) != len(want) {
		t.Fatalf("len %d != %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Rect != want[i].Rect {
			t.Fatalf("level %d rect %v != %v", i, got[i].Rect, want[i].Rect)
		}
		if !bytes.Equal(got[i].Pix, want[i].Pix) {
			t.Fatalf("level %d pixels differ", i)
		}
	}
}

func TestGenerateMipmapsIntoReuses(t *testing.T) {
	big := intoGradient(64, 64)
	pool := GenerateMipmapsInto(nil, big, 0, false)
	if len(pool) < 2 {
		t.Fatalf("expected multiple mips, got %d", len(pool))
	}
	level1 := pool[1]
	level1Cap := cap(level1.Pix)

	// Reuse for a smaller image: the level-1 struct and Pix buffer must be reused.
	small := intoGradient(32, 32)
	got := GenerateMipmapsInto(pool, small, 0, false)
	if got[1] != level1 {
		t.Fatal("expected level-1 NRGBA struct to be reused")
	}
	if cap(got[1].Pix) != level1Cap {
		t.Fatalf("expected Pix reuse, cap %d != %d", cap(got[1].Pix), level1Cap)
	}

	want := GenerateMipmapsN(small, 0, false)
	if len(got) != len(want) {
		t.Fatalf("len %d != %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i].Pix, want[i].Pix) {
			t.Fatalf("reused level %d pixels differ", i)
		}
	}
}
