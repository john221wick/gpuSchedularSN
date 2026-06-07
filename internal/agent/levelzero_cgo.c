//go:build !mock && !smi

// Separate translation unit for the Intel Level Zero backend. See cgo.go.
#include "gpu/gpu.h"
#include "gpu/vendor/intel/levelzero.c"
