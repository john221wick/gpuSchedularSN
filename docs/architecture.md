---
title: Architecture
nav_order: 3
---

# Architecture

this doc explains how the scheduler works from the inside. if you just want to use it, see [getting-started.md](getting-started.md).

## Overview

the scheduler has these main pieces:

1. **GPU Detection** - finds what GPUs you have and how they are connected
2. **Topology Matrix** - a bandwidth matrix showing GPU-to-GPU link speeds
3. **Scorer** - picks the best group of N GPUs using backtracking
4. **Queue** - holds jobs waiting for GPUs
5. **State** - one central struct that ties everything together, thread safe with a mutex
6. **Runner** - launches your command with the right GPU env vars
7. **Scheduler Loop** - a background goroutine that checks the queue every second and places jobs

## GPU Detection

the agent package detects GPUs at startup. it uses CGo to call C code that reads hardware info.

on macOS it uses `sysctl` to find Apple Silicon GPUs. on Linux it scans the PCI bus for NVIDIA, AMD, and Intel GPUs.

it only detects one vendor at a time. whatever GPU is in the system first, that is what the scheduler uses. no multi-vendor pool. if you have NVIDIA it uses NVIDIA, if you have Apple Silicon it uses Apple.

each GPU gets these fields:
- `ID` - index (0, 1, 2, ...)
- `Vendor` - NVIDIA, AMD, Intel, or Apple
- `Name` - like "A100 80GB" or "Apple M1 Max"
- `VRAMTotalMB` - total VRAM in megabytes
- `VRAMUsedMB` - currently used VRAM
- `UtilizationPct` - GPU utilization percentage
- `TemperatureC` - temperature in celsius
- `VendorIndex` - the vendor's own device index (for CUDA_VISIBLE_DEVICES etc)

## Topology Matrix

the topology is basically a matrix of bandwidth between every pair of GPUs. it also tracks what type of link connects them.

supported link types:
- **NVLink** - NVIDIA GPU-to-GPU, up to 600 GB/s
- **NVSwitch** - NVIDIA switch-based, even faster
- **PCIe** - standard bus, slower than NVLink
- **XGMI** - AMD Infinity Fabric
- **XeLink** - Intel discrete GPU link
- **Thunderbolt** - Apple interconnect
- **UnifiedMemory** - Apple Silicon shared memory (single GPU, no interconnect needed)

if there is only 1 GPU (like on Apple Silicon), there is no topology to worry about. the scheduler just uses that one GPU directly.

## Scorer

the scorer finds the best combination of N GPUs from the topology matrix. it uses backtracking to try all possible groups and picks the one with the highest total bandwidth.

for example, if you ask for 2 GPUs and you have 4 GPUs connected with NVLink and PCIe, the scorer will pick the pair connected by NVLink because it has higher bandwidth.

it also checks VRAM - if you ask for `--vram 40g`, it skips any GPU that has less than 40GB.

the algorithm is basically:
1. generate all combinations of N GPUs from the available pool
2. for each combination, sum up the bandwidth between all pairs
3. return the combination with the highest score

## Queue

the queue holds jobs that are waiting for GPUs. it is a priority queue where lower number = higher priority.

when you submit a job, it goes into the queue. the scheduler loop checks the queue every second and if there is a job waiting and enough free GPUs, it picks it up and launches it.

## State

the state struct is the central place that holds everything:

- the topology (bandwidth matrix + links)
- the queue (jobs waiting)
- running jobs (map of jobID -> Job)
- completed jobs (map of jobID -> Job)
- allocated GPUs (map of gpuID -> jobID)

everything is protected by a mutex because the scheduler loop runs in a goroutine and you dont want race conditions when allocating GPUs.

checking if a GPU is free is just a map lookup on the allocated GPUs map.

## Runner

the runner launches your command as a child process. the key thing it does is set GPU environment variables:

- `CUDA_VISIBLE_DEVICES` for NVIDIA
- `HIP_VISIBLE_DEVICES` for AMD
- `GPU_DEVICE_ORDINAL` for Intel
- nothing for Apple (only one GPU, Metal handles it)

so if i allocate GPU 0 and 1, i set `CUDA_VISIBLE_DEVICES=0,1` and the process only sees those two GPUs.

each process runs in its own process group (`Setpgid: true`). this matters for killing - i can kill the whole group with one `syscall.Kill(-pid, SIGKILL)` instead of hunting for child processes.

the launch function is async. it starts the process and returns immediately. a goroutine waits for the process to exit and calls the onDone callback. this way the scheduler can keep processing other jobs while one is running.

## Scheduler Loop

a goroutine with a 1-second ticker. each tick it:

1. peeks at the top job in the queue (without popping)
2. checks if there are enough free GPUs
3. runs the scorer to find the best GPU group
4. if the scorer returns a valid group, pops the job, allocates GPUs, and launches it
5. if not enough GPUs, the job stays in the queue for next tick

the loop also stores the process reference (`exec.Cmd`) so we can kill it later.

## Signal Handling

when you press Ctrl+C, the scheduler catches SIGINT and SIGTERM. it calls `state.Stop()` which kills all running process groups and exits cleanly. no orphan processes left behind.

for the kill command, it uses `syscall.Kill(-pid, SIGKILL)` with the negative PID to kill the entire process group, not just the parent process.

## Cluster Mode

the scheduler can also work across multiple machines. in cluster mode:

1. a **NodeManager** keeps track of connected remote nodes
2. each node runs an **agent server** (HTTP on port 9712)
3. the **ClusterScheduler** picks the best node + GPU combination across the cluster
4. SSH is used to deploy the agent binary, start it, and set up port forwarding
5. file sync uses rsync to push your code to the remote node before running

the desktop app can toggle between local mode and cluster mode. in cluster mode you get additional pages for managing nodes, cluster-wide dashboards, and remote terminals.
