// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

// genScoreDXT1Palette emits the AVX2 kernel computing the total weighted block
// error for one candidate BC1 endpoint pair: the sum over 16 pixels of the
// minimum weighted RGB SSE across the 4 palette entries (limit 4, no alpha
// skip). This matches the scalar opaque-mode dxt1BlockError, and the kernel
// evaluates all pixels exactly (no cutoff): callers compare totals with a
// strict <, so dropping the cutoff cannot change which candidate wins.
//
// The block is the 16 NRGBA pixels ([64]byte). cc packs c0 (low 16) and c1
// (high 16). weights points to {wr, wg, wb, _} int32. Returns the total
// (always < 16*66.6M < 2^31, fits uint32).
func genScoreDXT1Palette() {
	TEXT("ScoreDXT1PaletteAVX2", NOSPLIT, "func(block *[64]byte, cc uint32, weights *[4]int32) uint32")
	Pragma("noescape")
	Doc(
		"ScoreDXT1PaletteAVX2 returns the total weighted block error of one BC1",
		"endpoint pair (cc = c0 | c1<<16) over 16 pixels, palette built like the",
		"Go encoder. Used to drive endpoint refinement.",
	)

	block := Load(Param("block"), GP64())
	cc := Load(Param("cc"), GP32())
	wptr := Load(Param("weights"), GP64())

	c0 := GP32()
	MOVL(cc, c0)
	ANDL(U32(0xFFFF), c0)
	c1 := GP32()
	MOVL(cc, c1)
	SHRL(Imm(16), c1)

	// Extract block channels into resident YMM groups (pixels 0-7 and 8-15).
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	maskFF := YMM()
	VPSRLD(Imm(24), ones, maskFF)

	pxLo := YMM()
	VMOVDQU(Mem{Base: block}, pxLo)
	pxHi := YMM()
	VMOVDQU(Mem{Base: block, Disp: 32}, pxHi)

	rLo, gLo, bLo := splitChannels(pxLo, maskFF)
	rHi, gHi, bHi := splitChannels(pxHi, maskFF)

	wr := YMM()
	VPBROADCASTD(Mem{Base: wptr}, wr)
	wg := YMM()
	VPBROADCASTD(Mem{Base: wptr, Disp: 4}, wg)
	wb := YMM()
	VPBROADCASTD(Mem{Base: wptr, Disp: 8}, wb)

	runLo := YMM()
	runHi := YMM()

	bcast := func(g GPVirtual) VecVirtual {
		x := XMM()
		VMOVD(g, x)
		y := YMM()
		VPBROADCASTD(x, y)
		return y
	}

	// scoreEntry accumulates weighted SSE of all 16 pixels against one palette
	// entry, then folds it into the running per-pixel minimum.
	scoreEntry := func(pr, pg, pb GPVirtual, first bool) {
		chanErr := func(blk VecVirtual, pv, weight VecVirtual) VecVirtual {
			d := YMM()
			VPSUBD(pv, blk, d)
			VPMULLD(d, d, d)
			VPMULLD(weight, d, d)
			return d
		}

		prv := bcast(pr)
		eLo := chanErr(rLo, prv, wr)
		eHi := chanErr(rHi, prv, wr)

		pgv := bcast(pg)
		VPADDD(chanErr(gLo, pgv, wg), eLo, eLo)
		VPADDD(chanErr(gHi, pgv, wg), eHi, eHi)

		pbv := bcast(pb)
		VPADDD(chanErr(bLo, pbv, wb), eLo, eLo)
		VPADDD(chanErr(bHi, pbv, wb), eHi, eHi)

		if first {
			VMOVDQU(eLo, runLo)
			VMOVDQU(eHi, runHi)
			return
		}
		VPMINSD(eLo, runLo, runLo)
		VPMINSD(eHi, runHi, runHi)
	}

	r0, g0, b0 := expand565RGB(c0)
	r1, g1, b1 := expand565RGB(c1)
	scoreEntry(r0, g0, b0, true)
	scoreEntry(r1, g1, b1, false)

	CMPL(c0, c1)
	JHI(LabelRef("mode4"))

	// 3-color mode: entry 2 = (p0+p1)/2, entry 3 = black (origin).
	scoreEntry(avgChan(r0, r1), avgChan(g0, g1), avgChan(b0, b1), false)
	zero := GP32()
	XORL(zero, zero)
	scoreEntry(zero, zero, zero, false)
	JMP(LabelRef("reduce"))

	Label("mode4")
	scoreEntry(interp3(r0, r1), interp3(g0, g1), interp3(b0, b1), false)
	scoreEntry(interp3(r1, r0), interp3(g1, g0), interp3(b1, b0), false)

	Label("reduce")
	sum := YMM()
	VPADDD(runHi, runLo, sum)
	x := sum.AsX()
	hi := XMM()
	VEXTRACTI128(Imm(1), sum, hi)
	VPADDD(hi, x, x)
	t := XMM()
	VPSHUFD(Imm(0x4E), x, t)
	VPADDD(t, x, x)
	VPSHUFD(Imm(0xE5), x, t)
	VPADDD(t, x, x)

	out := GP32()
	VMOVD(x, out)
	VZEROUPPER()
	Store(out, ReturnIndex(0))
	RET()
}

// splitChannels extracts R, G, B int32 lanes from 8 packed NRGBA pixels.
func splitChannels(px, maskFF VecVirtual) (r, g, b VecVirtual) {
	r = YMM()
	VPAND(maskFF, px, r)
	g = YMM()
	VPSRLD(Imm(8), px, g)
	VPAND(maskFF, g, g)
	b = YMM()
	VPSRLD(Imm(16), px, b)
	VPAND(maskFF, b, b)
	return r, g, b
}

// avgChan emits (a + b) / 2 for two 8-bit channel registers.
func avgChan(a, b GPVirtual) GPVirtual {
	t := GP32()
	LEAQ(Mem{Base: a.As64(), Index: b.As64(), Scale: 1}, t.As64())
	SHRL(Imm(1), t)
	return t
}
