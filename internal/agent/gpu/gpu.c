#include "gpu.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#ifdef __APPLE__
#include <sys/sysctl.h>
#endif

static struct gpu_device detected_devices[1];
static int               detected_count = 0;

#ifdef __APPLE__

static void detect_apple_gpu(struct gpu_device *dev) {
	memset(dev, 0, sizeof(struct gpu_device));
	dev->id = 0;
	dev->vendor = GPU_VENDOR_APPLE;
	dev->vendor_index = 0;

	char name[128] = "Apple GPU";
	char buf[256];
	size_t len = sizeof(buf);
	if (sysctlbyname("machdep.cpu.brand_string", buf, &len, NULL, 0) == 0) {
		snprintf(name, sizeof(name), "%s (GPU)", buf);
	}
	strncpy(dev->name, name, 127);

	int cores = 0;
	len = sizeof(cores);
	sysctlbyname("hw.ncpu", &cores, &len, NULL, 0);
	dev->temperature_c = 0;

	long long memsize = 0;
	len = sizeof(memsize);
	if (sysctlbyname("hw.memsize", &memsize, &len, NULL, 0) == 0) {
		dev->vram_total_mb = (uint64_t)(memsize / (1024 * 1024));
	}
	dev->vram_used_mb = 0;
	dev->utilization_pct = 0.0f;
}

#endif

static int detect_gpus(void) {
	if (detected_count > 0) return detected_count;

#ifdef __APPLE__
	detect_apple_gpu(&detected_devices[0]);
	detected_count = 1;
#elif __linux__
	FILE *f = fopen("/sys/bus/pci/devices/0000:01:00.0/vendor", "r");
	if (f) {
		char vendor_str[16] = {0};
		if (fgets(vendor_str, sizeof(vendor_str), f)) {
			if (strstr(vendor_str, "0x10de")) {
				detected_devices[0].vendor = GPU_VENDOR_NVIDIA;
				strncpy(detected_devices[0].name, "NVIDIA GPU (detected)", 127);
			} else if (strstr(vendor_str, "0x1002")) {
				detected_devices[0].vendor = GPU_VENDOR_AMD;
				strncpy(detected_devices[0].name, "AMD GPU (detected)", 127);
			} else if (strstr(vendor_str, "0x8086")) {
				detected_devices[0].vendor = GPU_VENDOR_INTEL;
				strncpy(detected_devices[0].name, "Intel GPU (detected)", 127);
			}
			detected_devices[0].id = 0;
			detected_devices[0].vendor_index = 0;
			detected_devices[0].vram_total_mb = 0;
			detected_devices[0].vram_used_mb = 0;
			detected_devices[0].utilization_pct = 0.0f;
			detected_devices[0].temperature_c = 0;
			detected_count = 1;
		}
		fclose(f);
	}
#endif

	return detected_count;
}

int gpu_init(void) {
	return detect_gpus();
}

int gpu_num_devices(void) {
	return detected_count;
}

int gpu_get_devices(struct gpu_device *out, int max_devices) {
	int n = detected_count;
	if (n > max_devices) n = max_devices;
	memcpy(out, detected_devices, n * sizeof(struct gpu_device));
	return n;
}

int gpu_get_topology(struct gpu_link *out, int max_links) {
	(void)out;
	(void)max_links;
	return 0;
}

int gpu_refresh(struct gpu_device *out, int max_devices) {
	return gpu_get_devices(out, max_devices);
}

void gpu_shutdown(void) {
}
