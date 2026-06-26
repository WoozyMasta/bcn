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

// BC7 subset-eval param block (*[34]int32 passed from Go):
//
//	0..7   palette R per entry (8 entries, padded with entry 0)
//	8..15  palette G per entry
//	16..23 palette B per entry
//	24..31 beta numerator per entry (the BC7 weight table value)
//	32     interpolation denominator d (always 64)
//	33     target subset id (0..2)
const (
	bc7SubPalR   = 0
	bc7SubPalG   = 32
	bc7SubPalB   = 64
	bc7SubBeta   = 96
	bc7SubD      = 128
	bc7SubSubset = 132
)

// genBC7SubsetEval emits the AVX2 kernel shared by the BC7 partition modes:
// for the texels of one subset it finds the nearest of up to 8 RGB palette entries and
// accumulates both the least-squares normal-equation sums
// and the total nearest-entry error in a single pass.
// Non-subset texels contribute nothing.
// The metric and tie-breaking match the scalar bc7RGBErr argmin.
func genBC7SubsetEval() {
	TEXT("BC7SubsetEvalAVX2", NOSPLIT, "func(block *[64]byte, part *[16]byte, params *[34]int32, out *[10]int32)")
	Pragma("noescape")
	Doc(
		"BC7SubsetEvalAVX2 assigns nearest RGB palette indices for one subset",
		"and accumulates least-squares sums {aa,bb,ab,aR,aG,aB,bR,bG,bB} plus the total error in out[9].",
		"Texels outside the subset (part&3 != subset) are ignored.",
	)

	block := Load(Param("block"), GP64())
	part := Load(Param("part"), GP64())
	params := Load(Param("params"), GP64())
	out := Load(Param("out"), GP64())

	// Lane constants derived without memory loads.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)
	one := YMM()
	VPSRLD(Imm(31), ones, one)
	three := YMM()
	VPSRLD(Imm(30), ones, three)
	zero := YMM()
	VPXOR(zero, zero, zero)

	dv := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: bc7SubD}, dv)
	subv := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: bc7SubSubset}, subv)

	// entryErr returns the per-lane unweighted RGB squared distance to one entry,
	// matching the scalar bc7RGBErr metric (alpha is ignored by these modes).
	entryErr := func(e int, r, g, b VecVirtual) VecVirtual {
		err := YMM()
		t := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: bc7SubPalR + e*4}, t)
		VPSUBD(t, r, err)
		VPMULLD(err, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7SubPalG + e*4}, t)
		VPSUBD(t, g, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7SubPalB + e*4}, t)
		VPSUBD(t, b, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		return err
	}

	// store reduces eight int32 lanes into one scalar output slot.
	store := func(slot int, v VecVirtual, add bool) {
		gp := horizontalAddYMM(v)
		dst := Mem{Base: out, Disp: slot * 4}
		if add {
			ADDL(gp, dst)
			return
		}
		MOVL(gp, dst)
	}
	mul := func(x, y VecVirtual) VecVirtual {
		v := YMM()
		VMOVDQU(x, v)
		VPMULLD(y, v, v)
		return v
	}

	// Process eight texels at a time to stay within Y0..Y15.
	processHalf := func(blockDisp, partDisp int, add bool) {
		px := YMM()
		VMOVDQU(Mem{Base: block, Disp: blockDisp}, px)
		r := YMM()
		VPAND(maskFF, px, r)
		g := YMM()
		VPSRLD(Imm(8), px, g)
		VPAND(maskFF, g, g)
		b := YMM()
		VPSRLD(Imm(16), px, b)
		VPAND(maskFF, b, b)

		// Assign each texel to the lowest-error entry.
		// The running counter cur holds the candidate entry number;
		// strict less-than keeps the lower index on ties, exactly like the scalar loop.
		best := entryErr(0, r, g, b)
		idx := YMM()
		VPXOR(idx, idx, idx)
		cur := YMM()
		VMOVDQU(one, cur)
		for e := 1; e < 8; e++ {
			err := entryErr(e, r, g, b)
			m := YMM()
			VPCMPGTD(err, best, m)
			VPBLENDVB(m, err, best, best)
			VPBLENDVB(m, cur, idx, idx)
			if e < 7 {
				VPADDD(one, cur, cur)
			}
		}

		// Translate the winning index to its beta numerator.
		// Entry 0 maps to beta 0, so only entries 1..7 need an explicit equality blend.
		beta := YMM()
		VPXOR(beta, beta, beta)
		cur2 := YMM()
		VMOVDQU(one, cur2)
		for e := 1; e < 8; e++ {
			ev := YMM()
			VPBROADCASTD(Mem{Base: params, Disp: bc7SubBeta + e*4}, ev)
			cmp := YMM()
			VPCMPEQD(cur2, idx, cmp)
			VPBLENDVB(cmp, ev, beta, beta)
			if e < 7 {
				VPADDD(one, cur2, cur2)
			}
		}
		alpha := YMM()
		VPSUBD(beta, dv, alpha)

		// Keep only texels of the target subset: load the partition bytes,
		// mask to the 2-bit subset id, and zero the weights
		// and error of any texel that does not belong to this subset.
		pm := YMM()
		VPMOVZXBD(Mem{Base: part, Disp: partDisp}, pm)
		VPAND(three, pm, pm)
		valid := YMM()
		VPCMPEQD(subv, pm, valid)
		VPBLENDVB(valid, alpha, zero, alpha)
		VPBLENDVB(valid, beta, zero, beta)
		errMasked := YMM()
		VPBLENDVB(valid, best, zero, errMasked)

		store(0, mul(alpha, alpha), add)
		store(1, mul(beta, beta), add)
		store(2, mul(alpha, beta), add)
		store(3, mul(alpha, r), add)
		store(4, mul(alpha, g), add)
		store(5, mul(alpha, b), add)
		store(6, mul(beta, r), add)
		store(7, mul(beta, g), add)
		store(8, mul(beta, b), add)
		store(9, errMasked, add)
	}

	processHalf(0, 0, false)
	processHalf(32, 8, true)

	VZEROUPPER()
	RET()
}

// BC7 mode 7 subset-eval param block (*[22]int32 passed from Go):
//
//	0..3   palette R per entry (4 entries)
//	4..7   palette G per entry
//	8..11  palette B per entry
//	12..15 palette A per entry
//	16..19 beta numerator per entry (the BC7 weight2 table)
//	20     interpolation denominator d (always 64)
//	21     target subset id (0 or 1)
const (
	bc7M7PalR   = 0
	bc7M7PalG   = 16
	bc7M7PalB   = 32
	bc7M7PalA   = 48
	bc7M7Beta   = 64
	bc7M7D      = 80
	bc7M7Subset = 84
)

// genBC7Mode7SubsetEval emits the RGBA counterpart of BC7SubsetEvalAVX2 for two-subset mode 7:
// for the texels of one subset it finds the nearest of 4 RGBA palette entries
// and accumulates the full RGBA least-squares sums plus the total error in one pass.
// The metric and tie-breaking match the scalar bc7SSE.
func genBC7Mode7SubsetEval() {
	TEXT("BC7Mode7SubsetEvalAVX2", NOSPLIT, "func(block *[64]byte, part *[16]byte, params *[22]int32, out *[12]int32)")
	Pragma("noescape")
	Doc(
		"BC7Mode7SubsetEvalAVX2 assigns nearest RGBA palette indices for one subset",
		"and accumulates least-squares sums {aa,bb,ab,aR,aG,aB,aA,bR,bG,bB,bA} plus the total error in out[11].",
		"Texels outside the subset are ignored.",
	)

	block := Load(Param("block"), GP64())
	part := Load(Param("part"), GP64())
	params := Load(Param("params"), GP64())
	out := Load(Param("out"), GP64())

	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)
	one := YMM()
	VPSRLD(Imm(31), ones, one)
	three := YMM()
	VPSRLD(Imm(30), ones, three)
	zero := YMM()
	VPXOR(zero, zero, zero)

	dv := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: bc7M7D}, dv)
	subv := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: bc7M7Subset}, subv)

	// entryErr returns the per-lane RGBA squared distance to one entry,
	// matching the scalar bc7SSE metric used by mode 7.
	entryErr := func(e int, r, g, b, a VecVirtual) VecVirtual {
		err := YMM()
		t := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: bc7M7PalR + e*4}, t)
		VPSUBD(t, r, err)
		VPMULLD(err, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7M7PalG + e*4}, t)
		VPSUBD(t, g, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7M7PalB + e*4}, t)
		VPSUBD(t, b, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		VPBROADCASTD(Mem{Base: params, Disp: bc7M7PalA + e*4}, t)
		VPSUBD(t, a, t)
		VPMULLD(t, t, t)
		VPADDD(t, err, err)
		return err
	}

	store := func(slot int, v VecVirtual, add bool) {
		gp := horizontalAddYMM(v)
		dst := Mem{Base: out, Disp: slot * 4}
		if add {
			ADDL(gp, dst)
			return
		}
		MOVL(gp, dst)
	}
	mul := func(x, y VecVirtual) VecVirtual {
		v := YMM()
		VMOVDQU(x, v)
		VPMULLD(y, v, v)
		return v
	}

	processHalf := func(blockDisp, partDisp int, add bool) {
		px := YMM()
		VMOVDQU(Mem{Base: block, Disp: blockDisp}, px)
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

		// Nearest of the 4 RGBA entries; strict less-than keeps the lower index.
		best := entryErr(0, r, g, b, a)
		idx := YMM()
		VPXOR(idx, idx, idx)
		cur := YMM()
		VMOVDQU(one, cur)
		for e := 1; e < 4; e++ {
			err := entryErr(e, r, g, b, a)
			m := YMM()
			VPCMPGTD(err, best, m)
			VPBLENDVB(m, err, best, best)
			VPBLENDVB(m, cur, idx, idx)
			if e < 3 {
				VPADDD(one, cur, cur)
			}
		}

		// Index -> beta numerator (entry 0 maps to beta 0).
		beta := YMM()
		VPXOR(beta, beta, beta)
		cur2 := YMM()
		VMOVDQU(one, cur2)
		for e := 1; e < 4; e++ {
			ev := YMM()
			VPBROADCASTD(Mem{Base: params, Disp: bc7M7Beta + e*4}, ev)
			cmp := YMM()
			VPCMPEQD(cur2, idx, cmp)
			VPBLENDVB(cmp, ev, beta, beta)
			if e < 3 {
				VPADDD(one, cur2, cur2)
			}
		}
		alpha := YMM()
		VPSUBD(beta, dv, alpha)

		// Restrict to the target subset.
		pm := YMM()
		VPMOVZXBD(Mem{Base: part, Disp: partDisp}, pm)
		VPAND(three, pm, pm)
		valid := YMM()
		VPCMPEQD(subv, pm, valid)
		VPBLENDVB(valid, alpha, zero, alpha)
		VPBLENDVB(valid, beta, zero, beta)
		errMasked := YMM()
		VPBLENDVB(valid, best, zero, errMasked)

		store(0, mul(alpha, alpha), add)
		store(1, mul(beta, beta), add)
		store(2, mul(alpha, beta), add)
		store(3, mul(alpha, r), add)
		store(4, mul(alpha, g), add)
		store(5, mul(alpha, b), add)
		store(6, mul(alpha, a), add)
		store(7, mul(beta, r), add)
		store(8, mul(beta, g), add)
		store(9, mul(beta, b), add)
		store(10, mul(beta, a), add)
		store(11, errMasked, add)
	}

	processHalf(0, 0, false)
	processHalf(32, 8, true)

	VZEROUPPER()
	RET()
}
