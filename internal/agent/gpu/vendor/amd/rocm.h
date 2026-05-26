#pragma once

#include "../../gpu.h"

/*
 * AMD ROCm SMI backend — dlopen-based, no compile-time linking.
 *
 * Loads librocm_smi64.so at runtime. If ROCm SMI is not present
 * (no AMD driver / ROCm installed), rocm_detect() returns 0.
 */

int  rocm_detect(void);
int  rocm_enumerate(struct gpu_device *out, int max_devices);
int  rocm_topology(struct gpu_link *out, int max_links, int num_devices);
int  rocm_refresh(struct gpu_device *out, int max_devices);
void rocm_shutdown(void);
