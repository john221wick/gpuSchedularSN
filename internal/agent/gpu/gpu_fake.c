#include "gpu.h"
#include <string.h>

static struct gpu_device fake_devices[4];
static struct gpu_link   fake_links[6];
static int               initialized = 0;

static void init_fake_data(void) {
	if (initialized) return;

	const char *names[] = {
		"NVIDIA A100-SXM4-80GB",
		"NVIDIA A100-SXM4-80GB",
		"NVIDIA A100-SXM4-80GB",
		"NVIDIA A100-SXM4-80GB"
	};

	for (int i = 0; i < 4; i++) {
		memset(&fake_devices[i], 0, sizeof(struct gpu_device));
		fake_devices[i].id = i;
		fake_devices[i].vendor = GPU_VENDOR_NVIDIA;
		strncpy(fake_devices[i].name, names[i], 127);
		fake_devices[i].vram_total_mb = 80000;
		fake_devices[i].vram_used_mb = 0;
		fake_devices[i].utilization_pct = 0.0f;
		fake_devices[i].temperature_c = 35;
		fake_devices[i].vendor_index = i;
	}

	struct gpu_link links[] = {
		{0, 1, GPU_LINK_NVLINK, 600.0f},
		{2, 3, GPU_LINK_NVLINK, 600.0f},
		{0, 2, GPU_LINK_PCIE,   32.0f},
		{0, 3, GPU_LINK_PCIE,   32.0f},
		{1, 2, GPU_LINK_PCIE,   32.0f},
		{1, 3, GPU_LINK_PCIE,   32.0f}
	};
	memcpy(fake_links, links, sizeof(links));

	initialized = 1;
}

int gpu_init(void) {
	init_fake_data();
	return 4;
}

int gpu_num_devices(void) {
	return 4;
}

int gpu_get_devices(struct gpu_device *out, int max_devices) {
	int n = 4;
	if (n > max_devices) n = max_devices;
	memcpy(out, fake_devices, n * sizeof(struct gpu_device));
	return n;
}

int gpu_get_topology(struct gpu_link *out, int max_links) {
	int n = 6;
	if (n > max_links) n = max_links;
	memcpy(out, fake_links, n * sizeof(struct gpu_link));
	return n;
}

int gpu_refresh(struct gpu_device *out, int max_devices) {
	return gpu_get_devices(out, max_devices);
}

void gpu_shutdown(void) {
}
