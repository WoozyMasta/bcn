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
