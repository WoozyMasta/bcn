// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

const (
	// bc7PalR is the byte offset of the 16-entry R palette lane in the params block.
	bc7PalR = 0
	// bc7PalG is the byte offset of the 16-entry G palette lane in the params block.
	bc7PalG = 64
	// bc7PalB is the byte offset of the 16-entry B palette lane in the params block.
	bc7PalB = 128
	// bc7PalA is the byte offset of the 16-entry A palette lane in the params block.
	bc7PalA = 192
)

// bc7Consts holds shared read-only vectors used by BC7 assembly kernels.
type bc7Consts struct {
	// indices maps palette entry number to a broadcastable int32 lane value.
	indices Mem
}

// genBC7Consts emits shared BC7 read-only constants.
func genBC7Consts() bc7Consts {
	indices := GLOBL("bc7IndexConsts", RODATA|NOPTR)
	for i := range 16 {
		DATA(i*4, U32(uint32(i))) // #nosec G115 -- i is in [0,15].
	}

	return bc7Consts{indices: indices}
}

// genBC7Mode6Indices emits the AVX2 nearest-palette kernel for BC7 mode 6.
func genBC7Mode6Indices(c bc7Consts) {
	TEXT("BC7Mode6IndicesAVX2", NOSPLIT, "func(block *[64]byte, params *[64]int32, idx *[16]int32) uint32")
	Pragma("noescape")
	Doc(
		"BC7Mode6IndicesAVX2 assigns 16 RGBA pixels to the nearest mode 6 palette entry.",
		"params stores palette channels as 16 R, 16 G, 16 B, 16 A int32 values.",
	)

	block := Load(Param("block"), GP64())
	params := Load(Param("params"), GP64())
	idxOut := Load(Param("idx"), GP64())

	// Common lane constants. maskFF extracts one byte channel after shifting;
	// zero is kept for cheap vector initialization and tie-preserving index zero.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)
	zero := YMM()
	VPXOR(zero, zero, zero)

	// entryErr returns per-pixel RGBA squared error against one palette entry.
	// All channels are already expanded to int32 lanes, matching bc7SSE exactly.
	entryErr := func(e int, r, g, b, a VecVirtual) VecVirtual {
		err := YMM()
		t := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: bc7PalR + e*4}, t)
		VPSUBD(t, r, err)
		VPMULLD(err, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7PalG + e*4}, t)
		VPSUBD(t, g, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7PalB + e*4}, t)
		VPSUBD(t, b, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7PalA + e*4}, t)
		VPSUBD(t, a, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		return err
	}

	total := YMM()
	VPXOR(total, total, total)

	// processHalf handles eight pixels. The block is 16 packed RGBA pixels,
	// so two calls cover the full 4x4 texel block without a loop counter.
	processHalf := func(disp int, outDisp int) {
		// Load eight RGBA pixels and split packed little-endian bytes into
		// int32 R/G/B/A lanes: bits 0..7, 8..15, 16..23, and 24..31.
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

		// Palette entry 0 seeds the argmin. Later updates use strict
		// less-than masks, so ties keep the lower index like the Go loop.
		best := entryErr(0, r, g, b, a)
		bestIdx := YMM()
		VPXOR(bestIdx, bestIdx, bestIdx)

		// Walk the remaining 15 palette entries, replacing both the current
		// error and the current index only for lanes with a strictly lower SSE.
		for e := 1; e < 16; e++ {
			err := entryErr(e, r, g, b, a)
			mask := YMM()
			VPCMPGTD(err, best, mask)
			VPBLENDVB(mask, err, best, best)

			yv := YMM()
			VPBROADCASTD(c.indices.Offset(e*4), yv)
			VPBLENDVB(mask, yv, bestIdx, bestIdx)
		}

		// Accumulate the winning per-pixel errors and store indices as int32
		// lanes. The Go wrapper narrows those indices to uint8.
		VPADDD(best, total, total)
		VMOVDQU(bestIdx, Mem{Base: idxOut, Disp: outDisp})
	}

	processHalf(0, 0)
	processHalf(32, 32)

	// Return the total block SSE as one uint32, matching the scalar int range
	// for 16 texels * 4 channels * 255^2.
	out := horizontalAddYMM(total)
	Store(out, ReturnIndex(0))
	VZEROUPPER()
	RET()
}
