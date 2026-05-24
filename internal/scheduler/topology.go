package scheduler

import "github.com/john221wick/gpuSchedularSN/internal/agent"

type Topology struct {
	Devices   []agent.GPUDevice
	Bandwidth [][]float32
}

func BuildTopology(devices []agent.GPUDevice, links []agent.GPULink) *Topology {
	n := len(devices)
	if n == 0 {
		return nil
	}

	bw := make([][]float32, n)
	for i := range bw {
		bw[i] = make([]float32, n)
	}

	for _, l := range links {
		if l.GPUA < n && l.GPUB < n {
			bw[l.GPUA][l.GPUB] = l.BandwidthGBps
			bw[l.GPUB][l.GPUA] = l.BandwidthGBps
		}
	}

	return &Topology{
		Devices:   devices,
		Bandwidth: bw,
	}
}
