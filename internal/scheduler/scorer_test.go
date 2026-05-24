package scheduler

import (
	"testing"
	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

func buildMockTopology() *Topology {
	devices := []agent.GPUDevice{
		{ID: 0, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 1, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 2, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 3, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
	}
	links := []agent.GPULink{
		{GPUA: 0, GPUB: 1, Type: agent.NVLink, BandwidthGBps: 600},
		{GPUA: 2, GPUB: 3, Type: agent.NVLink, BandwidthGBps: 600},
		{GPUA: 0, GPUB: 2, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 0, GPUB: 3, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 2, Type: agent.PCIe, BandwidthGBps: 32},
		{GPUA: 1, GPUB: 3, Type: agent.PCIe, BandwidthGBps: 32},
	}
	return BuildTopology(devices, links)
}

func TestScorePicksNVLinkPair(t *testing.T) {
	topo := buildMockTopology()
	result := Score(topo, 2, 0)
	if result == nil {
		t.Fatal("Score returned nil")
	}
	if result.Score != 600 {
		t.Errorf("score = %f, want 600 (NVLink)", result.Score)
	}
	// should be {0,1} or {2,3}
	if len(result.GPUIDs) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(result.GPUIDs))
	}
	a, b := result.GPUIDs[0], result.GPUIDs[1]
	if !((a == 0 && b == 1) || (a == 2 && b == 3)) {
		t.Errorf("expected NVLink pair {0,1} or {2,3}, got {%d,%d}", a, b)
	}
}

func TestScoreSkipsGPUsWithLowVRAM(t *testing.T) {
	topo := buildMockTopology()
	// GPU 0 has 70GB used, only 10GB free
	topo.Devices[0].VRAMUsedMB = 70000

	// need 2 GPUs, minVRAM 20GB → GPU 0 is ineligible
	result := Score(topo, 2, 20000)
	if result == nil {
		t.Fatal("Score returned nil, expected a valid group")
	}

	for _, id := range result.GPUIDs {
		if id == 0 {
			t.Error("GPU 0 should be skipped (only 10GB free, need 20GB)")
		}
	}
}

func TestScoreAllFourGPUs(t *testing.T) {
	topo := buildMockTopology()
	result := Score(topo, 4, 0)
	if result == nil {
		t.Fatal("Score returned nil")
	}
	if len(result.GPUIDs) != 4 {
		t.Fatalf("expected 4 GPUs, got %d", len(result.GPUIDs))
	}
	// all 4 GPUs: 600+600+32+32+32+32 = 1328
	if result.Score != 1328 {
		t.Errorf("score = %f, want 1328", result.Score)
	}
}

func TestScoreNotEnoughGPUs(t *testing.T) {
	topo := buildMockTopology()
	// GPUs 0,1 fully used, need 3 GPUs with minVRAM 1GB
	topo.Devices[0].VRAMUsedMB = 80000
	topo.Devices[1].VRAMUsedMB = 80000

	result := Score(topo, 3, 1000)
	if result != nil {
		t.Errorf("expected nil (not enough GPUs), got %v", result)
	}
}

func TestScoreAllVRAMUsed(t *testing.T) {
	topo := buildMockTopology()
	for i := range topo.Devices {
		topo.Devices[i].VRAMUsedMB = 80000
	}

	result := Score(topo, 2, 1000)
	if result != nil {
		t.Errorf("expected nil (no free VRAM), got %v", result)
	}
}
