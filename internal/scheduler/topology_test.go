package scheduler

import (
	"testing"
	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

func mockDevices() []agent.GPUDevice {
	return []agent.GPUDevice{
		{ID: 0, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 1, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 2, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 3, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
	}
}

func mockLinks() []agent.GPULink {
	return []agent.GPULink{
		{GPUA: 0, GPUB: 1, Type: agent.NVLink, BandwidthGBps: 600},
		{GPUA: 2, GPUB: 3, Type: agent.NVLink, BandwidthGBps: 600},
		{GPUA: 0, GPUB: 2, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 0, GPUB: 3, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 2, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 3, Type: agent.PCIe, BandwidthGBps: 32},
	}
}

func TestBuildTopology(t *testing.T) {
	topo := BuildTopology(mockDevices(), mockLinks())
	if topo == nil {
		t.Fatal("BuildTopology returned nil")
	}

	n := len(topo.Devices)
	if n != 4 {
		t.Fatalf("expected 4 devices, got %d", n)
	}

	bw := topo.Bandwidth

	if len(bw) != 4 {
		t.Fatalf("expected 4x4 matrix, got %dx%d", len(bw), len(bw[0]))
	}

	// diagonal is 0
	for i := 0; i < n; i++ {
		if bw[i][i] != 0 {
			t.Errorf("diagonal bw[%d][%d] = %f, want 0", i, i, bw[i][i])
		}
	}

	// symmetric
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if bw[i][j] != bw[j][i] {
				t.Errorf("not symmetric: bw[%d][%d]=%f, bw[%d][%d]=%f", i, j, bw[i][j], j, i, bw[j][i])
			}
		}
	}

	// NVLink pairs
	if bw[0][1] != 600 {
		t.Errorf("bw[0][1] = %f, want 600 (NVLink)", bw[0][1])
	}
	if bw[2][3] != 600 {
		t.Errorf("bw[2][3] = %f, want 600 (NVLink)", bw[2][3])
	}

	// PCIe pairs
	if bw[0][2] != 32 {
		t.Errorf("bw[0][2] = %f, want 32 (PCIe)", bw[0][2])
	}
	if bw[0][3] != 32 {
		t.Errorf("bw[0][3] = %f, want 32 (PCIe)", bw[0][3])
	}
	if bw[1][2] != 32 {
		t.Errorf("bw[1][2] = %f, want 32 (PCIe)", bw[1][2])
	}
	if bw[1][3] != 32 {
		t.Errorf("bw[1][3] = %f, want 32 (PCIe)", bw[1][3])
	}
}

func TestBuildTopologyEmpty(t *testing.T) {
	topo := BuildTopology(nil, nil)
	if topo != nil {
		t.Errorf("expected nil for empty devices, got %v", topo)
	}
}

func TestBuildTopologySingleGPU(t *testing.T) {
	devices := []agent.GPUDevice{
		{ID: 0, Vendor: agent.Apple, Name: "Apple M1", VRAMTotalMB: 8192},
	}
	topo := BuildTopology(devices, nil)
	if topo == nil {
		t.Fatal("BuildTopology returned nil for single GPU")
	}
	if len(topo.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(topo.Devices))
	}
	if topo.Bandwidth[0][0] != 0 {
		t.Errorf("single GPU diagonal should be 0, got %f", topo.Bandwidth[0][0])
	}
}
