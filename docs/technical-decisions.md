---
title: Technical Decisions
nav_order: 6
---

# Technical Decisions

this doc explains why i built things the way i did. if you are curious about the internals or want to contribute, read this first.

## CGo File Layout

when building with Go + CGo, there is a problem. Go scans the package directory for `.c` files even when CGo is disabled (like when you do `go build -tags mock`). so if i put the C files next to the Go files, the mock build breaks.

to fix this i put all C files in a subdirectory `internal/agent/gpu/` and reference them from the CGo bridge with:

```go
#cgo CFLAGS: -I${SRCDIR}/gpu
#include "gpu.c"
```

this way the mock build (pure Go, no C) works fine and the real build picks up the C files through the bridge.

## Platform Detection

i used `#ifdef __APPLE__` and `#ifdef __linux__` in `gpu.c`. so the same source code detects Apple GPUs on macOS (using `sysctl`) and uses Linux GPU backends for NVIDIA/AMD/Intel. AMD uses ROCm SMI first, then falls back to AMDGPU sysfs so Radeon/APU devices can still be detected and monitored when ROCm does not expose them.

the binary you build on a Mac will detect Apple GPU. the binary you build on Linux will detect whatever GPU is there. one codebase, different behavior at compile time.

## Single Vendor at a Time

i chose to detect only one vendor at a time. whatever GPU is in the system, the scheduler uses that. no multi-vendor pool for now.

if you have NVIDIA it uses NVIDIA, if you have Apple Silicon it uses Apple. first detected wins.

this simplifies the env var logic (CUDA_VISIBLE_DEVICES vs HIP_VISIBLE_DEVICES vs GPU_DEVICE_ORDINAL) and the topology scoring. mixing vendors in one pool would need a lot more complexity.

## Topology is Optional

if there is only 1 GPU (like on Apple Silicon), there is no interconnect to worry about. the scheduler just uses that one GPU directly.

for multi-GPU systems it looks at the links between GPUs (NVLink vs PCIe) and picks the best group. but if topology detection fails, the scheduler still works - it just treats all GPUs as equal.

## Fake GPU Data

i kept `gpu_fake.c` which has 4x A100 with NVLink and PCIe links. this is used for testing the scheduler logic without real hardware.

you can run it with `make mock` (or `make cli-mock`) which builds with `-tags mock` and uses pure Go, no C needed. this is also how the tests run in CI where there are no real GPUs.

## State Struct Design

i needed one place to hold everything - the topology, the queue, which GPUs are allocated, and what jobs are running.

i used maps for the allocated GPUs (`gpuID -> jobID`) so checking if a GPU is free is just a map lookup. O(1).

everything is protected by a mutex because the scheduler loop runs in a goroutine and you dont want race conditions when allocating GPUs. i could have used channels but mutex is simpler for this case since most operations are reads with occasional writes.

## Process Launcher and Env Vars

the key thing is `CUDA_VISIBLE_DEVICES`. when you set this env var, CUDA only sees the GPUs you specify. so if i allocate GPU 0 and 1, i set `CUDA_VISIBLE_DEVICES=0,1` and the process only uses those two.

AMD has `HIP_VISIBLE_DEVICES` and Intel has `GPU_DEVICE_ORDINAL`, same idea. for Apple i dont set anything because there is only one GPU and Metal just uses it.

i used `Setpgid: true` when starting processes so each job gets its own process group. this matters for the kill command - i can kill the whole group with one `syscall.Kill(-pid, SIGKILL)` instead of hunting for child processes.

the launch function is async. it starts the process and returns immediately. a goroutine waits for the process to exit and calls the onDone callback. this way the scheduler can keep processing other jobs while one is running.

## CLI Design

i used a global state variable so the run, status, and kill commands can share the same state. no external deps, just `os.Args` and `flag` package.

the run command parses flags before the `--` separator and everything after `--` is the user command. so `gpusched run --gpus 2 -- python train.py` splits into flags (`gpus=2`) and command (`python train.py`).

the VRAM flag accepts human readable formats like `40g`, `512m`, `1000`. i wrote a parser that handles the suffixes and converts to MB.

the run command blocks until the job finishes. it submits the job to the state, the scheduler loop picks it up, and run waits on a channel for the done callback. this way the user sees the output and the shell waits for the process.

## Scheduler Loop

i used a goroutine with a 1-second ticker. each tick it checks if the top job in the queue can be placed.

i used `PeekJob` to look at the top without popping, so if there arent enough free GPUs the job stays in the queue. only when the scorer returns a valid group do i pop the job and launch it.

the loop also stores the process reference (`exec.Cmd`) so we can kill it later.

## Signal Handling

i catch SIGINT and SIGTERM in `main.go` with a goroutine that waits on a signal channel. when it fires, it calls `state.Stop()` which kills all running process groups and exits cleanly.

for the kill command, i used `syscall.Kill(-pid, SIGKILL)` with the negative PID to kill the entire process group, not just the parent. this is why i used `Setpgid: true` when launching processes - it creates the process group that we can kill later.

## Desktop App - Wails Choice

i used Wails v2 which lets you build desktop apps with Go backend and any web frontend. i went with Svelte because it compiles to vanilla JS so the bundle is tiny, and Tailwind CSS for styling.

every exported method in `app.go` becomes callable from the Svelte frontend via Wails bindings. no REST API needed, the bridge handles everything.

the desktop app embeds the built Svelte dist folder into the Go binary so it ships as one executable. no need to distribute frontend assets separately.

## Cluster Architecture

for multi-node support, each remote machine runs a lightweight agent server (HTTP on port 9712). the main app connects over SSH, deploys the agent, and talks to it through port forwarding.

the agent server exposes endpoints for submitting jobs, checking status, getting logs, and monitoring the machine. the cluster scheduler on the main app coordinates across all agents.

file sync uses rsync over SSH. before dispatching a job, the scheduler rsyncs the local source directory to the remote node so the code is always up to date.

the desktop app saves node configs to `desktop-config.json` so you can reconnect to previously configured nodes without re-entering SSH details.
