/*
 * GPU dispatcher — routes to vendor-specific backends.
 *
 * Detection order: NVIDIA (NVML) → AMD (ROCm SMI) → AMDGPU sysfs → Intel (Level Zero) → Apple (IOKit)
 * On Linux, tries each backend. First one that returns GPUs wins.
 * On macOS, only Apple backend is available.
 */

#include "gpu.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

/* ------------------------------------------------------------------ */
/*  Vendor backend headers                                            */
/* ------------------------------------------------------------------ */

#ifdef __linux__
#include "vendor/nvidia/nvml.h"
#include "vendor/amd/rocm.h"
#include "vendor/amd/sysfs.h"
#include "vendor/intel/levelzero.h"
#endif

#ifdef __APPLE__
#include "vendor/apple/metal.h"
#endif

/* ------------------------------------------------------------------ */
/*  Active backend tracking                                           */
/* ------------------------------------------------------------------ */

typedef enum {
    BACKEND_NONE = 0,
    BACKEND_NVIDIA,
    BACKEND_AMD,
    BACKEND_AMD_SYSFS,
    BACKEND_INTEL,
    BACKEND_APPLE
} active_backend_t;

static active_backend_t active_backend = BACKEND_NONE;
static int              active_count   = 0;

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

int gpu_init(void) {
    if (active_backend != BACKEND_NONE) return active_count;

#ifdef __linux__
    /* Try NVIDIA first */
    int n = nvml_detect();
    if (n > 0) {
        active_backend = BACKEND_NVIDIA;
        active_count = n;
        return n;
    }

    /* Try AMD via ROCm SMI first. */
    n = rocm_detect();
    if (n > 0) {
        active_backend = BACKEND_AMD;
        active_count = n;
        return n;
    }

    /* Fallback for Radeon/APU devices exposed by the Linux amdgpu driver. */
    n = amdgpu_sysfs_detect();
    if (n > 0) {
        active_backend = BACKEND_AMD_SYSFS;
        active_count = n;
        return n;
    }

    /* Try Intel */
    n = ze_detect();
    if (n > 0) {
        active_backend = BACKEND_INTEL;
        active_count = n;
        return n;
    }
#endif

#ifdef __APPLE__
    if (apple_detect()) {
        active_backend = BACKEND_APPLE;
        active_count = 1;
        return 1;
    }
#endif

    return 0;
}

int gpu_num_devices(void) {
    return active_count;
}

int gpu_get_devices(struct gpu_device *out, int max_devices) {
    switch (active_backend) {
#ifdef __linux__
        case BACKEND_NVIDIA: return nvml_enumerate(out, max_devices);
        case BACKEND_AMD:    return rocm_enumerate(out, max_devices);
        case BACKEND_AMD_SYSFS: return amdgpu_sysfs_enumerate(out, max_devices);
        case BACKEND_INTEL:  return ze_enumerate(out, max_devices);
#endif
#ifdef __APPLE__
        case BACKEND_APPLE:  return apple_enumerate(out, max_devices);
#endif
        default: return 0;
    }
}

int gpu_get_topology(struct gpu_link *out, int max_links) {
    switch (active_backend) {
#ifdef __linux__
        case BACKEND_NVIDIA: return nvml_topology(out, max_links, active_count);
        case BACKEND_AMD:    return rocm_topology(out, max_links, active_count);
        case BACKEND_AMD_SYSFS: return amdgpu_sysfs_topology(out, max_links, active_count);
        case BACKEND_INTEL:  return ze_topology(out, max_links, active_count);
#endif
#ifdef __APPLE__
        case BACKEND_APPLE:  return apple_topology(out, max_links, active_count);
#endif
        default: return 0;
    }
}

int gpu_refresh(struct gpu_device *out, int max_devices) {
    switch (active_backend) {
#ifdef __linux__
        case BACKEND_NVIDIA: return nvml_refresh(out, max_devices);
        case BACKEND_AMD:    return rocm_refresh(out, max_devices);
        case BACKEND_AMD_SYSFS: return amdgpu_sysfs_refresh(out, max_devices);
        case BACKEND_INTEL:  return ze_refresh(out, max_devices);
#endif
#ifdef __APPLE__
        case BACKEND_APPLE:  return apple_refresh(out, max_devices);
#endif
        default: return 0;
    }
}

void gpu_shutdown(void) {
    switch (active_backend) {
#ifdef __linux__
        case BACKEND_NVIDIA: nvml_shutdown(); break;
        case BACKEND_AMD:    rocm_shutdown(); break;
        case BACKEND_AMD_SYSFS: amdgpu_sysfs_shutdown(); break;
        case BACKEND_INTEL:  ze_shutdown(); break;
#endif
#ifdef __APPLE__
        case BACKEND_APPLE:  apple_shutdown(); break;
#endif
        default: break;
    }
    active_backend = BACKEND_NONE;
    active_count = 0;
}
