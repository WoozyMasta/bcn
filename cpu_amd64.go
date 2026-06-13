// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

package bcn

import (
	"os"

	"golang.org/x/sys/cpu"
)

var (
	// useASM gates all assembly kernels. SSE2 is part of the amd64 baseline,
	// so only the BCN_PUREGO=1 escape hatch can disable them at runtime.
	useASM = os.Getenv("BCN_PUREGO") == ""
	// hasAVX2 enables AVX2 kernels; x/sys/cpu verifies OS support (XGETBV).
	hasAVX2 = useASM && cpu.X86.HasAVX2
	// hasAVX2BMI2 additionally requires BMI2 (PDEP) for alpha index unpacking.
	hasAVX2BMI2 = hasAVX2 && cpu.X86.HasBMI2
)
