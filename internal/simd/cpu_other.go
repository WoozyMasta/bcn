// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

//go:build !amd64 || purego

package simd

// No assembly kernels on this platform; the bcn package uses its pure-Go path.
const (
	Enabled     = false
	HasAVX2     = false
	HasAVX2BMI2 = false
)
