//go:build mock

package agent

import (
	"fmt"
	"os"
	"strconv"
)

var mockGPUCount = 4

func Init() (int, error) {
	if env := os.Getenv("GPUSCHED_MOCK_GPUS"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 && n <= 32 {
			mockGPUCount = n
		}
	}
	return mockGPUCount, nil
}

func GetDevices() []GPUDevice {
	devices := make([]GPUDevice, mockGPUCount)
	for i := 0; i < mockGPUCount; i++ {
		devices[i] = GPUDevice{
			ID:             i,
			Vendor:         NVIDIA,
			Name:           "NVIDIA A100-SXM4-80GB",
			VRAMTotalMB:    80000,
			VRAMUsedMB:     0,
			UtilizationPct: 0,
			TemperatureC:   35,
			VendorIndex:    i,
		}
	}
	return devices
}

func GetTopology() []GPULink {
	var links []GPULink
	for i := 0; i < mockGPUCount; i++ {
		for j := i + 1; j < mockGPUCount; j++ {
			// Pair GPUs (0,1), (2,3), (4,5)... with NVLink; cross-pair with PCIe
			if i/2 == j/2 && j == i+1 {
				links = append(links, GPULink{GPUA: i, GPUB: j, Type: NVLink, BandwidthGBps: 600})
			} else {
				links = append(links, GPULink{GPUA: i, GPUB: j, Type: PCIe, BandwidthGBps: 32})
			}
		}
	}
	return links
}

func Refresh() []GPUDevice {
	return GetDevices()
}

func Shutdown() {
	fmt.Println("mock shutdown (no-op)")
}
