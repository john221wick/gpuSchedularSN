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
	"github.com/john221wick/gpuSchedularSN/internal/agentserver"
	"github.com/john221wick/gpuSchedularSN/internal/cluster"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
	"github.com/john221wick/gpuSchedularSN/internal/state"
)

// App is the Wails application bridge.
// Every exported method is callable from the frontend via Wails bindings.
type App struct {
	ctx   context.Context
	state *state.State

	// Cluster mode fields
	remoteMode  bool
	manager     *cluster.NodeManager
	scheduler   *cluster.ClusterScheduler
	sshSessions map[string]*cluster.SSHSession
	savedNodes  map[string]*SavedNode // nodeID -> saved config
}

func NewApp() *App {
	return &App{
		sshSessions: make(map[string]*cluster.SSHSession),
		savedNodes:  make(map[string]*SavedNode),
	}
}

// Startup is called by Wails when the app starts.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	agent.Init()
	devices := agent.GetDevices()
	links := agent.GetTopology()
	a.state = state.NewState(devices, links)
	a.state.StartSchedulerLoop()

	// Load saved config (but don't auto-connect — user reconnects manually)
	a.loadSavedConfig()
}

// Shutdown is called by Wails when the app closes.
func (a *App) Shutdown(ctx context.Context) {
	if a.state != nil {
		a.state.Stop()
	}
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	if a.manager != nil {
		a.manager.Stop()
	}
	for _, sess := range a.sshSessions {
		sess.Close()
	}
	agent.Shutdown()
}

// loadSavedConfig loads saved node configs for reference (no auto-connect).
func (a *App) loadSavedConfig() {
	cfg, err := LoadDesktopConfig()
	if err != nil {
		return
	}
	for _, sn := range cfg.Nodes {
		snCopy := sn
		a.savedNodes[sn.ID] = &snCopy
	}
	if cfg.RemoteMode {
		a.SetRemoteMode(true)
	}
}

// saveConfig persists current state to desktop-config.json.
func (a *App) saveConfig() {
	cfg := &DesktopConfig{
		RemoteMode: a.remoteMode,
	}

	for _, sn := range a.savedNodes {
		// Update paths from live node data
		if a.manager != nil {
			if node, ok := a.manager.GetNode(sn.ID); ok {
				sn.LocalDir = node.LocalDir
				sn.RemoteDir = node.RemoteDir
			}
		}
		cfg.Nodes = append(cfg.Nodes, *sn)
	}

	if err := SaveDesktopConfig(cfg); err != nil {
		fmt.Printf("Warning: save config failed: %v\n", err)
	}
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
	Mode        string `json:"mode"`
	NumGPUs     int    `json:"numGPUs"`
	MinVRAMMB   uint64 `json:"minVRAMMB"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submittedAt"`
	StartedAt   string `json:"startedAt"`
	GPUIDs      []int  `json:"gpuIDs"`
	NodeID      string `json:"nodeID"`
	NodeName    string `json:"nodeName"`
}

type SubmitRequest struct {
	Command      string `json:"command"`
	PathVariable string `json:"pathVariable"`
	Mode         string `json:"mode"`
	NumGPUs      int    `json:"numGPUs"`
	MinVRAMMB    uint64 `json:"minVRAMMB"`
	Priority     int    `json:"priority"`
}

// normalizeMode defaults to "training" unless the caller explicitly asked for
// "inference". Keeps job records consistent regardless of frontend/recovery path.
func normalizeMode(mode string) string {
	if mode == "inference" {
		return "inference"
	}
	return "training"
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

type NodeInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	NumGPUs   int    `json:"numGPUs"`
	FreeGPUs  int    `json:"freeGPUs"`
	LocalDir  string `json:"localDir"`
	RemoteDir string `json:"remoteDir"`
	GPUVendor string `json:"gpuVendor"`
	GPUName   string `json:"gpuName"`
	Arch      string `json:"arch"`
	OS        string `json:"os"`
}

type LogData struct {
	Data   string `json:"data"`
	Offset int64  `json:"offset"`
	EOF    bool   `json:"eof"`
}

// --- Mode toggle ---

func (a *App) SetRemoteMode(enabled bool) error {
	if enabled && a.manager == nil {
		a.manager = cluster.NewNodeManager()
		a.scheduler = cluster.NewClusterScheduler(a.manager)

		// Wire file transfer: rsync localDir → remoteDir before job dispatch
		a.scheduler.TransferFn = func(localDir, nodeID string, job *cluster.ClusterJob) (string, error) {
			sess, ok := a.sshSessions[nodeID]
			if !ok {
				return "", fmt.Errorf("no SSH session for node %s", nodeID)
			}
			remoteDir := job.WorkDir
			if remoteDir == "" {
				return "", fmt.Errorf("no remote dir configured for node %s", nodeID)
			}
			fmt.Printf("Rsyncing %s → %s:%s\n", localDir, nodeID, remoteDir)
			if err := cluster.RsyncToRemote(localDir, sess.Config, remoteDir); err != nil {
				return "", err
			}
			return remoteDir, nil
		}

		a.manager.StartHeartbeat()
		a.scheduler.StartLoop()
	}
	a.remoteMode = enabled
	a.saveConfig()
	return nil
}

func (a *App) GetRemoteMode() bool {
	return a.remoteMode
}

// --- App Logs ---

type AppLogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

func (a *App) GetAppLogs(offset int) []AppLogEntry {
	entries := appLogger.GetEntries(offset)
	result := make([]AppLogEntry, len(entries))
	for i, e := range entries {
		result[i] = AppLogEntry{Time: e.Time, Message: e.Message}
	}
	return result
}

func (a *App) GetAppLogCount() int {
	return appLogger.Len()
}

// --- Node methods (cluster/remote mode) ---

func (a *App) ConnectNode(sshCommand string, keyPath string, mockMode bool) (NodeInfo, error) {
	if a.manager == nil {
		return NodeInfo{}, fmt.Errorf("remote mode not enabled")
	}

	config, err := cluster.ParseSSHCommand(sshCommand)
	if err != nil {
		return NodeInfo{}, fmt.Errorf("invalid SSH command: %w", err)
	}
	if keyPath != "" {
		config.KeyPath = keyPath
	}

	session, err := cluster.Connect(*config)
	if err != nil {
		return NodeInfo{}, fmt.Errorf("SSH connection failed: %w", err)
	}

	// Auto-detect remote capabilities
	remoteInfo, err := session.DetectRemote()
	if err != nil {
		session.Close()
		return NodeInfo{}, fmt.Errorf("detection failed: %w", err)
	}

	// Create ~/gpuschedular as the base directory for everything on remote
	remoteBase := "~/gpuschedular"
	session.RunCommand(fmt.Sprintf("mkdir -p %s/logs", remoteBase))

	// Clean up old ~/.gpusched if it exists (migrated to ~/gpuschedular)
	session.RunCommand("rm -rf ~/.gpusched 2>/dev/null || true")

	// Ensure rsync is installed on remote (needed for file transfer)
	session.RunCommand("which rsync > /dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq rsync 2>/dev/null || yum install -y -q rsync 2>/dev/null || apk add rsync 2>/dev/null || true)")

	agentPort := 9712

	// Deploy agent binary to ~/gpuschedular/
	// Builds with -tags smi (real GPUs via nvidia-smi) or -tags mock (fake GPUs)
	// Both avoid CGO so cross-compile macOS→Linux works
	if err := session.CrossCompileAndSCP(remoteBase+"/gpusched", mockMode); err != nil {
		session.Close()
		return NodeInfo{}, fmt.Errorf("deploy agent failed: %w", err)
	}

	// Start agent (reuses if already running from this dir via pidfile)
	if err := session.StartRemoteAgent(agentPort, remoteBase, remoteInfo.GPUCount); err != nil {
		session.Close()
		return NodeInfo{}, fmt.Errorf("start remote agent failed: %w", err)
	}

	localPort, err := session.ForwardPort(agentPort)
	if err != nil {
		session.Close()
		return NodeInfo{}, fmt.Errorf("port forward failed: %w", err)
	}

	client := cluster.NewAgentClient(fmt.Sprintf("http://localhost:%d", localPort))
	nodeID := fmt.Sprintf("ssh-%s-%d", config.Host, config.Port)

	// Use detected hostname if available, otherwise fall back to SSH host
	nodeName := remoteInfo.Hostname
	if nodeName == "" {
		nodeName = config.Host
	}

	node, err := a.manager.AddRemoteNode(nodeID, nodeName, client)
	if err != nil {
		session.Close()
		return NodeInfo{}, fmt.Errorf("add node failed: %w", err)
	}

	// Store detection info and default paths on node
	node.GPUVendor = remoteInfo.GPUVendor
	node.GPUName = remoteInfo.GPUName
	node.Arch = remoteInfo.Arch
	node.OS = remoteInfo.OS

	// Resolve remote base to absolute path
	if absOut, err := session.RunCommand(fmt.Sprintf("eval echo %s", remoteBase)); err == nil {
		absPath := strings.TrimSpace(absOut)
		if absPath != "" {
			node.RemoteDir = absPath
		}
	}
	if node.RemoteDir == "" {
		node.RemoteDir = "/root/gpuschedular"
	}

	a.sshSessions[nodeID] = session

	// Reconcile: recover any jobs from previous session still running on remote
	if a.scheduler != nil {
		if status, err := client.GetStatus(); err == nil && len(status.Jobs) > 0 {
			a.scheduler.RecoverRemoteJobs(nodeID, status.Jobs)
			fmt.Printf("Recovered %d jobs from remote agent\n", len(status.Jobs))
		}
	}

	// Persist node config for auto-reconnect
	a.savedNodes[nodeID] = &SavedNode{
		ID:         nodeID,
		SSHCommand: sshCommand,
		KeyPath:    keyPath,
		MockMode:   mockMode,
	}
	a.saveConfig()

	return NodeInfo{
		ID:        node.ID,
		Name:      node.Name,
		Status:    node.Status.String(),
		NumGPUs:   len(node.Devices),
		LocalDir:  node.LocalDir,
		RemoteDir: node.RemoteDir,
		GPUVendor: remoteInfo.GPUVendor,
		GPUName:   remoteInfo.GPUName,
		Arch:      remoteInfo.Arch,
		OS:        remoteInfo.OS,
	}, nil
}

func (a *App) SetNodePaths(nodeID string, localDir string, remoteDir string) error {
	if a.manager == nil {
		return fmt.Errorf("remote mode not enabled")
	}
	if err := a.manager.SetNodePaths(nodeID, localDir, remoteDir); err != nil {
		return err
	}
	a.saveConfig()
	return nil
}

// SyncFilesToNode rsyncs a local directory to the node's remote destination.
// Returns the remote path where files were synced.
func (a *App) SyncFilesToNode(nodeID string, localPath string) (string, error) {
	if a.manager == nil {
		return "", fmt.Errorf("remote mode not enabled")
	}
	sess, ok := a.sshSessions[nodeID]
	if !ok {
		return "", fmt.Errorf("no SSH session for node %s", nodeID)
	}
	node, ok := a.manager.GetNode(nodeID)
	if !ok {
		return "", fmt.Errorf("node %s not found", nodeID)
	}
	remoteDir := node.RemoteDir
	if remoteDir == "" {
		remoteDir = "/root/gpuschedular"
	}

	localPath = strings.TrimRight(localPath, "/")

	// Ensure remote directory exists
	sess.RunCommand(fmt.Sprintf("mkdir -p %s", remoteDir))

	fmt.Printf("Syncing %s → %s:%s\n", localPath, nodeID, remoteDir)
	if err := cluster.RsyncToRemote(localPath, sess.Config, remoteDir); err != nil {
		return "", fmt.Errorf("sync failed: %w", err)
	}

	// Verify files arrived
	lsOut, _ := sess.RunCommand(fmt.Sprintf("ls -la %s/", remoteDir))
	fmt.Printf("Remote files at %s:\n%s\n", remoteDir, lsOut)

	return remoteDir, nil
}

func (a *App) DisconnectNode(nodeID string) error {
	if a.manager == nil {
		return fmt.Errorf("remote mode not enabled")
	}
	if sess, ok := a.sshSessions[nodeID]; ok {
		sess.Close()
		delete(a.sshSessions, nodeID)
	}
	// Keep savedNodes config — user can reconnect later
	// Just remove from live manager
	a.manager.RemoveNode(nodeID)
	a.saveConfig()
	return nil
}

// RemoveNode permanently removes a saved node config.
func (a *App) RemoveNode(nodeID string) error {
	if a.manager == nil {
		return fmt.Errorf("remote mode not enabled")
	}
	if sess, ok := a.sshSessions[nodeID]; ok {
		sess.Close()
		delete(a.sshSessions, nodeID)
	}
	delete(a.savedNodes, nodeID)
	a.manager.RemoveNode(nodeID)
	a.saveConfig()
	return nil
}

// ReconnectNode reconnects to a previously saved node.
func (a *App) ReconnectNode(nodeID string) (NodeInfo, error) {
	sn, ok := a.savedNodes[nodeID]
	if !ok {
		return NodeInfo{}, fmt.Errorf("no saved config for node %s", nodeID)
	}
	return a.ConnectNode(sn.SSHCommand, sn.KeyPath, sn.MockMode)
}

type SavedNodeInfo struct {
	ID         string `json:"id"`
	SSHCommand string `json:"sshCommand"`
	MockMode   bool   `json:"mockMode"`
}

// GetSavedNodes returns saved node configs that aren't currently connected.
func (a *App) GetSavedNodes() []SavedNodeInfo {
	var result []SavedNodeInfo
	for _, sn := range a.savedNodes {
		// Skip if currently connected
		if a.manager != nil {
			if _, ok := a.manager.GetNode(sn.ID); ok {
				continue
			}
		}
		result = append(result, SavedNodeInfo{
			ID:         sn.ID,
			SSHCommand: sn.SSHCommand,
			MockMode:   sn.MockMode,
		})
	}
	return result
}

func (a *App) GetNodes() []NodeInfo {
	if a.manager == nil {
		return nil
	}
	nodes := a.manager.AllNodes()
	result := make([]NodeInfo, len(nodes))
	for i, n := range nodes {
		result[i] = NodeInfo{
			ID:        n.ID,
			Name:      n.Name,
			Status:    n.Status.String(),
			NumGPUs:   len(n.Devices),
			LocalDir:  n.LocalDir,
			RemoteDir: n.RemoteDir,
			GPUVendor: n.GPUVendor,
			GPUName:   n.GPUName,
			Arch:      n.Arch,
			OS:        n.OS,
		}
	}
	return result
}

// NodeMonitorInfo is per-node machine + container health for the Monitor page.
type NodeMonitorInfo struct {
	NodeID      string                      `json:"nodeID"`
	NodeName    string                      `json:"nodeName"`
	Reachable   bool                        `json:"reachable"`
	Error       string                      `json:"error,omitempty"`
	Host         agentserver.HostStats       `json:"host"`
	GPUs         []DeviceInfo                `json:"gpus"`
	Containers   agentserver.ContainerReport `json:"containers"`
	Processes    []agentserver.ProcInfo      `json:"processes"`
	GPUProcesses []agentserver.GPUProcInfo   `json:"gpuProcesses"`
	CollectedAt  string                      `json:"collectedAt"`
}

// GetClusterMonitor polls every connected node's /monitor endpoint.
// Per-node failures are reported inline (Reachable=false) so one dead node
// does not hide the others.
func (a *App) GetClusterMonitor() []NodeMonitorInfo {
	if a.manager == nil {
		return nil
	}
	nodes := a.manager.AllNodes()
	result := make([]NodeMonitorInfo, 0, len(nodes))
	for _, n := range nodes {
		info := NodeMonitorInfo{NodeID: n.ID, NodeName: n.Name}
		client, ok := a.manager.GetClient(n.ID)
		if !ok {
			info.Error = "node not connected"
			result = append(result, info)
			continue
		}
		mon, err := client.GetMonitor()
		if err != nil {
			info.Error = err.Error()
			result = append(result, info)
			continue
		}
		info.Reachable = true
		info.Host = mon.Host
		info.GPUs = make([]DeviceInfo, len(mon.GPUs))
		for i, d := range mon.GPUs {
			info.GPUs[i] = DeviceInfo{
				ID:             d.ID,
				Vendor:         d.Vendor.String(),
				Name:           d.Name,
				VRAMTotalMB:    d.VRAMTotalMB,
				VRAMUsedMB:     d.VRAMUsedMB,
				UtilizationPct: d.UtilizationPct,
				TemperatureC:   d.TemperatureC,
			}
		}
		info.Containers = mon.Containers
		info.Processes = mon.Processes
		info.GPUProcesses = mon.GPUProcesses
		info.CollectedAt = mon.CollectedAt
		result = append(result, info)
	}
	return result
}

func (a *App) GetJobLogs(jobID string, offset int64) (LogData, error) {
	if a.manager == nil {
		return LogData{}, fmt.Errorf("remote mode not enabled")
	}

	// First: check scheduler for which node owns this job
	if a.scheduler != nil {
		// Check running jobs
		running := a.scheduler.RunningJobs()
		if cj, ok := running[jobID]; ok {
			return a.fetchLogsFromNode(cj.NodeID, jobID, offset)
		}
		// Check completed jobs
		for _, cj := range a.scheduler.CompletedJobs() {
			if cj.ID == jobID {
				return a.fetchLogsFromNode(cj.NodeID, jobID, offset)
			}
		}
	}

	// Fallback: try all nodes
	for _, n := range a.manager.AllNodes() {
		result, err := a.fetchLogsFromNode(n.ID, jobID, offset)
		if err == nil {
			return result, nil
		}
	}
	return LogData{}, fmt.Errorf("job %s not found on any node", jobID)
}

func (a *App) fetchLogsFromNode(nodeID, jobID string, offset int64) (LogData, error) {
	client, ok := a.manager.GetClient(nodeID)
	if !ok {
		return LogData{}, fmt.Errorf("node %s not connected", nodeID)
	}
	chunk, err := client.GetLogs(jobID, offset)
	if err != nil {
		return LogData{}, err
	}
	return LogData{
		Data:   chunk.Data,
		Offset: chunk.Offset,
		EOF:    chunk.EOF,
	}, nil
}

// --- Remote terminal ---

type TerminalResult struct {
	Output string `json:"output"`
	Error  string `json:"error"`
}

func (a *App) RunTerminalCommand(nodeID string, command string) TerminalResult {
	if a.manager == nil {
		return TerminalResult{Error: "remote mode not enabled"}
	}
	sess, ok := a.sshSessions[nodeID]
	if !ok {
		return TerminalResult{Error: fmt.Sprintf("no SSH session for node %s", nodeID)}
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return TerminalResult{Error: "empty command"}
	}

	out, err := sess.RunCommand(command)
	if err != nil {
		// Still return output (stderr might be in combined output)
		return TerminalResult{Output: out, Error: err.Error()}
	}
	return TerminalResult{Output: out}
}

// --- Cluster-aware job methods ---

func (a *App) GetClusterRunningJobs() []JobInfo {
	if a.scheduler == nil {
		return nil
	}
	running := a.scheduler.RunningJobs()
	result := make([]JobInfo, 0, len(running))
	for _, cj := range running {
		info := clusterJobToInfo(cj)
		result = append(result, info)
	}
	return result
}

func (a *App) GetClusterCompletedJobs() []JobInfo {
	if a.scheduler == nil {
		return nil
	}
	completed := a.scheduler.CompletedJobs()
	result := make([]JobInfo, 0, len(completed))
	for _, cj := range completed {
		result = append(result, clusterJobToInfo(cj))
	}
	return result
}

func (a *App) GetClusterQueuedJobs() []JobInfo {
	if a.scheduler == nil {
		return nil
	}
	queued := a.scheduler.QueuedJobs()
	result := make([]JobInfo, 0, len(queued))
	for _, j := range queued {
		result = append(result, jobToInfo(j))
	}
	return result
}

func (a *App) ClusterSubmitJob(req SubmitRequest) (string, error) {
	if a.scheduler == nil {
		return "", fmt.Errorf("remote mode not enabled")
	}

	command := strings.TrimSpace(req.Command)
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	if req.NumGPUs < 1 {
		req.NumGPUs = 1
	}
	if req.Priority < 1 {
		req.Priority = 10
	}

	// Check if cluster has enough GPUs
	totalGPUs := 0
	if a.manager != nil {
		for _, n := range a.manager.AllNodes() {
			totalGPUs += len(n.Devices)
		}
	}
	if totalGPUs == 0 {
		return "", fmt.Errorf("no GPUs available in cluster — connect a node with GPUs first")
	}
	if req.NumGPUs > totalGPUs {
		return "", fmt.Errorf("requested %d GPUs but only %d available in cluster", req.NumGPUs, totalGPUs)
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := &scheduler.Job{
		ID:        jobID,
		Command:   command,
		Mode:      normalizeMode(req.Mode),
		NumGPUs:   req.NumGPUs,
		MinVRAMMB: req.MinVRAMMB,
		Priority:  req.Priority,
	}
	a.scheduler.SubmitJob(job)
	return jobID, nil
}

func (a *App) ClusterKillJob(jobID string) error {
	if a.scheduler == nil {
		return fmt.Errorf("remote mode not enabled")
	}
	return a.scheduler.KillJob(jobID)
}

func (a *App) GetClusterDashboard() DashboardInfo {
	if a.manager == nil {
		return DashboardInfo{}
	}

	nodes := a.manager.AllNodes()
	var totalGPUs, freeGPUs int
	var totalVRAM, usedVRAM uint64
	var totalUtil float32

	for _, n := range nodes {
		totalGPUs += len(n.Devices)
		for _, d := range n.Devices {
			totalVRAM += d.VRAMTotalMB
			usedVRAM += d.VRAMUsedMB
			totalUtil += d.UtilizationPct
		}
	}

	// TODO: calculate free GPUs from scheduler allocated map
	freeGPUs = totalGPUs
	if a.scheduler != nil {
		running := a.scheduler.RunningJobs()
		for _, cj := range running {
			freeGPUs -= len(cj.GPUIDs)
		}
	}

	avgUtil := float32(0)
	if totalGPUs > 0 {
		avgUtil = totalUtil / float32(totalGPUs)
	}

	runningCount := 0
	queuedCount := 0
	if a.scheduler != nil {
		runningCount = len(a.scheduler.RunningJobs())
		queuedCount = a.scheduler.QueueLen()
	}

	return DashboardInfo{
		TotalGPUs:   totalGPUs,
		FreeGPUs:    freeGPUs,
		RunningJobs: runningCount,
		QueuedJobs:  queuedCount,
		AvgUtil:     avgUtil,
		TotalVRAMMB: totalVRAM,
		UsedVRAMMB:  usedVRAM,
	}
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

// --- Cluster devices ---

type ClusterDeviceInfo struct {
	DeviceInfo
	NodeID   string `json:"nodeID"`
	NodeName string `json:"nodeName"`
}

func (a *App) GetClusterDevices() []ClusterDeviceInfo {
	if a.manager == nil {
		return nil
	}
	nodes := a.manager.AllNodes()
	var result []ClusterDeviceInfo
	for _, n := range nodes {
		for _, d := range n.Devices {
			result = append(result, ClusterDeviceInfo{
				DeviceInfo: DeviceInfo{
					ID:             d.ID,
					Vendor:         d.Vendor.String(),
					Name:           d.Name,
					VRAMTotalMB:    d.VRAMTotalMB,
					VRAMUsedMB:     d.VRAMUsedMB,
					UtilizationPct: d.UtilizationPct,
					TemperatureC:   d.TemperatureC,
				},
				NodeID:   n.ID,
				NodeName: n.Name,
			})
		}
	}
	return result
}

// --- Cluster topology ---

type ClusterTopologyInfo struct {
	Nodes []NodeTopologyInfo `json:"nodes"`
}

type NodeTopologyInfo struct {
	NodeID    string      `json:"nodeID"`
	NodeName  string      `json:"nodeName"`
	NumGPUs   int         `json:"numGPUs"`
	Bandwidth [][]float32 `json:"bandwidth"`
	Links     []LinkInfo  `json:"links"`
}

func (a *App) GetClusterTopology() ClusterTopologyInfo {
	if a.manager == nil {
		return ClusterTopologyInfo{}
	}

	nodes := a.manager.AllNodes()
	result := ClusterTopologyInfo{
		Nodes: make([]NodeTopologyInfo, 0, len(nodes)),
	}

	for _, n := range nodes {
		numGPUs := len(n.Devices)
		bw := make([][]float32, numGPUs)
		for i := 0; i < numGPUs; i++ {
			bw[i] = make([]float32, numGPUs)
			if n.Topology != nil && i < len(n.Topology.Bandwidth) {
				copy(bw[i], n.Topology.Bandwidth[i])
			}
		}

		links := make([]LinkInfo, len(n.Links))
		for i, l := range n.Links {
			links[i] = LinkInfo{
				GPUA:          l.GPUA,
				GPUB:          l.GPUB,
				Type:          l.Type.String(),
				BandwidthGBps: l.BandwidthGBps,
			}
		}

		result.Nodes = append(result.Nodes, NodeTopologyInfo{
			NodeID:    n.ID,
			NodeName:  n.Name,
			NumGPUs:   numGPUs,
			Bandwidth: bw,
			Links:     links,
		})
	}

	return result
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
		Mode:        normalizeMode(req.Mode),
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
		Mode:        normalizeMode(j.Mode),
		NumGPUs:     j.NumGPUs,
		MinVRAMMB:   j.MinVRAMMB,
		Priority:    j.Priority,
		Status:      j.Status.String(),
		SubmittedAt: j.SubmittedAt.Format(time.RFC3339),
		StartedAt:   j.StartedAt.Format(time.RFC3339),
		GPUIDs:      j.GPUIDs,
	}
}

func clusterJobToInfo(cj *cluster.ClusterJob) JobInfo {
	return JobInfo{
		ID:          cj.ID,
		Command:     cj.Command,
		Mode:        normalizeMode(cj.Mode),
		NumGPUs:     cj.NumGPUs,
		MinVRAMMB:   cj.MinVRAMMB,
		Priority:    cj.Priority,
		Status:      cj.Status.String(),
		SubmittedAt: cj.SubmittedAt.Format(time.RFC3339),
		StartedAt:   cj.StartedAt.Format(time.RFC3339),
		GPUIDs:      cj.GPUIDs,
		NodeID:      cj.NodeID,
	}
}
