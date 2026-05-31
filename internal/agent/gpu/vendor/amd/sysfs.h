#pragma once

#include "../../gpu.h"

/*
 * AMDGPU sysfs fallback backend.
 *
 * Used when ROCm SMI is not available or does not expose an AMD GPU.
 * This is primarily for Linux AMDGPU devices, including many Radeon iGPUs/APUs
 * that expose monitoring data through /sys/class/drm but not through ROCm SMI.
 */

int  amdgpu_sysfs_detect(void);
int  amdgpu_sysfs_enumerate(struct gpu_device *out, int max_devices);
int  amdgpu_sysfs_topology(struct gpu_link *out, int max_links, int num_devices);
int  amdgpu_sysfs_refresh(struct gpu_device *out, int max_devices);
void amdgpu_sysfs_shutdown(void);
