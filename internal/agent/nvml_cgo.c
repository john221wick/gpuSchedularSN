//go:build !mock && !smi

// Separate translation unit for the NVIDIA backend. See cgo.go for why each
// vendor .c must be compiled on its own (file-scope static name collisions).
#include "gpu/gpu.h"
#include "gpu/vendor/nvidia/nvml.c"
