// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build amd64 && !purego

package bcn

import "os"

// useASM gates all assembly kernels. SSE2 is part of the amd64 baseline,
// so only the BCN_PUREGO=1 escape hatch can disable them at runtime.
// AVX2 detection (golang.org/x/sys/cpu) arrives with the AVX2 kernels.
var useASM = os.Getenv("BCN_PUREGO") == ""
