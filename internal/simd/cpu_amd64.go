// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

// The assembly kernels (kernels_amd64.s, kernels_stubs_amd64.go) are generated
// by the build-time-only module in ./asmgen; regenerate with `make generate`.

package simd

import (
	"os"

	"golang.org/x/sys/cpu"
)

var (
	// Enabled gates all assembly kernels. SSE2 is part of the amd64 baseline,
	// so only the BCN_PUREGO=1 escape hatch can disable them at runtime.
	Enabled = os.Getenv("BCN_PUREGO") == ""
	// HasAVX2 enables AVX2 kernels; x/sys/cpu verifies OS support (XGETBV).
	HasAVX2 = Enabled && cpu.X86.HasAVX2
	// HasAVX2BMI2 additionally requires BMI2 (PDEP) for alpha and DXT3/DXT5
	// index unpacking.
	HasAVX2BMI2 = HasAVX2 && cpu.X86.HasBMI2
)
