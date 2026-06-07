//go:build !mock && !smi

// Separate translation unit for the GPU dispatch layer. See cgo.go.
// gpu.c calls into the vendor backends via their (non-static) public API,
// declared in the vendor headers, so cross-TU linkage resolves at link time.
#include "gpu/gpu.c"
