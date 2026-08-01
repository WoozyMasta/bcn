// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

// packParams layout (*[20]int32 passed from Go):
//
//	0..3   palette R per entry    12  weight R      15  alpha threshold (0 = opaque)
//	4..7   palette G per entry    13  weight G      16  entry-3 penalty (1<<30 in alpha mode)
//	8..11  palette B per entry    14  weight B      17..19 reserved
const (
	ppPalR    = 0
	ppPalG    = 16
	ppPalB    = 32
	ppWeights = 48
	ppThresh  = 60
	ppPenalty = 64
)

// genPackBC1Indices emits the AVX2 kernel assigning the best palette index
// to each of 16 pixels under the fixed-point weighted SSE metric and packing
// the 2-bit indices into a uint32 (same bit layout as the Go encoder).
func genPackBC1Indices(c decodeConsts) {
	TEXT("PackBC1IndicesAVX2", NOSPLIT, "func(block *[64]byte, params *[20]int32) uint32")
	Pragma("noescape")
	Doc(
		"PackBC1IndicesAVX2 maps 16 NRGBA pixels to weighted-SSE-nearest palette entries",
		"and packs the 2-bit indices. Alpha mode is driven by the params block:",
		"a sub-threshold pixel is forced to index 3 and the entry-3 penalty keeps that entry from winning the argmin.",
		"Ties keep the lowest index.",
	)

	block := Load(Param("block"), GP64())
	params := Load(Param("params"), GP64())

	// Lane constants derived without memory loads.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)
	one := YMM()
	VPSRLD(Imm(31), ones, one)
	three := YMM()
	VPSRLD(Imm(30), ones, three)

	wr := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights}, wr)
	wg := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights + 4}, wg)
	wb := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights + 8}, wb)

	// entryErr computes the weighted SSE of all 8 lanes against palette entry e.
	entryErr := func(e int, r, g, b VecVirtual) VecVirtual {
		err := YMM()
		t := YMM()

		VPBROADCASTD(Mem{Base: params, Disp: ppPalR + e*4}, t)
		VPSUBD(t, r, err)
		VPMULLD(err, err, err)
		VPMULLD(wr, err, err)

		VPBROADCASTD(Mem{Base: params, Disp: ppPalG + e*4}, t)
		VPSUBD(t, g, t)
		VPMULLD(t, t, t)
		VPMULLD(wg, t, t)
		VPADDD(t, err, err)

		VPBROADCASTD(Mem{Base: params, Disp: ppPalB + e*4}, t)
		VPSUBD(t, b, t)
		VPMULLD(t, t, t)
		VPMULLD(wb, t, t)
		VPADDD(t, err, err)

		return err
	}

	// half processes 8 pixels and returns their shifted 2-bit indices OR-able
	// into the final word.
	half := func(disp int, shifts Mem) VecVirtual {
		px := YMM()
		VMOVDQU(Mem{Base: block, Disp: disp}, px)
		r := YMM()
		VPAND(maskFF, px, r)
		g := YMM()
		VPSRLD(Imm(8), px, g)
		VPAND(maskFF, g, g)
		b := YMM()
		VPSRLD(Imm(16), px, b)
		VPAND(maskFF, b, b)
		a := YMM()
		VPSRLD(Imm(24), px, a)

		bestErr := entryErr(0, r, g, b)
		bestIdx := YMM()
		VPXOR(bestIdx, bestIdx, bestIdx)

		update := func(err, idx VecVirtual) {
			m := YMM()
			VPCMPGTD(err, bestErr, m)
			VPBLENDVB(m, err, bestErr, bestErr)
			VPBLENDVB(m, idx, bestIdx, bestIdx)
		}

		update(entryErr(1, r, g, b), one)
		two := YMM()
		VPSLLD(Imm(1), one, two)
		update(entryErr(2, r, g, b), two)

		err3 := entryErr(3, r, g, b)
		pen := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: ppPenalty}, pen)
		VPADDD(pen, err3, err3)
		update(err3, three)

		// Sub-threshold alpha forces index 3 (threshold 0 never triggers).
		thr := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: ppThresh}, thr)
		VPCMPGTD(a, thr, thr)
		VPBLENDVB(thr, three, bestIdx, bestIdx)

		VPSLLVD(shifts, bestIdx, bestIdx)
		return bestIdx
	}

	lo := half(0, c.shiftsLo)
	hi := half(32, c.shiftsHi)
	VPOR(hi, lo, lo)

	// Horizontal OR of the 8 disjoint lane fields.
	x := lo.AsX()
	t := XMM()
	VEXTRACTI128(Imm(1), lo, t)
	VPOR(t, x, x)
	VPSHUFD(Imm(0x4E), x, t)
	VPOR(t, x, x)
	VPSHUFD(Imm(0xE5), x, t)
	VPOR(t, x, x)

	out := GP32()
	VMOVD(x, out)
	VZEROUPPER()
	Store(out, ReturnIndex(0))
	RET()
}
