---
title: CLI Reference
nav_order: 4
---

# CLI Reference

the CLI binary is `gpusched`. usage: `gpusched <command> [flags]`

## devices

lists all detected GPUs on your machine.

```bash
gpusched devices
```

output:
```
Detected 4 GPU(s):
  [0] A100 80GB  81920MB  vendor:NVIDIA
  [1] A100 80GB  81920MB  vendor:NVIDIA
  [2] A100 80GB  81920MB  vendor:NVIDIA
  [3] A100 80GB  81920MB  vendor:NVIDIA
```

## topo

shows the GPU topology - bandwidth matrix and link types between GPUs.

```bash
gpusched topo
```

output:
```
Topology (4 GPUs):

         GPU0   GPU1   GPU2   GPU3
  GPU0    ---    600    200    200
  GPU1    600    ---    200    200
  GPU2    200    200    ---    600
  GPU3    200    200    600    ---

Links:
  GPU0 <-> GPU1  NVLink  600 GB/s
  GPU2 <-> GPU3  NVLink  600 GB/s
  GPU0 <-> GPU2  PCIe    200 GB/s
  GPU1 <-> GPU3  PCIe    200 GB/s
```

if you only have 1 GPU, it just says `1 GPU (Apple M1 Max), no topology`.

## run

runs a command with GPU allocation. this is the main command.

```bash
gpusched run --gpus N [--vram X] [--priority P] -- command [args...]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--gpus` | `1` | number of GPUs to allocate |
| `--vram` | `0` | minimum VRAM per GPU. accepts `40g`, `512m`, `1000` (raw MB) |
| `--priority` | `10` | job priority. lower number = higher priority |

### Examples

```bash
# run on 1 GPU (default)
gpusched run -- python train.py

# run on 2 GPUs with 40GB VRAM each
gpusched run --gpus 2 --vram 40g -- python train.py

# run with high priority
gpusched run --gpus 4 --priority 1 -- torchrun --nproc_per_node=4 train.py
```

the `--` separator is important. everything before it is flags for gpusched, everything after is your command.

the command blocks until your process exits. the scheduler picks the best GPU group based on topology scoring and sets the right env vars (CUDA_VISIBLE_DEVICES, HIP_VISIBLE_DEVICES, etc).

### VRAM Formats

| Format | Meaning |
|--------|---------|
| `40g` or `40gb` | 40,000 MB |
| `512m` or `512mb` | 512 MB |
| `1000` | 1000 MB (raw number = MB) |

## status

shows running and queued jobs.

```bash
gpusched status
```

output:
```
Running (1):
  job-1717123456789  python train.py  GPUs:[0 1]
Queued: 2
```

if nothing is running or queued, it says `No jobs running or queued`.

## kill

kills a running job by its ID.

```bash
gpusched kill <jobID>
```

it kills the entire process group (not just the parent process) using `syscall.Kill(-pid, SIGKILL)`.

## connect

connects to a remote agent node over SSH or direct TCP.

```bash
# SSH connection
gpusched connect "ssh -p 22 user@host" [--key ~/.ssh/id_rsa]

# direct TCP (for testing, no SSH)
gpusched connect --direct host:9712
```

### What it does

1. SSH connects to the remote machine
2. creates `~/gpuschedular/logs` on the remote
3. installs rsync if not present
4. cross-compiles the agent binary for the remote arch and SCPs it over
5. starts the agent on port 9712
6. sets up SSH port forwarding so you can reach the agent locally

### Flags

| Flag | Description |
|------|-------------|
| `--direct host:port` | skip SSH, connect directly to an already-running agent |
| `--key path` | path to SSH private key |

## nodes

lists all connected cluster nodes.

```bash
gpusched nodes
```

output:
```
Nodes (2):
  gpu-server-1          ssh-10.0.0.1-22  4 GPUs  connected
  gpu-server-2          ssh-10.0.0.2-22  8 GPUs  connected
```

## logs

views job output. works for both local and cluster jobs.

```bash
gpusched logs <jobID> [--follow]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--follow` | `false` | keep reading new output as it arrives (like `tail -f`) |

for cluster jobs, it automatically finds which node has the job and fetches logs from there. it polls every 500ms when following.

## version

prints the version.

```bash
gpusched version
```

output:
```
gpusched 0.1.0
```
