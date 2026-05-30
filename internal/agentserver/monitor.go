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
	UptimeSeconds uint64     `json:"uptimeSeconds"`
	CPUPercent    float64    `json:"cpuPercent"`
	CPUCores      int        `json:"cpuCores"`
	MemTotalMB    uint64     `json:"memTotalMB"`
	MemUsedMB     uint64     `json:"memUsedMB"`
	LoadAvg       [3]float64 `json:"loadAvg"`
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
	Host        HostStats         `json:"host"`
	GPUs        []agent.GPUDevice `json:"gpus"`
	Containers  ContainerReport   `json:"containers"`
	CollectedAt string            `json:"collectedAt"`
}

// CollectMonitor gathers host + GPU + container stats for this machine.
func CollectMonitor() MonitorResponse {
	return MonitorResponse{
		Host:        collectHostStats(),
		GPUs:        agent.Refresh(),
		Containers:  collectContainers(),
		CollectedAt: time.Now().Format(time.RFC3339),
	}
}

// ---- Host stats (Linux /proc; partial on other OSes) ----

func collectHostStats() HostStats {
	hs := HostStats{CPUCores: runtime.NumCPU()}
	if name, err := os.Hostname(); err == nil {
		hs.Hostname = name
	}
	if runtime.GOOS != "linux" {
		return hs // /proc unavailable; hostname + cores only
	}
	hs.UptimeSeconds = readUptime()
	hs.MemTotalMB, hs.MemUsedMB = readMem()
	hs.LoadAvg = readLoadAvg()
	hs.CPUPercent = readCPUPercent()
	return hs
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

// readCPUPercent samples /proc/stat twice to compute busy %.
func readCPUPercent() float64 {
	idle1, total1 := readCPUSample()
	time.Sleep(150 * time.Millisecond)
	idle2, total2 := readCPUSample()

	dt := float64(total2 - total1)
	di := float64(idle2 - idle1)
	if dt <= 0 {
		return 0
	}
	pct := (1.0 - di/dt) * 100.0
	if pct < 0 {
		pct = 0
	}
	return pct
}

// readCPUSample returns (idle, total) jiffies from the aggregate "cpu" line.
func readCPUSample() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 || fields[0] != "cpu" {
			continue
		}
		// fields: cpu user nice system idle iowait irq softirq ...
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 || i == 5 { // idle + iowait count as idle
				idle += v
			}
		}
		return idle, total
	}
	return 0, 0
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
