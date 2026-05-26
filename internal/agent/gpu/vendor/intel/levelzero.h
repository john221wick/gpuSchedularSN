#pragma once

#include "../../gpu.h"

/*
 * Intel Level Zero + Sysman backend — dlopen-based, no compile-time linking.
 *
 * Loads libze_loader.so at runtime. If Level Zero is not present
 * (no Intel GPU driver / oneAPI installed), ze_detect() returns 0.
 *
 * Requires ZES_ENABLE_SYSMAN=1 in environment for monitoring
 * (temperature, memory usage). Set automatically in ze_detect().
 */

int  ze_detect(void);
int  ze_enumerate(struct gpu_device *out, int max_devices);
int  ze_topology(struct gpu_link *out, int max_links, int num_devices);
int  ze_refresh(struct gpu_device *out, int max_devices);
void ze_shutdown(void);
