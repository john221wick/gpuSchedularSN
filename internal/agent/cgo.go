//go:build !mock && !smi

package agent

/*
#cgo CFLAGS: -I${SRCDIR}/gpu
#cgo linux LDFLAGS: -ldl
#cgo darwin LDFLAGS: -framework IOKit -framework CoreFoundation
#include "gpu.h"
#include "vendor/nvidia/nvml.c"
#include "vendor/amd/rocm.c"
#include "vendor/amd/sysfs.c"
#include "vendor/intel/levelzero.c"
#include "vendor/apple/metal.c"
#include "gpu.c"
#include <stdlib.h>
*/
import "C"

import "fmt"

const MaxDevices = 32
const MaxLinks = 128

func Init() (int, error) {
	n := int(C.gpu_init())
	if n <= 0 {
		return 0, fmt.Errorf("gpu_init failed: returned %d", n)
	}
	return n, nil
}

func GetDevices() []GPUDevice {
	var buf [MaxDevices]C.struct_gpu_device
	n := int(C.gpu_get_devices(&buf[0], C.int(MaxDevices)))
	devices := make([]GPUDevice, n)
	for i := 0; i < n; i++ {
		devices[i] = GPUDevice{
			ID:             int(buf[i].id),
			Vendor:         Vendor(buf[i].vendor),
			Name:           C.GoString(&buf[i].name[0]),
			VRAMTotalMB:    uint64(buf[i].vram_total_mb),
			VRAMUsedMB:     uint64(buf[i].vram_used_mb),
			UtilizationPct: float32(buf[i].utilization_pct),
			TemperatureC:   int(buf[i].temperature_c),
			VendorIndex:    int(buf[i].vendor_index),
		}
	}
	return devices
}

func GetTopology() []GPULink {
	var buf [MaxLinks]C.struct_gpu_link
	n := int(C.gpu_get_topology(&buf[0], C.int(MaxLinks)))
	links := make([]GPULink, n)
	for i := 0; i < n; i++ {
		links[i] = GPULink{
			GPUA:          int(buf[i].gpu_a),
			GPUB:          int(buf[i].gpu_b),
			Type:          LinkType(buf[i].link_type),
			BandwidthGBps: float32(buf[i].bandwidth_gbps),
		}
	}
	return links
}

func Refresh() []GPUDevice {
	var buf [MaxDevices]C.struct_gpu_device
	n := int(C.gpu_refresh(&buf[0], C.int(MaxDevices)))
	devices := make([]GPUDevice, n)
	for i := 0; i < n; i++ {
		devices[i] = GPUDevice{
			ID:             int(buf[i].id),
			Vendor:         Vendor(buf[i].vendor),
			Name:           C.GoString(&buf[i].name[0]),
			VRAMTotalMB:    uint64(buf[i].vram_total_mb),
			VRAMUsedMB:     uint64(buf[i].vram_used_mb),
			UtilizationPct: float32(buf[i].utilization_pct),
			TemperatureC:   int(buf[i].temperature_c),
			VendorIndex:    int(buf[i].vendor_index),
		}
	}
	return devices
}

func Shutdown() {
	C.gpu_shutdown()
}
