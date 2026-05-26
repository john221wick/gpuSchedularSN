#pragma once

#include "../../gpu.h"

/*
 * NVIDIA NVML backend — dlopen-based, no compile-time linking.
 *
 * Loads libnvidia-ml.so.1 at runtime. If NVML is not present
 * (no NVIDIA driver installed), nvml_detect() returns 0 and
 * everything else is a no-op.
 */

int  nvml_detect(void);
int  nvml_enumerate(struct gpu_device *out, int max_devices);
int  nvml_topology(struct gpu_link *out, int max_links, int num_devices);
int  nvml_refresh(struct gpu_device *out, int max_devices);
void nvml_shutdown(void);
