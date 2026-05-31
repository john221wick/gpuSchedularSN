---
title: Home
nav_order: 1
layout: home
---

# GPU Schedular Single Node

a single-node GPU scheduler that saves waste of GPU resources. one executable that handles multiple interconnects (NVLink, NVSwitch, XeLink etc).

## What it does

1. auto-detects your GPUs on startup (NVIDIA, AMD, Intel, Apple Silicon)
2. builds a topology matrix showing bandwidth between GPUs
3. scores and picks the best GPU combination for your job using backtracking
4. manages a priority queue for jobs waiting for GPUs
5. launches your command with the right GPU env vars (CUDA_VISIBLE_DEVICES etc)
6. runs a background scheduler loop that places jobs as GPUs free up
7. kills processes cleanly with no orphans on Ctrl+C

## Modes

- **CLI** - `gpusched` binary, run from terminal
- **Desktop App** - Wails v2 + Svelte 5 GUI with dashboard, topology view, job management, cluster mode, remote terminal

## Docs

- [Getting Started](getting-started.md) - build, install, quick start
- [Architecture](architecture.md) - how the scheduler works internally
- [CLI Reference](cli-reference.md) - all commands with flags and examples
- [Desktop App](desktop-app.md) - GUI features, cluster mode, terminal, file sync
- [Technical Decisions](technical-decisions.md) - why things are built the way they are

## Quick Start

```bash
# build CLI
make cli

# see your GPUs
./build/gpusched devices

# run a job on 2 GPUs
./build/gpusched run --gpus 2 -- python train.py
```

## Supported Interconnects

| Type | Vendor | Bandwidth |
|------|--------|-----------|
| NVLink | NVIDIA | up to 600 GB/s |
| NVSwitch | NVIDIA | up to 900 GB/s |
| PCIe | All | up to 64 GB/s |
| XGMI | AMD | up to 64 GB/s |
| XeLink | Intel | up to 42 GB/s |
| Thunderbolt | Apple | up to 10 GB/s |
| UnifiedMemory | Apple | N/A (single GPU) |
