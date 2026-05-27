package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	Command      string `json:"command"`
	PathVariable string `json:"pathVariable"`
	NumGPUs      int    `json:"numGPUs"`
	MinVRAMMB    uint64 `json:"minVRAMMB"`
	Priority     int    `json:"priority"`
}

type DashboardInfo struct {
	TotalGPUs   int     `json:"totalGPUs"`
	FreeGPUs    int     `json:"freeGPUs"`
	RunningJobs int     `json:"runningJobs"`
	QueuedJobs  int     `json:"queuedJobs"`
	AvgUtil     float32 `json:"avgUtil"`
	TotalVRAMMB uint64  `json:"totalVRAMMB"`
	UsedVRAMMB  uint64  `json:"usedVRAMMB"`
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

func (a *App) GetCompletedJobs() []JobInfo {
	completed := a.state.CompletedJobs()
	result := make([]JobInfo, 0, len(completed))
	for _, job := range completed {
		result = append(result, jobToInfo(job))
	}
	return result
}

func (a *App) GetQueueLength() int {
	return a.state.QueueLen()
}

func (a *App) GetQueuedJobs() []JobInfo {
	queued := a.state.QueuedJobs()
	result := make([]JobInfo, 0, len(queued))
	for _, job := range queued {
		result = append(result, jobToInfo(job))
	}
	return result
}

func (a *App) SubmitJob(req SubmitRequest) (string, error) {
	command := strings.TrimSpace(req.Command)
	execCommand, err := normalizeSubmitCommand(command, req.PathVariable)
	if err != nil {
		return "", err
	}
	if command == "" {
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
		Command:     command,
		ExecCommand: execCommand,
		NumGPUs:     req.NumGPUs,
		MinVRAMMB:   req.MinVRAMMB,
		Priority:    req.Priority,
		Status:      scheduler.Queued,
		SubmittedAt: time.Now(),
	}

	a.state.SubmitJob(job)
	return jobID, nil
}

func normalizeSubmitCommand(command string, pathVariable string) (string, error) {
	command = strings.TrimSpace(command)
	pathVariable = strings.TrimSpace(pathVariable)
	if command == "" {
		return "", nil
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", nil
	}

	first := trimTokenQuotes(fields[0])
	if isRelativePythonScript(first) {
		scriptPath, err := resolvePythonScriptPath(first, pathVariable)
		if err != nil {
			return "", err
		}
		expanded := "python3 " + shellQuote(scriptPath)
		if len(fields) > 1 {
			expanded += " " + strings.TrimSpace(command[len(fields[0]):])
		}
		return expanded, nil
	}

	if isPythonCommand(first) && len(fields) > 1 {
		scriptArg := trimTokenQuotes(fields[1])
		if isRelativePythonScript(scriptArg) {
			scriptPath, err := resolvePythonScriptPath(scriptArg, pathVariable)
			if err != nil {
				return "", err
			}
			rest := strings.TrimSpace(command[len(fields[0]):])
			suffix := ""
			if strings.HasPrefix(rest, fields[1]) {
				suffix = strings.TrimSpace(rest[len(fields[1]):])
			}
			expanded := fields[0] + " " + shellQuote(scriptPath)
			if suffix != "" {
				expanded += " " + suffix
			}
			return expanded, nil
		}
	}

	return command, nil
}

func resolvePythonScriptPath(scriptPath string, pathVariable string) (string, error) {
	if pathVariable == "" {
		return resolveRelativeRepoPath(scriptPath)
	}

	basePath, err := resolvePathVariable(pathVariable)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return "", fmt.Errorf("path variable %q is not accessible: %w", pathVariable, err)
	}

	var candidates []string
	if info.IsDir() {
		candidates = append(candidates, filepath.Join(basePath, scriptPath))
	} else {
		if strings.EqualFold(filepath.Base(basePath), filepath.Base(scriptPath)) {
			candidates = append(candidates, basePath)
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(basePath), scriptPath))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not find %s under path variable %s", scriptPath, basePath)
}

func resolvePathVariable(pathVariable string) (string, error) {
	pathVariable = trimTokenQuotes(strings.TrimSpace(pathVariable))
	if pathVariable == "" {
		return "", fmt.Errorf("path variable cannot be empty")
	}

	if strings.HasPrefix(pathVariable, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		if pathVariable == "~" {
			pathVariable = home
		} else if strings.HasPrefix(pathVariable, "~/") {
			pathVariable = filepath.Join(home, pathVariable[2:])
		}
	}

	if filepath.IsAbs(pathVariable) {
		return filepath.Clean(pathVariable), nil
	}

	return resolveRelativeRepoPath(pathVariable)
}

func isPythonCommand(command string) bool {
	name := filepath.Base(command)
	return name == "python" || strings.HasPrefix(name, "python3")
}

func isRelativePythonScript(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "~") {
		return false
	}
	return strings.HasSuffix(strings.ToLower(path), ".py")
}

func trimTokenQuotes(value string) string {
	return strings.Trim(value, `"'`)
}

func resolveRelativeRepoPath(relativePath string) (string, error) {
	relativePath = filepath.Clean(relativePath)
	if filepath.IsAbs(relativePath) {
		return relativePath, nil
	}
	if relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("relative path %q must stay inside the project", relativePath)
	}

	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}

	seen := make(map[string]bool)
	for _, start := range starts {
		dir, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for {
			if !seen[dir] {
				seen[dir] = true
				candidate := filepath.Join(dir, relativePath)
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("could not find %s", relativePath)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (a *App) RemoveQueuedJob(jobID string) error {
	return a.state.RemoveQueuedJob(jobID)
}

func (a *App) UpdateQueuedPriority(jobID string, newPriority int) error {
	return a.state.UpdateQueuedPriority(jobID, newPriority)
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
