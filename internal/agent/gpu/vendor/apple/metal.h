#pragma once

#include "../../gpu.h"

/*
 * Apple Metal/IOKit backend — uses sysctl for basic detection
 * and IOKit for GPU utilization and VRAM usage on Apple Silicon.
 *
 * Single GPU (unified memory architecture). No multi-GPU topology.
 * Works on macOS only.
 */

int  apple_detect(void);
int  apple_enumerate(struct gpu_device *out, int max_devices);
int  apple_topology(struct gpu_link *out, int max_links, int num_devices);
int  apple_refresh(struct gpu_device *out, int max_devices);
void apple_shutdown(void);
