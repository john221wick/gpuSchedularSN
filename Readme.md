# GPU Schedular Single Node

The goal of this project is to make gpu schedular which would save waste of gpu resources via a single exectable that can handle mulitple Interconnects(NVLink, NVSwitch, Xe Link etc).


# Phase 1 done
1. Added gpu detection - Now it auto detects the gpu on `make run`, so if you want to know about your own computer, just type make run. It will work
2. Fake gpu - For running in Nvidia gpus, i have added **gpu_fake.c** file, which containes four A100(80GB) and there is 2 NVLink, and 4 pcie connection
3. Right now - I am running in macos so there is gpu detection in the beginning itself

# Phase 2 and 3 done

1. I added and tested topology matrix, which is basically for gpus which are made basically about connection of gpus to gpus
2. Added a scorer, which basically finds the best combination of n gpus from the topology matrix with backtracking

# Phase 4 done

1. Added queue to handle the ongoing requests, for now the priority is set by int, lateron i will fix that

# Phase 5 done

1. Added a central state struct that ties everything together - topology, queue, running jobs, and which GPUs are allocated. All thread safe with a mutex
2. You can submit a job to the queue, allocate GPUs to it, free them when done, and check which GPUs are free

# Phase 6 done

1. Added a process launcher that actually runs your command. It sets the right env vars based on your GPU vendor - CUDA_VISIBLE_DEVICES for NVIDIA, HIP_VISIBLE_DEVICES for AMD, etc
2. For Apple it doesnt set anything since there is only one GPU and Metal handles it
3. Each process runs in its own process group so we can kill it cleanly later (no orphan processes)
4. It runs async - starts the process and calls a callback when it exits, so the scheduler can keep working while a job is running

# Phase 7 done

1. Added CLI with subcommands - `gpusched devices`, `topo`, `run`, `status`, `kill`, `version`. No external deps, just os.Args and flag package
2. The `run` command parses flags before `--` and everything after is the user command. So `gpusched run --gpus 2 -- python train.py` splits into flags (gpus=2) and command (python train.py)
3. The VRAM flag accepts human readable formats like 40g, 512m, 1000 - converts to MB internally

# Technical Decisions i took

When building with Go + CGo, there is a problem. Go scans the package directory for `.c` files even when CGo is disabled (like when you do `go build -tags mock`). So if I put the C files next to the Go files, the mock build breaks. To fix this I put all C files in a subdirectory `internal/agent/gpu/` and reference them from the CGo bridge with `#cgo CFLAGS: -I${SRCDIR}/gpu` and `#include "gpu.c"`.

For platform detection, I used `#ifdef __APPLE__` and `#ifdef __linux__` in `gpu.c`. So the same source code detects Apple GPUs on macOS (using sysctl) and scans the PCI bus on Linux for NVIDIA/AMD/Intel. The binary you build on a Mac will detect Apple GPU. The binary you build on Linux will detect whatever GPU is there. One codebase, different behavior at compile time.

I chose to detect only one vendor at a time. Whatever GPU is in the system, the scheduler uses that. No multi-vendor pool for now. If you have NVIDIA it uses NVIDIA, if you have Apple Silicon it uses Apple. First detected wins.

Topology is optional. If there is only 1 GPU (like on Apple Silicon), there is no interconnect to worry about. The scheduler just uses that one GPU directly. For multi-GPU systems it looks at the links between GPUs (NVLink vs PCIe) and picks the best group.

For the fake data, I kept `gpu_fake.c` which has 4x A100 with NVLink and PCIe links. This is used for testing the scheduler logic without real hardware. You can run it with `make mock` which builds with `-tags mock` and uses pure Go, no C needed.

For the state struct, I needed one place to hold everything - the topology, the queue, which GPUs are allocated, and what jobs are running. I used maps for the allocated GPUs (gpuID → jobID) so checking if a GPU is free is just a map lookup. Everything is protected by a mutex because later the scheduler loop runs in a goroutine and you dont want race conditions when allocating GPUs.

For the runner, the key thing is CUDA_VISIBLE_DEVICES. When you set this env var, CUDA only sees the GPUs you specify. So if I allocate GPU 0 and 1, I set CUDA_VISIBLE_DEVICES=0,1 and the process only uses those two. AMD has HIP_VISIBLE_DEVICES and Intel has GPU_DEVICE_ORDINAL, same idea. For Apple I dont set anything because there is only one GPU and Metal just uses it. I used Setpgid: true when starting processes so each job gets its own process group. This matters for the kill command later - I can kill the whole group with one syscall.Kill(-pid, SIGKILL) instead of hunting for child processes. The launch function is async - it starts the process and returns immediately. A goroutine waits for the process to exit and calls the onDone callback. This way the scheduler can keep processing other jobs while one is running.

For the CLI I used a global state variable so the run, status, and kill commands can share the same state. The run command parses flags before the -- separator and everything after -- is the user command. The VRAM flag accepts human readable formats like 40g, 512m, 1000. I wrote a parser that handles the suffixes and converts to MB. The run command blocks until the job finishes. It submits the job to the state, the scheduler loop picks it up, and run waits on a channel for the done callback. This way the user sees the output and the shell waits for the process.
