// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/reg"     //nolint:revive,staticcheck // avo DSL convention
)

// decodeConsts holds shared read-only vectors for the decode kernels.
type decodeConsts struct {
	shiftsLo Mem // dword shifts 0,2,..,14 for color index extraction (pixels 0-7)
	shiftsHi Mem // dword shifts 16,18,..,30 (pixels 8-15)
}

// genDecodeConsts emits the shared constant globals.
func genDecodeConsts() decodeConsts {
	lo := GLOBL("decodeIdxShiftsLo", RODATA|NOPTR)
	for i := range 8 {
		DATA(i*4, U32(uint32(i*2))) // #nosec G115 -- small loop constants.
	}

	hi := GLOBL("decodeIdxShiftsHi", RODATA|NOPTR)
	for i := range 8 {
		DATA(i*4, U32(uint32(16+i*2))) // #nosec G115 -- small loop constants.
	}

	return decodeConsts{shiftsLo: lo, shiftsHi: hi}
}

// expand565RGB emits scalar code expanding a packed RGB565 word (zero-extended
// in c) to separate 8-bit channels using exact multiply-shift identities:
// 5-bit: (v*527+23)>>6, 6-bit: (v*259+33)>>6 (byte-identical to (v*255+k)/d).
func expand565RGB(c GPVirtual) (r8, g8, b8 GPVirtual) {
	// avo v0.6 lacks imm32 forms of IMUL3L, so multiplies use the Q form on
	// zero-extended values (identical low 32 bits).
	r8 = GP32()
	MOVL(c.As32(), r8)
	SHRL(Imm(11), r8)
	IMUL3Q(U32(527), r8.As64(), r8.As64())
	ADDL(Imm(23), r8)
	SHRL(Imm(6), r8)

	g8 = GP32()
	MOVL(c.As32(), g8)
	SHRL(Imm(5), g8)
	ANDL(Imm(63), g8)
	IMUL3Q(U32(259), g8.As64(), g8.As64())
	ADDL(Imm(33), g8)
	SHRL(Imm(6), g8)

	b8 = GP32()
	MOVL(c.As32(), b8)
	ANDL(Imm(31), b8)
	IMUL3Q(U32(527), b8.As64(), b8.As64())
	ADDL(Imm(23), b8)
	SHRL(Imm(6), b8)

	return r8, g8, b8
}

// expand565 expands a packed RGB565 word and additionally packs the channels
// into a little-endian RGBA dword (alpha 0xFF).
func expand565(c GPVirtual) (packed, r8, g8, b8 GPVirtual) {
	r8, g8, b8 = expand565RGB(c)

	packed = GP32()
	MOVL(g8, packed)
	SHLL(Imm(8), packed)
	ORL(r8, packed)
	t := GP32()
	MOVL(b8, t)
	SHLL(Imm(16), t)
	ORL(t, packed)
	ORL(U32(0xFF000000), packed)

	return packed, r8, g8, b8
}

// interp3 emits (2*a + b + 1) / 3 for 8-bit channels via the exact
// multiply-shift (n*683)>>11 (exact for n <= 766).
func interp3(a, b GPVirtual) GPVirtual {
	t := GP32()
	LEAQ(Mem{Base: b.As64(), Index: a.As64(), Scale: 2, Disp: 1}, t.As64())
	IMUL3Q(U32(683), t.As64(), t.As64())
	SHRL(Imm(11), t)
	return t
}

// packRGB packs three 8-bit channel registers and alpha into an RGBA dword.
// Channel values must already fit a byte; alpha is a compile-time constant.
func packRGB(r, g, b GPVirtual, alpha uint32) GPVirtual {
	p := GP32()
	MOVL(g, p)
	SHLL(Imm(8), p)
	ORL(r, p)
	t := GP32()
	MOVL(b, t)
	SHLL(Imm(16), t)
	ORL(t, p)
	if alpha != 0 {
		ORL(U32(alpha<<24), p)
	}
	return p
}

// emitDXT1Palette emits scalar BC1 palette construction for endpoints c0, c1
// (zero-extended 16-bit) and returns the palette in the low XMM lanes of a
// YMM register (entries 0..3 as little-endian RGBA dwords, upper lanes zero).
// Entries are inserted into the vector as soon as they are packed to keep GP
// register pressure low; channel pairs are interpolated together so their
// source registers die early.
func emitDXT1Palette(c0, c1 GPVirtual) VecVirtual {
	pal := YMM()

	p0, r0, g0, b0 := expand565(c0)
	// VEX VMOVD zeroes lanes 1..3 and the upper YMM half: entry 3 of the
	// 3-color mode (transparent black) is in place from the start.
	VMOVD(p0, pal.AsX())
	p1, r1, g1, b1 := expand565(c1)
	VPINSRD(Imm(1), p1, pal.AsX(), pal.AsX())

	CMPL(c0.As32(), c1.As32())
	JHI(LabelRef("mode4"))

	// 3-color mode: entry 2 = (p0+p1)/2, entry 3 stays transparent black.
	ar := GP32()
	LEAQ(Mem{Base: r0.As64(), Index: r1.As64(), Scale: 1}, ar.As64())
	SHRL(Imm(1), ar)
	ag := GP32()
	LEAQ(Mem{Base: g0.As64(), Index: g1.As64(), Scale: 1}, ag.As64())
	SHRL(Imm(1), ag)
	ab := GP32()
	LEAQ(Mem{Base: b0.As64(), Index: b1.As64(), Scale: 1}, ab.As64())
	SHRL(Imm(1), ab)
	VPINSRD(Imm(2), packRGB(ar, ag, ab, 255), pal.AsX(), pal.AsX())
	JMP(LabelRef("merge"))

	Label("mode4")
	er2 := interp3(r0, r1)
	er3 := interp3(r1, r0)
	eg2 := interp3(g0, g1)
	eg3 := interp3(g1, g0)
	eb2 := interp3(b0, b1)
	eb3 := interp3(b1, b0)
	VPINSRD(Imm(2), packRGB(er2, eg2, eb2, 255), pal.AsX(), pal.AsX())
	VPINSRD(Imm(3), packRGB(er3, eg3, eb3, 255), pal.AsX(), pal.AsX())

	Label("merge")
	return pal
}

// genAlphaConsts emits weight/bias/multiplier vectors for branch-reduced
// BC3/BC4 alpha palette construction. Layout: two 64-byte groups
// (a0>a1 "mode7" at +0, else "mode5" at +64), each wa|wb|bias|mul of 8 words.
// Palette lane k = (wa[k]*a0 + wb[k]*a1 + bias[k]) * mul[k] >> 16, where the
// multiply-shift reproduces floor(n/7) (n<=1788) and floor(n/5) (n<=1277)
// exactly; mode5 lanes 6,7 use zero weights and biases 0 / 1277 to produce
// the constant 0 and 255 entries.
func genAlphaConsts() Mem {
	m := GLOBL("decodeAlphaPalConsts", RODATA|NOPTR)
	vecs := [][8]uint16{
		{7, 0, 6, 5, 4, 3, 2, 1},                                 // wa7
		{0, 7, 1, 2, 3, 4, 5, 6},                                 // wb7
		{3, 3, 3, 3, 3, 3, 3, 3},                                 // bias7
		{9363, 9363, 9363, 9363, 9363, 9363, 9363, 9363},         // mul7 (/7)
		{5, 0, 4, 3, 2, 1, 0, 0},                                 // wa5
		{0, 5, 1, 2, 3, 4, 0, 0},                                 // wb5
		{2, 2, 2, 2, 2, 2, 0, 1277},                              // bias5 (+ const 0, 255 lanes)
		{13108, 13108, 13108, 13108, 13108, 13108, 13108, 13108}, // mul5 (/5)
	}
	off := 0
	for _, v := range vecs {
		for _, w := range v {
			DATA(off, U16(w))
			off += 2
		}
	}

	return m
}

// emitAlphaPaletteWords builds the 8-entry BC3/BC4 alpha palette from a0, a1
// (zero-extended bytes) and returns the values as 8 words in an XMM.
// The mode (a0>a1 vs a0<=a1) is selected branchlessly
// by pointing base at the matching constant group;
// lane k = (wa[k]*a0 + wb[k]*a1 + bias[k]) * mul[k] >> 16.
func emitAlphaPaletteWords(a0, a1 GPVirtual, ac Mem) VecVirtual {
	base := GP64()
	LEAQ(ac.Offset(64), base)
	b7 := GP64()
	LEAQ(ac, b7)
	CMPL(a0, a1)
	CMOVQHI(b7, base)

	xa0 := XMM()
	VMOVD(a0, xa0)
	VPBROADCASTW(xa0, xa0)
	xa1 := XMM()
	VMOVD(a1, xa1)
	VPBROADCASTW(xa1, xa1)

	t0 := XMM()
	VPMULLW(Mem{Base: base}, xa0, t0)
	t1 := XMM()
	VPMULLW(Mem{Base: base, Disp: 16}, xa1, t1)
	VPADDW(t1, t0, t0)
	VPADDW(Mem{Base: base, Disp: 32}, t0, t0)
	VPMULHUW(Mem{Base: base, Disp: 48}, t0, t0)

	return t0
}

// emitAlphaBytes emits BC3/BC4 alpha block decoding for the 8-byte alpha
// payload at src+disp and returns 16 alpha samples as XMM bytes.
// The 48-bit index field is loaded as 4+2 bytes to avoid reading past the
// block, then PDEP spreads the 3-bit indices into bytes for VPSHUFB.
func emitAlphaBytes(src Register, disp int, ac Mem) VecVirtual {
	a0 := GP32()
	MOVBLZX(Mem{Base: src, Disp: disp}, a0)
	a1 := GP32()
	MOVBLZX(Mem{Base: src, Disp: disp + 1}, a1)

	palWords := emitAlphaPaletteWords(a0, a1, ac)
	apal := XMM()
	VPACKUSWB(palWords, palWords, apal)

	vlo := GP64()
	MOVL(Mem{Base: src, Disp: disp + 2}, vlo.As32())
	vhi := GP64()
	MOVWLZX(Mem{Base: src, Disp: disp + 6}, vhi.As32())
	SHLQ(Imm(32), vhi)
	ORQ(vhi, vlo)

	pdepMask := GP64()
	MOVQ(Imm(0x0707070707070707), pdepMask)
	loIdx := GP64()
	PDEPQ(pdepMask, vlo, loIdx)
	SHRQ(Imm(24), vlo)
	hiIdx := GP64()
	PDEPQ(pdepMask, vlo, hiIdx)

	idx := XMM()
	VMOVQ(loIdx, idx)
	VPINSRQ(Imm(1), hiIdx, idx, idx)
	out := XMM()
	VPSHUFB(idx, apal, out)

	return out
}

// emitStore4Rows stores two YMM pixel groups (rows 0-1 and 2-3) into dst.
// The row-2 base is derived locally to avoid a persistent 3*stride register.
func emitStore4Rows(px0, px1 VecVirtual, dst, stride Register) {
	VMOVDQU(px0.AsX(), Mem{Base: dst})
	VEXTRACTI128(Imm(1), px0, Mem{Base: dst, Index: stride, Scale: 1})
	row2 := GP64()
	LEAQ(Mem{Base: dst, Index: stride, Scale: 2}, row2)
	VMOVDQU(px1.AsX(), Mem{Base: row2})
	VEXTRACTI128(Imm(1), px1, Mem{Base: row2, Index: stride, Scale: 1})
}

// genDecodeDXT5Row emits the AVX2+BMI2 kernel decoding n interior DXT5 blocks.
func genDecodeDXT5Row(c decodeConsts, ac Mem) {
	TEXT("DecodeDXT5RowAVX2", NOSPLIT, "func(dst *byte, src *byte, n int, stride int)")
	Pragma("noescape")
	Doc(
		"DecodeDXT5RowAVX2 decodes n consecutive interior DXT5 blocks (16 bytes each)",
		"into dst as 4 NRGBA rows of 16 bytes spaced stride bytes apart. Requires BMI2.",
	)

	dst := Load(Param("dst"), GP64())
	src := Load(Param("src"), GP64())
	n := Load(Param("n"), GP64())
	stride := Load(Param("stride"), GP64())

	shiftsLo := YMM()
	shiftsHi := YMM()
	VMOVDQU(c.shiftsLo, shiftsLo)
	VMOVDQU(c.shiftsHi, shiftsHi)
	mask3 := YMM()
	VPCMPEQD(mask3, mask3, mask3)
	VPSRLD(Imm(30), mask3, mask3)
	maskRGB := YMM()
	VPCMPEQD(maskRGB, maskRGB, maskRGB)
	VPSRLD(Imm(8), maskRGB, maskRGB)

	Label("loop")

	alpha16 := emitAlphaBytes(src, 0, ac)

	c0 := GP32()
	MOVWLZX(Mem{Base: src, Disp: 8}, c0)
	c1 := GP32()
	MOVWLZX(Mem{Base: src, Disp: 10}, c1)
	pal := emitDXT1Palette(c0, c1)

	yIdx := YMM()
	VPBROADCASTD(Mem{Base: src, Disp: 12}, yIdx)
	i0 := YMM()
	VPSRLVD(shiftsLo, yIdx, i0)
	VPAND(mask3, i0, i0)
	i1 := YMM()
	VPSRLVD(shiftsHi, yIdx, i1)
	VPAND(mask3, i1, i1)

	px0 := YMM()
	VPERMD(pal, i0, px0)
	px1 := YMM()
	VPERMD(pal, i1, px1)
	VPAND(maskRGB, px0, px0)
	VPAND(maskRGB, px1, px1)

	ad := YMM()
	VPMOVZXBD(alpha16, ad)
	VPSLLD(Imm(24), ad, ad)
	VPOR(ad, px0, px0)
	ahi := XMM()
	VPSRLDQ(Imm(8), alpha16, ahi)
	VPMOVZXBD(ahi, ad)
	VPSLLD(Imm(24), ad, ad)
	VPOR(ad, px1, px1)

	emitStore4Rows(px0, px1, dst, stride)

	ADDQ(Imm(16), src)
	ADDQ(Imm(16), dst)
	DECQ(n)
	JNZ(LabelRef("loop"))

	VZEROUPPER()
	RET()
}

// emitDXT3Nibbles expands the 16 explicit 4-bit alpha values
// at src[0:8] into 16 bytes (values 0..15) in an XMM, in pixel order.
// Two PDEPs spread each 32-bit nibble group (pixels 0-7 and 8-15)
// into one byte per nibble.
func emitDXT3Nibbles(src Register) VecVirtual {
	mask := GP64()
	MOVQ(Imm(0x0F0F0F0F0F0F0F0F), mask)

	vlo := GP64()
	MOVL(Mem{Base: src}, vlo.As32())
	loN := GP64()
	PDEPQ(mask, vlo, loN)

	vhi := GP64()
	MOVL(Mem{Base: src, Disp: 4}, vhi.As32())
	hiN := GP64()
	PDEPQ(mask, vhi, hiN)

	out := XMM()
	VMOVQ(loN, out)
	VPINSRQ(Imm(1), hiN, out, out)
	return out
}

// emitDXT3AlphaInto expands 8 nibble bytes to dword alpha (value*17),
// shifts to the high byte and ORs them into the masked color pixels.
func emitDXT3AlphaInto(nib VecVirtual, px VecVirtual) {
	ad := YMM()
	VPMOVZXBD(nib, ad)
	// value * 17 = (value << 4) + value, exact for nibbles (max 255).
	t := YMM()
	VPSLLD(Imm(4), ad, t)
	VPADDD(t, ad, ad)
	VPSLLD(Imm(24), ad, ad)
	VPOR(ad, px, px)
}

// genDecodeDXT3Row emits the AVX2+BMI2 kernel decoding n interior DXT3 blocks.
// Color decoding mirrors DXT1; alpha is the explicit 4-bit field expanded *17.
func genDecodeDXT3Row(c decodeConsts) {
	TEXT("DecodeDXT3RowAVX2", NOSPLIT, "func(dst *byte, src *byte, n int, stride int)")
	Pragma("noescape")
	Doc(
		"DecodeDXT3RowAVX2 decodes n consecutive interior DXT3 blocks (16 bytes each)",
		"into dst as 4 NRGBA rows of 16 bytes spaced stride bytes apart. Requires BMI2.",
	)

	dst := Load(Param("dst"), GP64())
	src := Load(Param("src"), GP64())
	n := Load(Param("n"), GP64())
	stride := Load(Param("stride"), GP64())

	shiftsLo := YMM()
	shiftsHi := YMM()
	VMOVDQU(c.shiftsLo, shiftsLo)
	VMOVDQU(c.shiftsHi, shiftsHi)
	mask3 := YMM()
	VPCMPEQD(mask3, mask3, mask3)
	VPSRLD(Imm(30), mask3, mask3)
	maskRGB := YMM()
	VPCMPEQD(maskRGB, maskRGB, maskRGB)
	VPSRLD(Imm(8), maskRGB, maskRGB)

	Label("loop")

	nib := emitDXT3Nibbles(src)

	c0 := GP32()
	MOVWLZX(Mem{Base: src, Disp: 8}, c0)
	c1 := GP32()
	MOVWLZX(Mem{Base: src, Disp: 10}, c1)
	pal := emitDXT1Palette(c0, c1)

	yIdx := YMM()
	VPBROADCASTD(Mem{Base: src, Disp: 12}, yIdx)
	i0 := YMM()
	VPSRLVD(shiftsLo, yIdx, i0)
	VPAND(mask3, i0, i0)
	i1 := YMM()
	VPSRLVD(shiftsHi, yIdx, i1)
	VPAND(mask3, i1, i1)

	px0 := YMM()
	VPERMD(pal, i0, px0)
	px1 := YMM()
	VPERMD(pal, i1, px1)
	VPAND(maskRGB, px0, px0)
	VPAND(maskRGB, px1, px1)

	emitDXT3AlphaInto(nib, px0)
	nibHi := XMM()
	VPSRLDQ(Imm(8), nib, nibHi)
	emitDXT3AlphaInto(nibHi, px1)

	emitStore4Rows(px0, px1, dst, stride)

	ADDQ(Imm(16), src)
	ADDQ(Imm(16), dst)
	DECQ(n)
	JNZ(LabelRef("loop"))

	VZEROUPPER()
	RET()
}

// genDecodeBC4Row emits the AVX2+BMI2 kernel decoding n interior BC4 blocks
// into gray RGBA pixels.
func genDecodeBC4Row(ac Mem) {
	TEXT("DecodeBC4RowAVX2", NOSPLIT, "func(dst *byte, src *byte, n int, stride int)")
	Pragma("noescape")
	Doc(
		"DecodeBC4RowAVX2 decodes n consecutive interior BC4 blocks (8 bytes each)",
		"into dst as 4 gray NRGBA rows spaced stride bytes apart. Requires BMI2.",
	)

	dst := Load(Param("dst"), GP64())
	src := Load(Param("src"), GP64())
	n := Load(Param("n"), GP64())
	stride := Load(Param("stride"), GP64())

	// 0x00010101 replicates a byte over the low three dword bytes.
	gray := YMM()
	VPBROADCASTD(grayMulConst, gray)
	alphaFF := YMM()
	VPCMPEQD(alphaFF, alphaFF, alphaFF)
	VPSLLD(Imm(24), alphaFF, alphaFF)

	Label("loop")

	alpha16 := emitAlphaBytes(src, 0, ac)

	px0 := YMM()
	VPMOVZXBD(alpha16, px0)
	VPMULLD(gray, px0, px0)
	VPOR(alphaFF, px0, px0)
	ahi := XMM()
	VPSRLDQ(Imm(8), alpha16, ahi)
	px1 := YMM()
	VPMOVZXBD(ahi, px1)
	VPMULLD(gray, px1, px1)
	VPOR(alphaFF, px1, px1)

	emitStore4Rows(px0, px1, dst, stride)

	ADDQ(Imm(8), src)
	ADDQ(Imm(16), dst)
	DECQ(n)
	JNZ(LabelRef("loop"))

	VZEROUPPER()
	RET()
}

// genDecodeBC5Row emits the AVX2+BMI2 kernel decoding n interior BC5 blocks
// (R and G channels from two alpha payloads, B=0, A=255).
func genDecodeBC5Row(ac Mem) {
	TEXT("DecodeBC5RowAVX2", NOSPLIT, "func(dst *byte, src *byte, n int, stride int)")
	Pragma("noescape")
	Doc(
		"DecodeBC5RowAVX2 decodes n consecutive interior BC5 blocks (16 bytes each)",
		"into dst as 4 NRGBA rows (R, G, 0, 255) spaced stride bytes apart. Requires BMI2.",
	)

	dst := Load(Param("dst"), GP64())
	src := Load(Param("src"), GP64())
	n := Load(Param("n"), GP64())
	stride := Load(Param("stride"), GP64())

	alphaFF := YMM()
	VPCMPEQD(alphaFF, alphaFF, alphaFF)
	VPSLLD(Imm(24), alphaFF, alphaFF)

	Label("loop")

	r16 := emitAlphaBytes(src, 0, ac)
	g16 := emitAlphaBytes(src, 8, ac)

	// Interleave R and G into 16-bit r|g<<8 pairs, widen to dwords, add alpha.
	w0 := XMM()
	VPUNPCKLBW(g16, r16, w0)
	px0 := YMM()
	VPMOVZXWD(w0, px0)
	VPOR(alphaFF, px0, px0)
	w1 := XMM()
	VPUNPCKHBW(g16, r16, w1)
	px1 := YMM()
	VPMOVZXWD(w1, px1)
	VPOR(alphaFF, px1, px1)

	emitStore4Rows(px0, px1, dst, stride)

	ADDQ(Imm(16), src)
	ADDQ(Imm(16), dst)
	DECQ(n)
	JNZ(LabelRef("loop"))

	VZEROUPPER()
	RET()
}

// grayMulConst is the BC4 gray replication constant global.
var grayMulConst Mem

// genMiscConsts emits small shared scalar constants.
func genMiscConsts() {
	grayMulConst = GLOBL("decodeGrayMul", RODATA|NOPTR)
	DATA(0, U32(0x00010101))
}

// genDecodeDXT1Row emits the AVX2 kernel decoding n consecutive interior
// DXT1 blocks straight into the destination image rows.
func genDecodeDXT1Row(c decodeConsts) {
	TEXT("DecodeDXT1RowAVX2", NOSPLIT, "func(dst *byte, src *byte, n int, stride int)")
	Pragma("noescape")
	Doc(
		"DecodeDXT1RowAVX2 decodes n consecutive interior DXT1 blocks (8 bytes each)",
		"into dst as 4 NRGBA rows of 16 bytes spaced stride bytes apart.",
	)

	dst := Load(Param("dst"), GP64())
	src := Load(Param("src"), GP64())
	n := Load(Param("n"), GP64())
	stride := Load(Param("stride"), GP64())

	shiftsLo := YMM()
	shiftsHi := YMM()
	VMOVDQU(c.shiftsLo, shiftsLo)
	VMOVDQU(c.shiftsHi, shiftsHi)
	mask3 := YMM()
	VPCMPEQD(mask3, mask3, mask3)
	VPSRLD(Imm(30), mask3, mask3)

	Label("loop")

	c0 := GP32()
	MOVWLZX(Mem{Base: src}, c0)
	c1 := GP32()
	MOVWLZX(Mem{Base: src, Disp: 2}, c1)
	pal := emitDXT1Palette(c0, c1)

	yIdx := YMM()
	VPBROADCASTD(Mem{Base: src, Disp: 4}, yIdx)
	i0 := YMM()
	VPSRLVD(shiftsLo, yIdx, i0)
	VPAND(mask3, i0, i0)
	i1 := YMM()
	VPSRLVD(shiftsHi, yIdx, i1)
	VPAND(mask3, i1, i1)

	px0 := YMM()
	VPERMD(pal, i0, px0)
	px1 := YMM()
	VPERMD(pal, i1, px1)

	emitStore4Rows(px0, px1, dst, stride)

	ADDQ(Imm(8), src)
	ADDQ(Imm(16), dst)
	DECQ(n)
	JNZ(LabelRef("loop"))

	VZEROUPPER()
	RET()
}
