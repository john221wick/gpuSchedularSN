---
title: Desktop App
nav_order: 5
---

# Desktop App

the desktop app is built with Wails v2 (Go backend + Svelte 5 frontend). it wraps everything in a native window and bridges Go methods to JavaScript. no REST API needed, the Wails bridge handles everything.

every exported method in `app.go` becomes callable from the Svelte frontend.

## Pages

the frontend has these pages:

1. **Dashboard** - GPU stats, running jobs, VRAM usage. auto-refreshes every 2 seconds
2. **Devices** - list of all GPUs with allocation status
3. **Topology** - bandwidth matrix and link types
4. **Jobs** - running, completed, and queued jobs
5. **Submit** - submit a new job with GPU count, VRAM, priority
6. **Nodes** - manage cluster nodes (connect, disconnect, reconnect)
7. **Monitor** - per-node CPU, memory, GPU, Docker containers, processes
8. **Terminal** - interactive SSH shell to remote nodes
9. **Logs** - view job output with incremental loading

dark and light theme toggle, stored in localStorage.

## Local Mode

in local mode, the app manages GPUs on your machine.

### Devices

- `GetDevices()` - returns all GPUs with allocation status, VRAM, utilization, temperature
- `GetFreeDevices()` - returns only unallocated GPUs

### Topology

- `GetTopology()` - returns bandwidth matrix and link info between GPUs

### Jobs

- `SubmitJob(req)` - submits a job to the local scheduler queue. returns job ID
  - `req.Command` - the command to run
  - `req.PathVariable` - optional path to resolve relative Python scripts
  - `req.NumGPUs` - number of GPUs needed
  - `req.MinVRAMMB` - minimum VRAM per GPU in MB
  - `req.Priority` - lower = higher priority
- `GetRunningJobs()` - all currently running jobs
- `GetCompletedJobs()` - all done/failed jobs
- `GetQueuedJobs()` - jobs waiting in queue
- `GetQueueLength()` - count of queued jobs
- `KillJob(jobID)` - kills a running job (SIGKILL process tree)
- `RemoveQueuedJob(jobID)` - removes a job from queue before it starts
- `UpdateQueuedPriority(jobID, newPriority)` - changes priority of a queued job

### Dashboard

- `GetDashboard()` - returns summary: total/free GPUs, running/queued jobs, avg utilization, VRAM usage

### Command Normalization

when you submit a job, the app normalizes the command:
- relative Python scripts like `train.py` get resolved to absolute paths
- `python train.py` becomes `python3 /absolute/path/to/train.py`
- the path variable helps resolve scripts relative to a specific directory

## Cluster Mode

toggle cluster mode with `SetRemoteMode(true)`. this initializes the NodeManager and ClusterScheduler.

### Node Management

- `ConnectNode(sshCommand, keyPath, mockMode)` - the big one. it:
  1. SSH connects to the remote machine
  2. auto-detects GPUs, arch, and OS
  3. cross-compiles the agent binary and deploys it
  4. starts the agent on port 9712
  5. sets up port forwarding
  6. adds the node to the cluster
  7. recovers any jobs from a previous session still running on remote
  8. saves the node config so you can reconnect later
- `ReconnectNode(nodeID)` - reconnects using saved SSH config
- `DisconnectNode(nodeID)` - closes SSH, removes from live cluster (keeps saved config)
- `RemoveNode(nodeID)` - permanently removes saved config and disconnects
- `GetNodes()` - all nodes in the cluster (connected or not)
- `GetSavedNodes()` - saved node configs that are not currently connected
- `SetNodePaths(nodeID, localDir, remoteDir)` - sets local source dir and remote destination dir for file sync

### Cluster Jobs

- `ClusterSubmitJob(req)` - submits to cluster scheduler. it picks the best node + GPU combination across all connected nodes
- `GetClusterRunningJobs()` - jobs running across the cluster
- `GetClusterQueuedJobs()` - jobs waiting in cluster queue
- `GetClusterCompletedJobs()` - completed cluster jobs
- `ClusterKillJob(jobID)` - kills a running cluster job (sends DELETE to remote agent)

### Cluster Dashboard

- `GetClusterDashboard()` - cluster-wide summary: total GPUs across all nodes, free GPUs, job counts, utilization

### Cluster Devices and Topology

- `GetClusterDevices()` - all GPU devices across all nodes, with node ID and name
- `GetClusterTopology()` - per-node bandwidth matrix and links

## Monitoring

- `GetClusterMonitor()` - polls every connected node's `/monitor` endpoint and returns:
  - host stats (hostname, OS, kernel, CPU model, uptime, CPU%, memory, load average, per-core CPU)
  - GPU device stats (live refresh)
  - Docker container stats (if Docker is running)
  - top OS processes by CPU/memory
  - GPU processes (via nvidia-smi)

per-node failures are reported inline (`Reachable=false`) so one dead node does not hide the others.

## File Sync

- `SyncFilesToNode(nodeID, localPath)` - rsyncs a local directory to the node's remote destination directory. uses the SSH session's config for auth.

when cluster mode is enabled, there is also an automatic transfer function that rsyncs your local dir to the remote node before dispatching a job. you set the local and remote dirs with `SetNodePaths`.

## Logs

- `GetJobLogs(jobID, offset)` - fetches job output from the correct cluster node. supports incremental reads via offset. returns data, new offset, and EOF flag.

it first checks the scheduler to find which node owns the job. if not found, it tries all nodes.

## Terminal (SSH PTY)

the desktop app has a built-in terminal that opens an interactive SSH shell to any connected node.

- `StartTerminalSession(nodeID, cols, rows)` - opens a PTY session. returns a session ID. output is emitted via Wails events to the frontend
- `WriteTerminalInput(sessionID, data)` - sends base64-encoded keystrokes to the PTY
- `ResizeTerminal(sessionID, cols, rows)` - sends a window resize event
- `StopTerminalSession(sessionID)` - closes the PTY session
- `RunTerminalCommand(nodeID, command)` - runs a one-shot command over SSH and returns output (no PTY)

## App Logs

- `GetAppLogs(offset)` - returns application log entries from a ring buffer (captured stdout)
- `GetAppLogCount()` - total number of log entries in the buffer

## Mode Toggle

- `SetRemoteMode(enabled)` - enables or disables cluster mode. when enabled, it initializes NodeManager, starts heartbeat, and starts the cluster scheduler loop
- `GetRemoteMode()` - returns whether cluster mode is active

the mode preference is saved to `desktop-config.json` so it persists across restarts.
