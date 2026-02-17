// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

// CubeFace identifies cubemap face order used by EncodeDDS/KTX.
type CubeFace int

const (
	// CubeFacePosX is +X.
	CubeFacePosX CubeFace = iota
	// CubeFaceNegX is -X.
	CubeFaceNegX
	// CubeFacePosY is +Y.
	CubeFacePosY
	// CubeFaceNegY is -Y.
	CubeFaceNegY
	// CubeFacePosZ is +Z.
	CubeFacePosZ
	// CubeFaceNegZ is -Z.
	CubeFaceNegZ
)
