//go:build !mock && !smi

// Separate translation unit for the AMD sysfs backend. See cgo.go.
#include "gpu/gpu.h"
#include "gpu/vendor/amd/sysfs.c"
