// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package main

import (
	. "github.com/mmcloughlin/avo/build"   //nolint:revive,staticcheck // avo DSL convention
	. "github.com/mmcloughlin/avo/operand" //nolint:revive,staticcheck // avo DSL convention
)

// BC6H 1-subset SOA palette layout in *[48]int32:
//
//	R[16] at byte offset   0 (64 bytes)
//	G[16] at byte offset  64 (64 bytes)
//	B[16] at byte offset 128 (64 bytes)
const (
	bc6hPal16G = 64  // G channel base offset (16-entry palette)
	bc6hPal16B = 128 // B channel base offset (16-entry palette)
)

// BC6H 2-subset SOA palette layout in *[24]int32:
//
//	R[8] at byte offset  0 (32 bytes)
//	G[8] at byte offset 32 (32 bytes)
//	B[8] at byte offset 64 (32 bytes)
const (
	bc6hPal8G = 32 // G channel base offset (8-entry palette)
	bc6hPal8B = 64 // B channel base offset (8-entry palette)
)

// genBC6HFindIndices1Sub emits the AVX2 L1-nearest-palette kernel for BC6H 1-subset.
// Processes all 16 texels against 16 palette entries using L1 (VPABSD+VPADDD)
// rather than L2 (VPMULLD) as in BC7. Uses 8 texels per YMM register in two passes.
func genBC6HFindIndices1Sub(c bc7Consts) {
	TEXT("BC6HFindIndices1SubAVX2", NOSPLIT,
		"func(block *[48]int32, pal *[48]int32, idx *[16]int32)")
	Pragma("noescape")
	Doc(
		"BC6HFindIndices1SubAVX2 assigns 16 BC6H texels to the nearest of 16 palette entries.",
		"block and pal use SOA layout: R[16], G[16], B[16] as int32 (192 bytes each).",
		"idx receives the winning palette index per texel as int32; caller narrows to byte.",
		"Distance metric is L1 (Manhattan): |dr|+|dg|+|db|, matching bc6hFindIndices1SubGo.",
	)

	block := Load(Param("block"), GP64())
	pal := Load(Param("pal"), GP64())
	idxOut := Load(Param("idx"), GP64())

	// Load block channels SOA: R lo (texels 0-7), R hi (texels 8-15), G lo/hi, B lo/hi.
	blkRL := YMM()
	VMOVDQU(Mem{Base: block, Disp: 0}, blkRL)
	blkRH := YMM()
	VMOVDQU(Mem{Base: block, Disp: 32}, blkRH)
	blkGL := YMM()
	VMOVDQU(Mem{Base: block, Disp: bc6hPal16G}, blkGL)
	blkGH := YMM()
	VMOVDQU(Mem{Base: block, Disp: bc6hPal16G + 32}, blkGH)
	blkBL := YMM()
	VMOVDQU(Mem{Base: block, Disp: bc6hPal16B}, blkBL)
	blkBH := YMM()
	VMOVDQU(Mem{Base: block, Disp: bc6hPal16B + 32}, blkBH)

	// Initialize best error to 0x7FFFFFFF (MaxInt32) per lane.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	bestL := YMM()
	VPSRLD(Imm(1), ones, bestL)
	bestH := YMM()
	VMOVDQU(bestL, bestH)

	// Initialize best indices to 0.
	bestIdxL := YMM()
	VPXOR(bestIdxL, bestIdxL, bestIdxL)
	bestIdxH := YMM()
	VPXOR(bestIdxH, bestIdxH, bestIdxH)

	// Shared iteration temporaries
	// (declared once, physical registers reused each iteration by the register allocator
	// since they have non-overlapping live ranges within the unrolled loop body).
	t := YMM()
	errL := YMM()
	errH := YMM()
	tmp := YMM()

	for e := range 16 {
		// R channel: broadcast pal_R[e]; compute absolute difference for lo and hi halves.
		VPBROADCASTD(Mem{Base: pal, Disp: e * 4}, t)
		VPSUBD(t, blkRL, errL)
		VPABSD(errL, errL)
		VPSUBD(t, blkRH, errH)
		VPABSD(errH, errH)
		// G channel: accumulate into errL/errH.
		VPBROADCASTD(Mem{Base: pal, Disp: bc6hPal16G + e*4}, t)
		VPSUBD(t, blkGL, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errL, errL)
		VPSUBD(t, blkGH, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errH, errH)
		// B channel: accumulate; errL/errH now hold the full L1 per texel.
		VPBROADCASTD(Mem{Base: pal, Disp: bc6hPal16B + e*4}, t)
		VPSUBD(t, blkBL, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errL, errL)
		VPSUBD(t, blkBH, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errH, errH)
		// Load index constant e for the blend step.
		VPBROADCASTD(c.indices.Offset(e*4), t)
		// Update lo 8 texels: strict less-than keeps the lower index on ties.
		VPCMPGTD(errL, bestL, tmp) // tmp[i] = bestL[i] > errL[i]
		VPBLENDVB(tmp, errL, bestL, bestL)
		VPBLENDVB(tmp, t, bestIdxL, bestIdxL)
		// Update hi 8 texels (tmp reused as new mask).
		VPCMPGTD(errH, bestH, tmp)
		VPBLENDVB(tmp, errH, bestH, bestH)
		VPBLENDVB(tmp, t, bestIdxH, bestIdxH)
	}

	VMOVDQU(bestIdxL, Mem{Base: idxOut, Disp: 0})
	VMOVDQU(bestIdxH, Mem{Base: idxOut, Disp: 32})
	VZEROUPPER()
	RET()
}

// genBC6HFindIndices2Sub emits the AVX2 L1-nearest-palette kernel for BC6H 2-subset.
// Processes one subset at a time; non-subset texels always produce idx[i]=0.
func genBC6HFindIndices2Sub(c bc7Consts) {
	TEXT("BC6HFindIndices2SubAVX2", NOSPLIT,
		"func(block *[48]int32, pal *[24]int32, part *[16]byte, subset int32, idx *[16]int32)")
	Pragma("noescape")
	Doc(
		"BC6HFindIndices2SubAVX2 assigns BC6H texels of one subset to the nearest of 8 palette entries.",
		"block uses SOA layout: R[16], G[16], B[16] as int32 (192 bytes).",
		"pal uses SOA layout: R[8], G[8], B[8] as int32 (96 bytes).",
		"part[i]&1 must equal subset for texel i to be assigned; non-subset texels get idx[i]=0.",
		"Distance metric is L1 (Manhattan): |dr|+|dg|+|db|, matching bc6hFindIndices2SubGo.",
	)

	block := Load(Param("block"), GP64())
	pal := Load(Param("pal"), GP64())
	part := Load(Param("part"), GP64())
	idxOut := Load(Param("idx"), GP64())

	// Derive lane constants: ones for MaxInt32, one for subset-bit mask.
	ones := YMM()
	VPCMPEQD(ones, ones, ones)
	one := YMM()
	VPSRLD(Imm(31), ones, one) // 0x00000001 per lane

	// Broadcast subset value for comparison (load GP32 -> XMM -> YMM).
	subGP := Load(Param("subset"), GP32())
	subXMM := XMM()
	VMOVD(subGP, subXMM)
	subv := YMM()
	VPBROADCASTD(subXMM, subv)

	// Compute valid masks: 0xFFFFFFFF where part[i]&1 == subset, 0 otherwise.
	pm := YMM()
	VPMOVZXBD(Mem{Base: part, Disp: 0}, pm) // texels 0..7 -> int32
	VPAND(one, pm, pm)
	validL := YMM()
	VPCMPEQD(pm, subv, validL)
	pm2 := YMM()
	VPMOVZXBD(Mem{Base: part, Disp: 8}, pm2) // texels 8..15 -> int32
	VPAND(one, pm2, pm2)
	validH := YMM()
	VPCMPEQD(pm2, subv, validH)

	// Load block channels SOA.
	blkRL := YMM()
	VMOVDQU(Mem{Base: block, Disp: 0}, blkRL)
	blkRH := YMM()
	VMOVDQU(Mem{Base: block, Disp: 32}, blkRH)
	blkGL := YMM()
	VMOVDQU(Mem{Base: block, Disp: 64}, blkGL)
	blkGH := YMM()
	VMOVDQU(Mem{Base: block, Disp: 96}, blkGH)
	blkBL := YMM()
	VMOVDQU(Mem{Base: block, Disp: 128}, blkBL)
	blkBH := YMM()
	VMOVDQU(Mem{Base: block, Disp: 160}, blkBH)

	// Initialize best error to MaxInt32, best index to 0.
	bestL := YMM()
	VPSRLD(Imm(1), ones, bestL)
	bestH := YMM()
	VMOVDQU(bestL, bestH)
	bestIdxL := YMM()
	VPXOR(bestIdxL, bestIdxL, bestIdxL)
	bestIdxH := YMM()
	VPXOR(bestIdxH, bestIdxH, bestIdxH)

	// Shared temporaries.
	t := YMM()
	errL := YMM()
	errH := YMM()
	tmp := YMM()

	for e := range 8 {
		// R channel.
		VPBROADCASTD(Mem{Base: pal, Disp: e * 4}, t)
		VPSUBD(t, blkRL, errL)
		VPABSD(errL, errL)
		VPSUBD(t, blkRH, errH)
		VPABSD(errH, errH)
		// G channel.
		VPBROADCASTD(Mem{Base: pal, Disp: bc6hPal8G + e*4}, t)
		VPSUBD(t, blkGL, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errL, errL)
		VPSUBD(t, blkGH, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errH, errH)
		// B channel.
		VPBROADCASTD(Mem{Base: pal, Disp: bc6hPal8B + e*4}, t)
		VPSUBD(t, blkBL, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errL, errL)
		VPSUBD(t, blkBH, tmp)
		VPABSD(tmp, tmp)
		VPADDD(tmp, errH, errH)
		// Load index constant e.
		VPBROADCASTD(c.indices.Offset(e*4), t)
		// Update lo 8 texels: apply subset mask to bestIdx update.
		VPCMPGTD(errL, bestL, tmp)         // tmp[i] = bestL[i] > errL[i]
		VPBLENDVB(tmp, errL, bestL, bestL) // update best error unconditionally
		VPAND(validL, tmp, tmp)            // apply subset filter to index update
		VPBLENDVB(tmp, t, bestIdxL, bestIdxL)
		// Update hi 8 texels (tmp reused as new mask).
		VPCMPGTD(errH, bestH, tmp)
		VPBLENDVB(tmp, errH, bestH, bestH)
		VPAND(validH, tmp, tmp)
		VPBLENDVB(tmp, t, bestIdxH, bestIdxH)
	}

	VMOVDQU(bestIdxL, Mem{Base: idxOut, Disp: 0})
	VMOVDQU(bestIdxH, Mem{Base: idxOut, Disp: 32})
	VZEROUPPER()
	RET()
}
