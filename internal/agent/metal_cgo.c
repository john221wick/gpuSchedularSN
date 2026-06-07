//go:build !mock && !smi

// Separate translation unit for the Apple Metal backend. See cgo.go.
#include "gpu/gpu.h"
#include "gpu/vendor/apple/metal.c"
