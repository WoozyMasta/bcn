// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

// Command asmgen generates the amd64 SIMD kernels for the internal/simd
// package via avo. Regenerate with `make generate` from the repository root.
package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

func main() {
	// No Package(...) call: kernel signatures use only built-in types, and
	// loading the target package would fail before stubs are generated.
	ConstraintExpr("amd64,!purego")

	genFindMinMax()

	consts := genDecodeConsts()
	alphaConsts := genAlphaConsts()
	genMiscConsts()
	mipmapConsts := genMipmapConsts()
	genDecodeDXT1Row(consts)
	genDecodeDXT3Row(consts)
	genDecodeDXT5Row(consts, alphaConsts)
	genDecodeBC4Row(alphaConsts)
	genDecodeBC5Row(alphaConsts)

	genPackDXT1Indices(consts)
	genScoreDXT1Palette()

	alphaShifts := genAlphaIdxShifts()
	genAlphaBlockError(alphaConsts)
	genBestAlphaIndices(alphaConsts, alphaShifts)
	genLSQColorAccumulate()
	genLSQAlphaAccumulate(alphaConsts)
	genDownscaleNRGBARow2x(mipmapConsts)

	Generate()
}

// genFindMinMax emits the SSE2 kernel computing per-channel min/max over a
// 4x4 RGBA block. Byte layout matches [16]rgba8 (64 contiguous bytes).
func genFindMinMax() {
	TEXT("FindMinMaxSSE2", NOSPLIT, "func(block *[64]byte) uint64")
	Pragma("noescape")
	Doc(
		"FindMinMaxSSE2 computes per-channel min/max over 16 RGBA pixels.",
		"Returns min RGBA packed little-endian in the low 32 bits and max RGBA in the high 32 bits.",
	)

	ptr := Load(Param("block"), GP64())

	rows := []VecVirtual{XMM(), XMM(), XMM(), XMM()}
	for i, r := range rows {
		MOVOU(Mem{Base: ptr, Disp: 16 * i}, r)
	}

	// Byte-wise min/max across the 4 rows leaves 4 candidate pixels per XMM.
	// Row 0 is reused as the max accumulator (PMAXUB destroys its dest).
	minV := XMM()
	MOVOU(rows[0], minV)
	maxV := rows[0]
	for _, r := range rows[1:] {
		PMINUB(r, minV)
		PMAXUB(r, maxV)
	}

	// Reduce 4 pixels to 1: fold qword halves, then the two remaining dwords.
	tmp := XMM()
	PSHUFD(Imm(0x4E), minV, tmp)
	PMINUB(tmp, minV)
	PSHUFD(Imm(0xE5), minV, tmp)
	PMINUB(tmp, minV)

	PSHUFD(Imm(0x4E), maxV, tmp)
	PMAXUB(tmp, maxV)
	PSHUFD(Imm(0xE5), maxV, tmp)
	PMAXUB(tmp, maxV)

	// Pack min (dword0) and max (dword0) into one qword inside the XMM
	// domain: a plan9 "MOVD xmm, reg64" assembles as a 64-bit move and would
	// leak reduction garbage from dword1, so no scalar 32-bit extracts here.
	PUNPCKLLQ(maxV, minV)
	out := GP64()
	MOVQ(minV, out)

	Store(out, ReturnIndex(0))
	RET()
}
