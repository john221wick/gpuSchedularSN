package agentserver

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

// HostStats describes the health of the machine the agent runs on.
type HostStats struct {
	Hostname      string     `json:"hostname"`
	OSName        string     `json:"osName"`   // distro pretty name, e.g. "Ubuntu 22.04.4 LTS"
	Kernel        string     `json:"kernel"`   // kernel release, e.g. "5.15.0-105-generic"
	Arch          string     `json:"arch"`     // CPU architecture, e.g. "amd64"
	CPUModel      string     `json:"cpuModel"` // CPU model name
	UptimeSeconds uint64     `json:"uptimeSeconds"`
	CPUPercent    float64    `json:"cpuPercent"`
	CPUCores      int        `json:"cpuCores"`
	MemTotalMB    uint64     `json:"memTotalMB"`
	MemUsedMB     uint64     `json:"memUsedMB"`
	LoadAvg       [3]float64 `json:"loadAvg"`
	PerCoreCPU    []float64  `json:"perCoreCPU"` // busy % per logical core
}

// ProcInfo is one OS process for the CPU/memory breakdown.
type ProcInfo struct {
	PID        int     `json:"pid"`
	Command    string  `json:"command"`
	CPUPercent float64 `json:"cpuPercent"`
	MemMB      float64 `json:"memMB"`
}

// GPUProcInfo is one process using a GPU (via nvidia-smi).
type GPUProcInfo struct {
	PID   int     `json:"pid"`
	Name  string  `json:"name"`
	MemMB float64 `json:"memMB"`
}

// ContainerInfo is one running container.
type ContainerInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Image      string  `json:"image"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpuPercent"`
	MemUsedMB  float64 `json:"memUsedMB"`
	MemLimitMB float64 `json:"memLimitMB"`
}

// ContainerReport is the container section of the monitor response.
type ContainerReport struct {
	Available  bool            `json:"available"`
	Runtime    string          `json:"runtime"`
	Error      string          `json:"error,omitempty"`
	Containers []ContainerInfo `json:"containers"`
}

// MonitorResponse is the payload returned by GET /monitor.
type MonitorResponse struct {
	Host         HostStats         `json:"host"`
	GPUs         []agent.GPUDevice `json:"gpus"`
	Containers   ContainerReport   `json:"containers"`
	Processes    []ProcInfo        `json:"processes"`
	GPUProcesses []GPUProcInfo     `json:"gpuProcesses"`
	CollectedAt  string            `json:"collectedAt"`
}

// CollectMonitor gathers host + GPU + container stats for this machine.
func CollectMonitor() MonitorResponse {
	return MonitorResponse{
		Host:         collectHostStats(),
		GPUs:         agent.Refresh(),
		Containers:   collectContainers(),
		Processes:    collectProcesses(),
		GPUProcesses: collectGPUProcesses(),
		CollectedAt:  time.Now().Format(time.RFC3339),
	}
}

// ---- Host stats (Linux /proc; partial on other OSes) ----

func collectHostStats() HostStats {
	hs := HostStats{CPUCores: runtime.NumCPU(), Arch: runtime.GOARCH, OSName: runtime.GOOS}
	if name, err := os.Hostname(); err == nil {
		hs.Hostname = name
	}
	if runtime.GOOS != "linux" {
		return hs // /proc unavailable; hostname + cores + arch only
	}
	if osName := readOSName(); osName != "" {
		hs.OSName = osName
	}
	hs.Kernel = readKernel()
	hs.CPUModel = readCPUModel()
	hs.UptimeSeconds = readUptime()
	hs.MemTotalMB, hs.MemUsedMB = readMem()
	hs.LoadAvg = readLoadAvg()
	hs.CPUPercent, hs.PerCoreCPU = readCPUUsage()
	return hs
}

// readOSName returns the distro PRETTY_NAME from /etc/os-release.
func readOSName() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return ""
}

func readKernel() string {
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

func readUptime() uint64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0
	}
	f, _ := strconv.ParseFloat(fields[0], 64)
	return uint64(f)
}

func readMem() (totalMB, usedMB uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var total, avail uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64) // value is in kB
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			avail = val
		}
	}
	totalMB = total / 1024
	if total >= avail {
		usedMB = (total - avail) / 1024
	}
	return totalMB, usedMB
}

func readLoadAvg() [3]float64 {
	var la [3]float64
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return la
	}
	fields := strings.Fields(string(b))
	for i := 0; i < 3 && i < len(fields); i++ {
		la[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return la
}

type cpuTimes struct{ idle, total uint64 }

// readCPUUsage samples /proc/stat twice and returns aggregate busy % plus per-core %.
func readCPUUsage() (aggPct float64, corePct []float64) {
	agg1, cores1 := readCPUTimes()
	time.Sleep(150 * time.Millisecond)
	agg2, cores2 := readCPUTimes()

	aggPct = busyPct(agg1, agg2)
	n := len(cores1)
	if len(cores2) < n {
		n = len(cores2)
	}
	corePct = make([]float64, n)
	for i := 0; i < n; i++ {
		corePct[i] = busyPct(cores1[i], cores2[i])
	}
	return aggPct, corePct
}

func busyPct(a, b cpuTimes) float64 {
	dt := float64(b.total - a.total)
	di := float64(b.idle - a.idle)
	if dt <= 0 {
		return 0
	}
	p := (1.0 - di/dt) * 100.0
	if p < 0 {
		p = 0
	}
	return p
}

// readCPUTimes parses the aggregate "cpu" line and every "cpuN" core line.
func readCPUTimes() (agg cpuTimes, cores []cpuTimes) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return agg, cores
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 || !strings.HasPrefix(fields[0], "cpu") {
			break // cpu lines are first in /proc/stat
		}
		// fields: cpu user nice system idle iowait irq softirq ...
		var t cpuTimes
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			t.total += v
			if i == 4 || i == 5 { // idle + iowait count as idle
				t.idle += v
			}
		}
		if fields[0] == "cpu" {
			agg = t
		} else {
			cores = append(cores, t)
		}
	}
	return agg, cores
}

// ---- Containers (docker) ----

func collectContainers() ContainerReport {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return ContainerReport{Available: false, Containers: []ContainerInfo{}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	psOut, err := exec.CommandContext(ctx, dockerPath, "ps",
		"--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}").Output()
	if err != nil {
		return ContainerReport{Available: true, Runtime: "docker", Error: err.Error(), Containers: []ContainerInfo{}}
	}

	order := []string{}
	byID := map[string]ContainerInfo{}
	for _, line := range strings.Split(strings.TrimSpace(string(psOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		ci := ContainerInfo{ID: parts[0], Name: parts[1], Image: parts[2], Status: parts[3]}
		byID[ci.ID] = ci
		order = append(order, ci.ID)
	}

	// Merge live CPU/mem stats (best-effort; ignore errors).
	if statsOut, serr := exec.CommandContext(ctx, dockerPath, "stats", "--no-stream",
		"--format", "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}").Output(); serr == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(statsOut)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) < 3 {
				continue
			}
			ci, ok := byID[parts[0]]
			if !ok {
				continue
			}
			ci.CPUPercent = parsePercent(parts[1])
			ci.MemUsedMB, ci.MemLimitMB = parseMemUsage(parts[2])
			byID[ci.ID] = ci
		}
	}

	containers := make([]ContainerInfo, 0, len(order))
	for _, id := range order {
		containers = append(containers, byID[id])
	}
	return ContainerReport{Available: true, Runtime: "docker", Containers: containers}
}

func parsePercent(s string) float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseMemUsage parses docker's "1.5GiB / 4GiB" into (used, limit) MB.
func parseMemUsage(s string) (usedMB, limitMB float64) {
	parts := strings.Split(s, "/")
	if len(parts) >= 1 {
		usedMB = parseSize(parts[0])
	}
	if len(parts) >= 2 {
		limitMB = parseSize(parts[1])
	}
	return usedMB, limitMB
}

// parseSize parses a docker size string ("512MiB", "1.2GB", "900B") into MB.
func parseSize(s string) float64 {
	s = strings.TrimSpace(s)
	units := []struct {
		suf string
		mb  float64
	}{
		{"GiB", 1024}, {"MiB", 1}, {"KiB", 1.0 / 1024},
		{"GB", 1000}, {"MB", 1}, {"kB", 1.0 / 1000}, {"B", 1.0 / (1024 * 1024)},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suf) {
			num := strings.TrimSpace(strings.TrimSuffix(s, u.suf))
			f, _ := strconv.ParseFloat(num, 64)
			return f * u.mb
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f / (1024 * 1024) // assume bytes
}

// ---- Processes ----

// collectProcesses lists OS processes with CPU% and resident memory (via ps).
func collectProcesses() []ProcInfo {
	procs := []ProcInfo{}
	if runtime.GOOS != "linux" {
		return procs
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid,%cpu,rss,comm",
		"--no-headers", "--sort=-%cpu").Output()
	if err != nil {
		return procs
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		rssKB, _ := strconv.ParseFloat(fields[2], 64)
		procs = append(procs, ProcInfo{
			PID:        pid,
			Command:    strings.Join(fields[3:], " "),
			CPUPercent: cpu,
			MemMB:      rssKB / 1024,
		})
		if len(procs) >= 120 {
			break
		}
	}
	return procs
}

// collectGPUProcesses lists processes using a GPU (via nvidia-smi; empty if absent).
func collectGPUProcesses() []GPUProcInfo {
	res := []GPUProcInfo{}
	smi, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, smi,
		"--query-compute-apps=pid,process_name,used_gpu_memory",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return res
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		mem, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		res = append(res, GPUProcInfo{
			PID:   pid,
			Name:  strings.TrimSpace(parts[1]),
			MemMB: mem,
		})
	}
	return res
}
