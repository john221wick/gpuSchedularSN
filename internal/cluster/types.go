package cluster

import (
	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

type NodeStatus int

const (
	NodeConnected NodeStatus = iota
	NodeDisconnected
	NodeConnecting
)

func (s NodeStatus) String() string {
	switch s {
	case NodeConnected:
		return "connected"
	case NodeDisconnected:
		return "disconnected"
	case NodeConnecting:
		return "connecting"
	default:
		return "unknown"
	}
}

type Node struct {
	ID        string
	Name      string
	AgentURL  string
	Status    NodeStatus
	LocalDir  string // local source directory to rsync FROM (e.g. /Users/me/myproject)
	RemoteDir string // remote destination directory to rsync TO (e.g. /root/myproject)
	GPUVendor string // nvidia, amd, intel, none
	GPUName   string // e.g. "NVIDIA A100-SXM4-80GB"
	Arch      string // x86_64, aarch64
	OS        string // e.g. "Ubuntu 22.04"
	Devices   []agent.GPUDevice
	Links     []agent.GPULink
	Topology  *scheduler.Topology
}

type ClusterJob struct {
	scheduler.Job
	NodeID  string
	WorkDir string
}
