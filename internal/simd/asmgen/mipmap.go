// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
)

type mipmapConsts struct {
	evenMask Mem
	oddMask  Mem
	bias     Mem
}

func genMipmapConsts() mipmapConsts {
	evenMask := GLOBL("mipmapEvenPixelMask", RODATA|NOPTR)
	DATA(0, U64(0x0b0a090803020100))
	DATA(8, U64(0x8080808080808080))

	oddMask := GLOBL("mipmapOddPixelMask", RODATA|NOPTR)
	DATA(0, U64(0x0f0e0d0c07060504))
	DATA(8, U64(0x8080808080808080))

	bias := GLOBL("mipmapAvg4Bias", RODATA|NOPTR)
	DATA(0, U64(0x0002000200020002))
	DATA(8, U64(0x0002000200020002))

	return mipmapConsts{evenMask: evenMask, oddMask: oddMask, bias: bias}
}

// genDownscaleNRGBARow2x emits a row kernel for exact RGBA8 box filtering.
// It processes n output pixels, where n must be even
// and source rows contain at least 2*n pixels.
// Edges and tails stay in the Go caller.
func genDownscaleNRGBARow2x(c mipmapConsts) {
	TEXT("DownscaleNRGBARow2xAVX2", NOSPLIT, "func(dst *byte, row0 *byte, row1 *byte, n int)")
	Pragma("noescape")
	Doc(
		"DownscaleNRGBARow2xAVX2 downsamples one NRGBA row by 2x using an exact rounded 2x2 box filter.",
		"n is the even number of output pixels to write; odd tails and clamp edges are handled by Go.",
	)

	dst := Load(Param("dst"), GP64())
	row0 := Load(Param("row0"), GP64())
	row1 := Load(Param("row1"), GP64())
	n := Load(Param("n"), GP64())

	// One loop iteration consumes 4 source pixels from each row and emits 2 destination pixels.
	// The Go caller passes only the even interior width.
	pairs := GP64()
	MOVQ(n, pairs)
	SHRQ(Imm(1), pairs)
	CMPQ(pairs, Imm(0))
	JE(LabelRef("done"))

	// The shuffle masks split RGBA pixels 0/2 and 1/3 from a 16-byte source chunk,
	// placing the two horizontal samples for each output pixel together.
	evenMask := XMM()
	oddMask := XMM()
	bias := XMM()
	VMOVDQU(c.evenMask, evenMask)
	VMOVDQU(c.oddMask, oddMask)
	VMOVDQU(c.bias, bias)

	// Used by VPUNPCKLBW to widen byte channels to words before accumulation.
	zero := XMM()
	VPXOR(zero, zero, zero)

	Label("loop")
	// Load 4 RGBA pixels from both source rows: enough for 2 output pixels.
	top := XMM()
	bottom := XMM()
	VMOVDQU(Mem{Base: row0}, top)
	VMOVDQU(Mem{Base: row1}, bottom)

	// Select left/right horizontal samples from each row.
	// Masked-off bytes are 0x80 in the constant and become zero,
	// so only the low 8 bytes are live.
	topEven := XMM()
	topOdd := XMM()
	bottomEven := XMM()
	bottomOdd := XMM()
	VPSHUFB(evenMask, top, topEven)
	VPSHUFB(oddMask, top, topOdd)
	VPSHUFB(evenMask, bottom, bottomEven)
	VPSHUFB(oddMask, bottom, bottomOdd)

	// Widen to u16, accumulate four samples per channel,
	// add +2 for nearest integer rounding,
	// divide by 4, then pack back to RGBA8.
	sum := XMM()
	tmp := XMM()
	VPUNPCKLBW(zero, topEven, sum)
	VPUNPCKLBW(zero, topOdd, tmp)
	VPADDW(tmp, sum, sum)
	VPUNPCKLBW(zero, bottomEven, tmp)
	VPADDW(tmp, sum, sum)
	VPUNPCKLBW(zero, bottomOdd, tmp)
	VPADDW(tmp, sum, sum)
	VPADDW(bias, sum, sum)
	VPSRLW(Imm(2), sum, sum)
	VPACKUSWB(sum, sum, sum)

	// The two averaged RGBA pixels occupy the low 8 bytes.
	VMOVQ(sum, Mem{Base: dst})

	ADDQ(Imm(16), row0)
	ADDQ(Imm(16), row1)
	ADDQ(Imm(8), dst)
	DECQ(pairs)
	JNZ(LabelRef("loop"))

	Label("done")
	RET()
}
