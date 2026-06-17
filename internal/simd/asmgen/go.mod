// This module is build-time only: it generates the SIMD assembly kernels in
// internal/simd via avo. It is intentionally separate from the root module so
// that consumers of github.com/woozymasta/bcn do not pull avo and its
// dependencies into their module graph.
module github.com/woozymasta/bcn/internal/simd/asmgen

go 1.25.5

require github.com/mmcloughlin/avo v0.6.0

require (
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/tools v0.46.0 // indirect
)
