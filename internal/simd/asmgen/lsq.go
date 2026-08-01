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
	lsqParamD    = 68
	lsqParamBeta = 72
)

// LSQ color params extend the PackBC1Indices param block:
//
//	17     interpolation denominator d
//	18..21 beta numerator table for palette indices 0..3
func genLSQColorAccumulate() {
	TEXT("LSQColorAccumulateAVX2", NOSPLIT, "func(block *[64]byte, params *[22]int32, out *[9]int32)")
	Pragma("noescape")
	Doc(
		"LSQColorAccumulateAVX2 assigns BC1 palette indices and accumulates",
		"least-squares normal-equation sums for 16 RGBA pixels.",
	)

	block := Load(Param("block"), GP64())
	params := Load(Param("params"), GP64())
	out := Load(Param("out"), GP64())

	// Common lane constants: byte mask for channel extraction
	// and small broadcastable index values used by the argmin and beta lookup stages.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)
	one := YMM()
	VPSRLD(Imm(31), ones, one)
	two := YMM()
	VPSLLD(Imm(1), one, two)
	three := YMM()
	VPADDD(one, two, three)
	zero := YMM()
	VPXOR(zero, zero, zero)

	// Fixed-point RGB weights are shared across all palette-entry distance checks.
	// The scalar path uses the same weighted SSE metric.
	wr := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights}, wr)
	wg := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights + 4}, wg)
	wb := YMM()
	VPBROADCASTD(Mem{Base: params, Disp: ppWeights + 8}, wb)

	// entryErr returns per-lane weighted RGB squared distance to one BC1 palette entry.
	// The caller keeps the per-pixel minimum across entries.
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

	// store reduces eight int32 lanes into one scalar output slot.
	// The first half writes the slot; the second half adds its partial sum.
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

	// Process eight pixels at a time to keep register pressure
	// below the AVX2 architectural limit accepted by Go's assembler (Y0..Y15).
	processHalf := func(disp int, add bool) {
		// Split packed NRGBA pixels into int32 channel lanes.
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
		aChan := YMM()
		VPSRLD(Imm(24), px, aChan)

		// Assign each pixel to the lowest-error palette entry.
		// Ties keep the lower index because updates are driven by strict less-than masks.
		best := entryErr(0, r, g, b)
		idx := YMM()
		VPXOR(idx, idx, idx)
		update := func(err, val VecVirtual) {
			m := YMM()
			VPCMPGTD(err, best, m)
			VPBLENDVB(m, err, best, best)
			VPBLENDVB(m, val, idx, idx)
		}
		update(entryErr(1, r, g, b), one)
		update(entryErr(2, r, g, b), two)
		err3 := entryErr(3, r, g, b)
		// Alpha-mode BC1 excludes entry 3 from the fit;
		// the wrapper passes a large penalty only for that mode, otherwise this is zero.
		pen := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: ppPenalty}, pen)
		VPADDD(pen, err3, err3)
		update(err3, three)

		// Translate palette index -> beta numerator.
		// Index 0 maps to beta 0, so only entries 1..3 need explicit blends.
		beta := YMM()
		VPXOR(beta, beta, beta)
		for e := 1; e < 4; e++ {
			ev := YMM()
			VPBROADCASTD(Mem{Base: params, Disp: lsqParamBeta + e*4}, ev)
			wanted := YMM()
			switch e {
			case 1:
				VMOVDQU(one, wanted)
			case 2:
				VMOVDQU(two, wanted)
			default:
				VMOVDQU(three, wanted)
			}
			cmp := YMM()
			VPCMPEQD(wanted, idx, cmp)
			VPBLENDVB(cmp, ev, beta, beta)
		}

		dv := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: lsqParamD}, dv)
		alpha := YMM()
		VPSUBD(beta, dv, alpha)

		// In BC1 alpha mode, sub-threshold source pixels are holes
		// and do not participate in the least-squares fit.
		// Opaque mode uses threshold 0, which cannot mask any NRGBA alpha byte.
		thr := YMM()
		VPBROADCASTD(Mem{Base: params, Disp: ppThresh}, thr)
		invalid := YMM()
		VPCMPGTD(aChan, thr, invalid)
		VPBLENDVB(invalid, zero, alpha, alpha)
		VPBLENDVB(invalid, zero, beta, beta)

		// Emit the normal-equation sums:
		// {aa, bb, ab, aR, aG, aB, bR, bG, bB}.
		store(0, mul(alpha, alpha), add)
		store(1, mul(beta, beta), add)
		store(2, mul(alpha, beta), add)
		store(3, mul(alpha, r), add)
		store(4, mul(alpha, g), add)
		store(5, mul(alpha, b), add)
		store(6, mul(beta, r), add)
		store(7, mul(beta, g), add)
		store(8, mul(beta, b), add)
	}

	processHalf(0, false)
	processHalf(32, true)

	VZEROUPPER()
	RET()
}

func genLSQAlphaAccumulate(ac Mem) {
	TEXT("LSQAlphaAccumulateAVX2", NOSPLIT, "func(samples *[16]uint8, aa uint32, out *[5]int32)")
	Pragma("noescape")
	Doc(
		"LSQAlphaAccumulateAVX2 assigns BC3/BC4 alpha indices and accumulates",
		"least-squares normal-equation sums for 16 samples.",
	)

	sp := Load(Param("samples"), GP64())
	aa := Load(Param("aa"), GP32())
	out := Load(Param("out"), GP64())
	pp := emitAlphaPaletteScratch(aa, ac)

	// Common lane constants. Alpha LSQ uses denominator 7
	// for the 8-value BC3/BC4 palette emitted by this encoder.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	one := YMM()
	VPSRLD(Imm(31), ones, one)
	zero := YMM()
	VPXOR(zero, zero, zero)
	seven := YMM()
	VPSLLD(Imm(3), one, seven)
	VPSUBD(one, seven, seven)

	// store reduces one half-block's vector sum and writes/adds it to output.
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

	// Process eight samples at a time, mirroring genBestAlphaIndices
	// but keeping the winning indices as lanes for LSQ accumulation.
	processHalf := func(disp int, add bool) {
		samp := YMM()
		VPMOVZXBD(Mem{Base: sp, Disp: disp}, samp)

		// Nearest alpha-palette assignment. Strict less-than preserves
		// the scalar tie-break behavior: the first/lower index wins.
		p0 := YMM()
		VPBROADCASTD(pp, p0)
		best := sqDiff(samp, p0)
		idx := YMM()
		VPXOR(idx, idx, idx)
		cur := YMM()
		VMOVDQU(one, cur)
		for e := 1; e < 8; e++ {
			pv := YMM()
			VPBROADCASTD(pp.Offset(e*4), pv)
			err := sqDiff(samp, pv)
			m := YMM()
			VPCMPGTD(err, best, m)
			VPBLENDVB(m, err, best, best)
			VPBLENDVB(m, cur, idx, idx)
			if e < 7 {
				VPADDD(one, cur, cur)
			}
		}

		// Convert alpha palette index to interpolation beta numerator:
		// {0,1,2,3,4,5,6,7} -> {0,7,1,2,3,4,5,6}.
		beta := YMM()
		VPSUBD(one, idx, beta)
		eq0 := YMM()
		VPCMPEQD(zero, idx, eq0)
		VPBLENDVB(eq0, zero, beta, beta)
		eq1 := YMM()
		VPCMPEQD(one, idx, eq1)
		VPBLENDVB(eq1, seven, beta, beta)

		alpha := YMM()
		VPSUBD(beta, seven, alpha)

		// Emit the normal-equation sums: {aa, bb, ab, aP, bP}.
		store(0, mul(alpha, alpha), add)
		store(1, mul(beta, beta), add)
		store(2, mul(alpha, beta), add)
		store(3, mul(alpha, samp), add)
		store(4, mul(beta, samp), add)
	}

	processHalf(0, false)
	processHalf(8, true)

	VZEROUPPER()
	RET()
}
