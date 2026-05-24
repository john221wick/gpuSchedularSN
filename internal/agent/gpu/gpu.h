#pragma once
#include <stdint.h>

enum gpu_vendor {
	GPU_VENDOR_UNKNOWN = 0,
	GPU_VENDOR_NVIDIA,
	GPU_VENDOR_AMD,
	GPU_VENDOR_INTEL,
	GPU_VENDOR_APPLE
};

enum gpu_link_type {
	GPU_LINK_NONE = 0,
	GPU_LINK_PCIE,
	GPU_LINK_NVLINK,
	GPU_LINK_NVSWITCH,
	GPU_LINK_XGMI,
	GPU_LINK_XELINK,
	GPU_LINK_THUNDERBOLT,
	GPU_LINK_UNIFIED_MEMORY
};

struct gpu_device {
	int      id;
	int      vendor;
	int      temperature_c;
	int      vendor_index;
	char     name[128];
	uint64_t vram_total_mb;
	uint64_t vram_used_mb;
	float    utilization_pct;
};

struct gpu_link {
	int   gpu_a;
	int   gpu_b;
	int   link_type;
	float bandwidth_gbps;
};

int  gpu_init(void);
int  gpu_num_devices(void);
int  gpu_get_devices(struct gpu_device *out, int max_devices);
int  gpu_get_topology(struct gpu_link *out, int max_links);
int  gpu_refresh(struct gpu_device *out, int max_devices);
void gpu_shutdown(void);
