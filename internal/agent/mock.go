//go:build mock

package agent

import "fmt"

func Init() (int, error) {
	return 4, nil
}

func GetDevices() []GPUDevice {
	return []GPUDevice{
		{ID: 0, Vendor: NVIDIA, Name: "NVIDIA A100-SXM4-80GB", VRAMTotalMB: 80000, VRAMUsedMB: 0, UtilizationPct: 0, TemperatureC: 35, VendorIndex: 0},
		{ID: 1, Vendor: NVIDIA, Name: "NVIDIA A100-SXM4-80GB", VRAMTotalMB: 80000, VRAMUsedMB: 0, UtilizationPct: 0, TemperatureC: 35, VendorIndex: 1},
		{ID: 2, Vendor: NVIDIA, Name: "NVIDIA A100-SXM4-80GB", VRAMTotalMB: 80000, VRAMUsedMB: 0, UtilizationPct: 0, TemperatureC: 35, VendorIndex: 2},
		{ID: 3, Vendor: NVIDIA, Name: "NVIDIA A100-SXM4-80GB", VRAMTotalMB: 80000, VRAMUsedMB: 0, UtilizationPct: 0, TemperatureC: 35, VendorIndex: 3},
	}
}

func GetTopology() []GPULink {
	return []GPULink{
		{GPUA: 0, GPUB: 1, Type: NVLink, BandwidthGBps: 600},
		{GPUA: 2, GPUB: 3, Type: NVLink, BandwidthGBps: 600},
		{GPUA: 0, GPUB: 2, Type: PCIe, BandwidthGBps: 32},
		{GPUA: 0, GPUB: 3, Type: PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 2, Type: PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 3, Type: PCIe, BandwidthGBps: 32},
	}
}

func Refresh() []GPUDevice {
	return GetDevices()
}

func Shutdown() {
	fmt.Println("mock shutdown (no-op)")
}
