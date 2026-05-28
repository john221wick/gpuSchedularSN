//go:build smi

package agent

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var smiDevices []GPUDevice

func Init() (int, error) {
	devs, err := detectNvidia()
	if err == nil && len(devs) > 0 {
		smiDevices = devs
		return len(devs), nil
	}

	devs, err = detectAMD()
	if err == nil && len(devs) > 0 {
		smiDevices = devs
		return len(devs), nil
	}

	// No GPUs found — not an error, just 0 devices
	fmt.Println("No GPUs detected via nvidia-smi or rocm-smi — running with 0 GPUs")
	smiDevices = nil
	return 0, nil
}

func GetDevices() []GPUDevice {
	// Refresh live stats
	if refreshed, err := detectNvidia(); err == nil && len(refreshed) > 0 {
		smiDevices = refreshed
	} else if refreshed, err := detectAMD(); err == nil && len(refreshed) > 0 {
		smiDevices = refreshed
	}
	return smiDevices
}

func GetTopology() []GPULink {
	n := len(smiDevices)
	var links []GPULink

	// Try nvidia-smi topo
	if n > 1 && smiDevices[0].Vendor == NVIDIA {
		if parsed := parseNvidiaTopology(n); len(parsed) > 0 {
			return parsed
		}
	}

	// Fallback: assume PCIe between all pairs
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			links = append(links, GPULink{GPUA: i, GPUB: j, Type: PCIe, BandwidthGBps: 32})
		}
	}
	return links
}

func Refresh() []GPUDevice {
	return GetDevices()
}

func Shutdown() {}

// detectNvidia parses nvidia-smi CSV output for GPU info.
func detectNvidia() ([]GPUDevice, error) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu",
		"--format=csv,noheader,nounits").CombinedOutput()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var devices []GPUDevice

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 6 {
			continue
		}

		id, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		name := strings.TrimSpace(fields[1])
		vramTotal, _ := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64)
		vramUsed, _ := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)
		util, _ := strconv.ParseFloat(strings.TrimSpace(fields[4]), 32)
		temp, _ := strconv.Atoi(strings.TrimSpace(fields[5]))

		devices = append(devices, GPUDevice{
			ID:             id,
			Vendor:         NVIDIA,
			Name:           name,
			VRAMTotalMB:    vramTotal,
			VRAMUsedMB:     vramUsed,
			UtilizationPct: float32(util),
			TemperatureC:   temp,
			VendorIndex:    id,
		})
	}
	return devices, nil
}

// detectAMD parses rocm-smi output for GPU info.
func detectAMD() ([]GPUDevice, error) {
	out, err := exec.Command("rocm-smi", "--showid", "--showproductname",
		"--showmeminfo", "vram", "--showtemp", "--showuse", "--csv").CombinedOutput()
	if err != nil {
		return nil, err
	}

	// rocm-smi CSV is less standardized; do basic parsing
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("no rocm-smi data")
	}

	var devices []GPUDevice
	for i, line := range lines[1:] { // skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		devices = append(devices, GPUDevice{
			ID:          i,
			Vendor:      AMD,
			Name:        fmt.Sprintf("AMD GPU %d", i),
			VRAMTotalMB: 16000,
			VendorIndex: i,
		})
	}
	return devices, nil
}

// parseNvidiaTopology uses nvidia-smi topo -m to detect NVLink/PCIe.
func parseNvidiaTopology(n int) []GPULink {
	out, err := exec.Command("nvidia-smi", "topo", "-m").CombinedOutput()
	if err != nil {
		return nil
	}

	var links []GPULink
	lines := strings.Split(string(out), "\n")

	for i, line := range lines {
		if i == 0 || !strings.HasPrefix(line, "GPU") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < n+1 {
			continue
		}
		gpuA := i - 1 // header offset
		if gpuA < 0 || gpuA >= n {
			continue
		}
		for j := 1; j <= n; j++ {
			gpuB := j - 1
			if gpuB <= gpuA || gpuB >= n {
				continue
			}
			field := fields[j]
			lt := PCIe
			bw := float32(32)
			if strings.Contains(field, "NV") {
				lt = NVLink
				bw = 600
				// Try parsing NVLink version count (e.g. "NV12" = 12 links)
				numStr := strings.TrimPrefix(field, "NV")
				if count, err := strconv.Atoi(numStr); err == nil {
					bw = float32(count) * 50 // ~50 GB/s per NVLink
				}
			} else if strings.Contains(field, "SYS") || strings.Contains(field, "NODE") {
				bw = 16
			} else if strings.Contains(field, "PHB") {
				bw = 32
			} else if strings.Contains(field, "PIX") || strings.Contains(field, "PXB") {
				bw = 16
			}
			links = append(links, GPULink{GPUA: gpuA, GPUB: gpuB, Type: lt, BandwidthGBps: bw})
		}
	}
	return links
}
