//go:build !mock && !smi

// Separate translation unit for the AMD ROCm backend. See cgo.go.
#include "gpu/gpu.h"
#include "gpu/vendor/amd/rocm.c"
