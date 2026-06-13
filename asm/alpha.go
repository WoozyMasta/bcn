// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

// genAlphaIdxShifts emits the per-lane bit positions used to pack eight 3-bit
// alpha indices into a 24-bit field (sample i at bit 3i).
func genAlphaIdxShifts() Mem {
	m := GLOBL("alphaIdxShifts", RODATA|NOPTR)
	for i := range 8 {
		DATA(i*4, U32(uint32(i*3))) // #nosec G115 -- small loop constants.
	}
	return m
}

// loadAlphaSamples widens the 16 alpha bytes at sp into two int32 YMM groups.
func loadAlphaSamples(sp Register) (lo, hi VecVirtual) {
	lo = YMM()
	VPMOVZXBD(Mem{Base: sp}, lo)
	hi = YMM()
	VPMOVZXBD(Mem{Base: sp, Disp: 8}, hi)
	return lo, hi
}

// sqDiff emits (samp - pv)^2 for an int32 lane group.
func sqDiff(samp, pv VecVirtual) VecVirtual {
	d := YMM()
	VPSUBD(pv, samp, d)
	VPMULLD(d, d, d)
	return d
}

// genAlphaBlockError emits the AVX2 kernel computing the total squared error of
// 16 alpha samples against an 8-entry palette. Like the color score kernel it
// evaluates all samples exactly (no cutoff): callers compare with strict <, so
// the exact total and the scalar cutoff sum select the same winner.
func genAlphaBlockError() {
	TEXT("alphaBlockErrorAVX2", NOSPLIT, "func(samples *[16]uint8, palette *[8]int32) uint32")
	Pragma("noescape")
	Doc(
		"alphaBlockErrorAVX2 returns the summed minimum squared error of 16 alpha",
		"samples against an 8-entry palette (BC3/BC4 alpha block scoring).",
	)

	sp := Load(Param("samples"), GP64())
	pp := Load(Param("palette"), GP64())

	loS, hiS := loadAlphaSamples(sp)

	p0 := YMM()
	VPBROADCASTD(Mem{Base: pp}, p0)
	runLo := sqDiff(loS, p0)
	runHi := sqDiff(hiS, p0)

	for e := 1; e < 8; e++ {
		pv := YMM()
		VPBROADCASTD(Mem{Base: pp, Disp: e * 4}, pv)
		VPMINSD(sqDiff(loS, pv), runLo, runLo)
		VPMINSD(sqDiff(hiS, pv), runHi, runHi)
	}

	sum := YMM()
	VPADDD(runHi, runLo, sum)
	out := horizontalAddYMM(sum)
	VZEROUPPER()
	Store(out, ReturnIndex(0))
	RET()
}

// genBestAlphaIndices emits the AVX2 kernel assigning each of 16 alpha samples
// the nearest 8-entry palette index and packing the 3-bit indices into a 48-bit
// value (sample i at bit 3i), matching the scalar encoder's bit order.
func genBestAlphaIndices(shifts Mem) {
	TEXT("bestAlphaIndices16AVX2", NOSPLIT, "func(samples *[16]uint8, palette *[8]int32) uint64")
	Pragma("noescape")
	Doc(
		"bestAlphaIndices16AVX2 returns the packed 48-bit BC3/BC4 alpha indices",
		"for 16 samples against an 8-entry palette. Ties keep the lowest index.",
	)

	sp := Load(Param("samples"), GP64())
	pp := Load(Param("palette"), GP64())

	loS, hiS := loadAlphaSamples(sp)

	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	one := YMM()
	VPSRLD(Imm(31), ones, one)

	p0 := YMM()
	VPBROADCASTD(Mem{Base: pp}, p0)
	bestLo := sqDiff(loS, p0)
	bestHi := sqDiff(hiS, p0)
	idxLo := YMM()
	VPXOR(idxLo, idxLo, idxLo)
	idxHi := YMM()
	VPXOR(idxHi, idxHi, idxHi)
	cur := YMM()
	VMOVDQU(one, cur)

	for e := 1; e < 8; e++ {
		pv := YMM()
		VPBROADCASTD(Mem{Base: pp, Disp: e * 4}, pv)

		eLo := sqDiff(loS, pv)
		mLo := YMM()
		VPCMPGTD(eLo, bestLo, mLo)
		VPBLENDVB(mLo, eLo, bestLo, bestLo)
		VPBLENDVB(mLo, cur, idxLo, idxLo)

		eHi := sqDiff(hiS, pv)
		mHi := YMM()
		VPCMPGTD(eHi, bestHi, mHi)
		VPBLENDVB(mHi, eHi, bestHi, bestHi)
		VPBLENDVB(mHi, cur, idxHi, idxHi)

		if e < 7 {
			VPADDD(one, cur, cur)
		}
	}

	sh := YMM()
	VMOVDQU(shifts, sh)
	VPSLLVD(sh, idxLo, idxLo)
	VPSLLVD(sh, idxHi, idxHi)

	loBits := horizontalOrYMM(idxLo)
	hiBits := horizontalOrYMM(idxHi)

	res := GP64()
	MOVL(loBits, res.As32())
	hi := GP64()
	MOVL(hiBits, hi.As32())
	SHLQ(Imm(24), hi)
	ORQ(hi, res)

	Store(res, ReturnIndex(0))
	RET()
}

// horizontalAddYMM sums the 8 int32 lanes of v into a GP32 result.
func horizontalAddYMM(v VecVirtual) GPVirtual {
	x := v.AsX()
	hi := XMM()
	VEXTRACTI128(Imm(1), v, hi)
	VPADDD(hi, x, x)
	t := XMM()
	VPSHUFD(Imm(0x4E), x, t)
	VPADDD(t, x, x)
	VPSHUFD(Imm(0xE5), x, t)
	VPADDD(t, x, x)
	out := GP32()
	VMOVD(x, out)
	return out
}

// horizontalOrYMM ORs the 8 int32 lanes of v into a GP32 result.
func horizontalOrYMM(v VecVirtual) GPVirtual {
	x := v.AsX()
	hi := XMM()
	VEXTRACTI128(Imm(1), v, hi)
	VPOR(hi, x, x)
	t := XMM()
	VPSHUFD(Imm(0x4E), x, t)
	VPOR(t, x, x)
	VPSHUFD(Imm(0xE5), x, t)
	VPOR(t, x, x)
	out := GP32()
	VMOVD(x, out)
	return out
}
