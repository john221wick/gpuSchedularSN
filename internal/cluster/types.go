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
	ID       string
	Name     string
	AgentURL string
	Status   NodeStatus
	Devices  []agent.GPUDevice
	Links    []agent.GPULink
	Topology *scheduler.Topology
}

type ClusterJob struct {
	scheduler.Job
	NodeID  string
	WorkDir string
}
