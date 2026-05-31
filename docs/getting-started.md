---
title: Getting Started
nav_order: 2
---

# Getting Started

gpuSchedularSN is a single-node GPU scheduler written in Go. you can use it as a CLI tool or as a desktop app with a GUI.

## Prerequisites

1. Go 1.21+
2. Make
3. For desktop app: Wails v2, Svelte 5, pnpm
4. For NVIDIA GPUs: CUDA toolkit (for nvidia-smi)
5. For AMD GPUs: ROCm

## Building the CLI

```bash
# real GPU detection (needs CGo on Linux for NVIDIA/AMD)
make cli

# mock mode - no CGo, uses fake 4x A100 GPUs for testing
make cli-mock
```

the binary goes to `build/gpusched` (or `build/gpusched_mock`).

## Building the Desktop App

```bash
# builds frontend (Svelte) then wraps it in a native window via Wails
make desktop

# dev mode with hot reload
make desktop-dev

# dev mode with mock GPUs
make desktop-mock
```

the desktop binary goes to `build/gpusched-desktop`.

## Quick Start - CLI

```bash
# see what GPUs you have
./build/gpusched devices

# see how GPUs are connected to each other
./build/gpusched topo

# run a command on 2 GPUs
./build/gpusched run --gpus 2 -- python train.py

# run with VRAM requirement (40GB per GPU)
./build/gpusched run --gpus 2 --vram 40g -- python train.py

# check what is running
./build/gpusched status

# kill a job
./build/gpusched kill job-1234567890

# see job output
./build/gpusched logs job-1234567890

# follow logs in real time
./build/gpusched logs job-1234567890 --follow
```

## Quick Start - Desktop

1. run `make desktop-dev`
2. the app opens with pages for Dashboard, Devices, Topology, Jobs, and Submit
3. dashboard auto-refreshes every 2 seconds
4. you can toggle dark/light theme, it saves in localStorage

## Running Tests

```bash
# standard tests (skips desktop package)
make test

# tests with mock GPUs
make test-mock
```

## Project Structure

```
cmd/                    # entry points (CLI main.go, desktop/)
internal/
  agent/                # GPU detection via CGo
    gpu/                # C source files (gpu.c, gpu_fake.c)
  agentserver/          # HTTP server that agents run on remote nodes
  cli/                  # CLI subcommands
  cluster/              # multi-node cluster management, SSH, scheduler
  desktop/              # Wails app bridge (Go <-> Svelte)
  runner/               # process launcher with GPU env vars
  scheduler/            # topology scoring and job queue
  state/                # central thread-safe state struct
frontend/               # Svelte 5 + Tailwind CSS frontend
docs/                   # you are here
```
