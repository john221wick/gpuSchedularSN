# GPU Schedular Single Node

The goal of this project is to make gpu schedular which would save waste of gpu resources via a single exectable that can handle mulitple Interconnects(NVLink, NVSwitch, Xe Link etc).


# Phase 1 done
1. Added gpu detection - Now it auto detects the gpu on `make run`, so if you want to know about your own computer, just type make run. It will work
2. Fake gpu - For running in Nvidia gpus, i have added **gpu_fake.c** file, which containes four A100(80GB) and there is 2 NVLink, and 4 pcie connection
3. Right now - I am running in macos so there is gpu detection in the beginning itself

# Phase 2 and 3 done

1. I added and tested topology matrix, which is basically for gpus which are made basically about connection of gpus to gpus
2. Added a scorer, which basically finds the best combination of n gpus from the topology matrix with backtracking

# Technical Decisions i took

When building with Go + CGo, there is a problem. Go scans the package directory for `.c` files even when CGo is disabled (like when you do `go build -tags mock`). So if I put the C files next to the Go files, the mock build breaks. To fix this I put all C files in a subdirectory `internal/agent/gpu/` and reference them from the CGo bridge with `#cgo CFLAGS: -I${SRCDIR}/gpu` and `#include "gpu.c"`.

For platform detection, I used `#ifdef __APPLE__` and `#ifdef __linux__` in `gpu.c`. So the same source code detects Apple GPUs on macOS (using sysctl) and scans the PCI bus on Linux for NVIDIA/AMD/Intel. The binary you build on a Mac will detect Apple GPU. The binary you build on Linux will detect whatever GPU is there. One codebase, different behavior at compile time.

I chose to detect only one vendor at a time. Whatever GPU is in the system, the scheduler uses that. No multi-vendor pool for now. If you have NVIDIA it uses NVIDIA, if you have Apple Silicon it uses Apple. First detected wins.

Topology is optional. If there is only 1 GPU (like on Apple Silicon), there is no interconnect to worry about. The scheduler just uses that one GPU directly. For multi-GPU systems it looks at the links between GPUs (NVLink vs PCIe) and picks the best group.

For the fake data, I kept `gpu_fake.c` which has 4x A100 with NVLink and PCIe links. This is used for testing the scheduler logic without real hardware. You can run it with `make mock` which builds with `-tags mock` and uses pure Go, no C needed.
