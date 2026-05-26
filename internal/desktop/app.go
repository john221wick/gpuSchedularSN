package desktop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
	"github.com/john221wick/gpuSchedularSN/internal/state"
)

// App is the Wails application bridge.
// Every exported method is callable from the frontend via Wails bindings.
type App struct {
	ctx   context.Context
	state *state.State
}

func NewApp() *App {
	return &App{}
}

// Startup is called by Wails when the app starts.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	agent.Init()
	devices := agent.GetDevices()
	links := agent.GetTopology()
	a.state = state.NewState(devices, links)
	a.state.StartSchedulerLoop()
}

// Shutdown is called by Wails when the app closes.
func (a *App) Shutdown(ctx context.Context) {
	if a.state != nil {
		a.state.Stop()
	}
	agent.Shutdown()
}

// --- DTOs (JSON-friendly structs for frontend) ---

type DeviceInfo struct {
	ID             int     `json:"id"`
	Vendor         string  `json:"vendor"`
	Name           string  `json:"name"`
	VRAMTotalMB    uint64  `json:"vramTotalMB"`
	VRAMUsedMB     uint64  `json:"vramUsedMB"`
	UtilizationPct float32 `json:"utilizationPct"`
	TemperatureC   int     `json:"temperatureC"`
	Allocated      bool    `json:"allocated"`
	AllocatedTo    string  `json:"allocatedTo"`
}

type TopologyInfo struct {
	NumGPUs   int         `json:"numGPUs"`
	Bandwidth [][]float32 `json:"bandwidth"`
	Links     []LinkInfo  `json:"links"`
}

type LinkInfo struct {
	GPUA          int     `json:"gpuA"`
	GPUB          int     `json:"gpuB"`
	Type          string  `json:"type"`
	BandwidthGBps float32 `json:"bandwidthGBps"`
}

type JobInfo struct {
	ID          string `json:"id"`
	Command     string `json:"command"`
	NumGPUs     int    `json:"numGPUs"`
	MinVRAMMB   uint64 `json:"minVRAMMB"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submittedAt"`
	StartedAt   string `json:"startedAt"`
	GPUIDs      []int  `json:"gpuIDs"`
}

type SubmitRequest struct {
	Command   string `json:"command"`
	NumGPUs   int    `json:"numGPUs"`
	MinVRAMMB uint64 `json:"minVRAMMB"`
	Priority  int    `json:"priority"`
}

type DashboardInfo struct {
	TotalGPUs    int     `json:"totalGPUs"`
	FreeGPUs     int     `json:"freeGPUs"`
	RunningJobs  int     `json:"runningJobs"`
	QueuedJobs   int     `json:"queuedJobs"`
	AvgUtil      float32 `json:"avgUtil"`
	TotalVRAMMB  uint64  `json:"totalVRAMMB"`
	UsedVRAMMB   uint64  `json:"usedVRAMMB"`
}

// --- Device methods ---

func (a *App) GetDevices() []DeviceInfo {
	devices := a.state.Devices()
	result := make([]DeviceInfo, len(devices))
	for i, d := range devices {
		allocated := !a.state.IsGPUFree(d.ID)
		result[i] = DeviceInfo{
			ID:             d.ID,
			Vendor:         d.Vendor.String(),
			Name:           d.Name,
			VRAMTotalMB:    d.VRAMTotalMB,
			VRAMUsedMB:     d.VRAMUsedMB,
			UtilizationPct: d.UtilizationPct,
			TemperatureC:   d.TemperatureC,
			Allocated:      allocated,
		}
	}
	return result
}

func (a *App) GetFreeDevices() []DeviceInfo {
	devices := a.state.GetFreeDevices()
	result := make([]DeviceInfo, len(devices))
	for i, d := range devices {
		result[i] = DeviceInfo{
			ID:             d.ID,
			Vendor:         d.Vendor.String(),
			Name:           d.Name,
			VRAMTotalMB:    d.VRAMTotalMB,
			VRAMUsedMB:     d.VRAMUsedMB,
			UtilizationPct: d.UtilizationPct,
			TemperatureC:   d.TemperatureC,
			Allocated:      false,
		}
	}
	return result
}

// --- Topology methods ---

func (a *App) GetTopology() TopologyInfo {
	topo := a.state.Topology()
	devices := a.state.Devices()
	n := len(devices)

	bw := make([][]float32, n)
	for i := 0; i < n; i++ {
		bw[i] = make([]float32, n)
		if topo != nil && i < len(topo.Bandwidth) {
			copy(bw[i], topo.Bandwidth[i])
		}
	}

	links := agent.GetTopology()
	linkInfos := make([]LinkInfo, len(links))
	for i, l := range links {
		linkInfos[i] = LinkInfo{
			GPUA:          l.GPUA,
			GPUB:          l.GPUB,
			Type:          l.Type.String(),
			BandwidthGBps: l.BandwidthGBps,
		}
	}

	return TopologyInfo{
		NumGPUs:   n,
		Bandwidth: bw,
		Links:     linkInfos,
	}
}

// --- Job methods ---

func (a *App) GetRunningJobs() []JobInfo {
	running := a.state.RunningJobs()
	result := make([]JobInfo, 0, len(running))
	for _, job := range running {
		result = append(result, jobToInfo(job))
	}
	return result
}

func (a *App) GetQueueLength() int {
	return a.state.QueueLen()
}

func (a *App) SubmitJob(req SubmitRequest) (string, error) {
	if strings.TrimSpace(req.Command) == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	if req.NumGPUs < 1 {
		req.NumGPUs = 1
	}
	if req.Priority < 1 {
		req.Priority = 10
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := &scheduler.Job{
		ID:          jobID,
		Command:     req.Command,
		NumGPUs:     req.NumGPUs,
		MinVRAMMB:   req.MinVRAMMB,
		Priority:    req.Priority,
		Status:      scheduler.Queued,
		SubmittedAt: time.Now(),
	}

	a.state.SubmitJob(job)
	return jobID, nil
}

func (a *App) KillJob(jobID string) error {
	return a.state.KillJob(jobID)
}

// --- Dashboard ---

func (a *App) GetDashboard() DashboardInfo {
	devices := a.state.Devices()
	freeDevices := a.state.GetFreeDevices()
	running := a.state.RunningJobs()

	var totalVRAM, usedVRAM uint64
	var totalUtil float32
	for _, d := range devices {
		totalVRAM += d.VRAMTotalMB
		usedVRAM += d.VRAMUsedMB
		totalUtil += d.UtilizationPct
	}

	avgUtil := float32(0)
	if len(devices) > 0 {
		avgUtil = totalUtil / float32(len(devices))
	}

	return DashboardInfo{
		TotalGPUs:   len(devices),
		FreeGPUs:    len(freeDevices),
		RunningJobs: len(running),
		QueuedJobs:  a.state.QueueLen(),
		AvgUtil:     avgUtil,
		TotalVRAMMB: totalVRAM,
		UsedVRAMMB:  usedVRAM,
	}
}

func jobToInfo(j *scheduler.Job) JobInfo {
	return JobInfo{
		ID:          j.ID,
		Command:     j.Command,
		NumGPUs:     j.NumGPUs,
		MinVRAMMB:   j.MinVRAMMB,
		Priority:    j.Priority,
		Status:      j.Status.String(),
		SubmittedAt: j.SubmittedAt.Format(time.RFC3339),
		StartedAt:   j.StartedAt.Format(time.RFC3339),
		GPUIDs:      j.GPUIDs,
	}
}
