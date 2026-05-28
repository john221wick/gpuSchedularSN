package agentserver

import "github.com/john221wick/gpuSchedularSN/internal/agent"

type TopologyResponse struct {
	Devices []agent.GPUDevice `json:"devices"`
	Links   []agent.GPULink   `json:"links"`
}

type JobSpec struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	GPUIDs  []int  `json:"gpuIDs"`
	WorkDir string `json:"workDir"`
}

type JobStatusResponse struct {
	ID        string `json:"id"`
	Command   string `json:"command"`
	GPUIDs    []int  `json:"gpuIDs"`
	PID       int    `json:"pid"`
	Status    string `json:"status"` // "running", "done", "failed"
	StartedAt string `json:"startedAt"`
	ExitCode  int    `json:"exitCode"`
}

type StatusResponse struct {
	Jobs []JobStatusResponse `json:"jobs"`
}

type LogChunk struct {
	Data   string `json:"data"`
	Offset int64  `json:"offset"`
	EOF    bool   `json:"eof"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
