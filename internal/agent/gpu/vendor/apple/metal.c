/*
 * Apple Metal/IOKit backend.
 *
 * Apple Silicon uses unified memory — GPU shares RAM with CPU.
 * Single GPU, no multi-GPU topology.
 *
 * Detection: sysctl for chip name and total memory.
 * Monitoring: IOKit IOAccelerator for GPU utilization and VRAM usage.
 *
 * macOS only. Compiles to no-ops on other platforms.
 */

#ifdef __APPLE__

#include "metal.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <sys/sysctl.h>
#include <mach/mach.h>

/* IOKit framework — loaded via framework linker flag */
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

/* Compatibility: kIOMainPortDefault introduced in macOS 12 SDK */
#if !defined(kIOMainPortDefault)
#define kIOMainPortDefault kIOMasterPortDefault
#endif

static struct gpu_device cached_device;
static int               detected = 0;

/* ------------------------------------------------------------------ */
/*  IOKit helpers for GPU utilization and VRAM usage                   */
/* ------------------------------------------------------------------ */

/*
 * IOKit exposes GPU stats via the IOAccelerator class.
 * Keys of interest:
 *   "PerformanceStatistics" dictionary containing:
 *     - "Device Utilization %" (int, 0-100)
 *     - "In use system memory" or "vramUsedBytes" (bytes)
 *     - "Alloc system memory" (bytes)
 */

static int iokit_get_gpu_stats(float *util_pct, uint64_t *vram_used_mb) {
    *util_pct = 0.0f;
    *vram_used_mb = 0;

    CFMutableDictionaryRef match = IOServiceMatching("IOAccelerator");
    if (!match) return -1;

    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
    if (kr != KERN_SUCCESS) return -1;

    io_service_t service;
    int found = 0;

    while ((service = IOIteratorNext(iter)) != 0) {
        CFMutableDictionaryRef props = NULL;
        kr = IORegistryEntryCreateCFProperties(service, &props, kCFAllocatorDefault, 0);
        if (kr != KERN_SUCCESS || !props) {
            IOObjectRelease(service);
            continue;
        }

        /* Look for PerformanceStatistics dictionary */
        CFDictionaryRef perf = (CFDictionaryRef)CFDictionaryGetValue(props,
            CFSTR("PerformanceStatistics"));

        if (perf && CFGetTypeID(perf) == CFDictionaryGetTypeID()) {
            /* GPU Utilization */
            CFNumberRef util_ref = (CFNumberRef)CFDictionaryGetValue(perf,
                CFSTR("Device Utilization %"));
            if (util_ref && CFGetTypeID(util_ref) == CFNumberGetTypeID()) {
                int64_t util_val = 0;
                CFNumberGetValue(util_ref, kCFNumberSInt64Type, &util_val);
                *util_pct = (float)util_val;
            }

            /* VRAM used — try multiple key names Apple uses */
            CFNumberRef mem_ref = NULL;

            /* Apple Silicon uses "In use system memory" */
            mem_ref = (CFNumberRef)CFDictionaryGetValue(perf,
                CFSTR("In use system memory"));
            if (!mem_ref || CFGetTypeID(mem_ref) != CFNumberGetTypeID()) {
                /* Fallback: "vramUsedBytes" (discrete GPUs / older) */
                mem_ref = (CFNumberRef)CFDictionaryGetValue(perf,
                    CFSTR("vramUsedBytes"));
            }
            if (!mem_ref || CFGetTypeID(mem_ref) != CFNumberGetTypeID()) {
                /* Fallback: "Alloc system memory" */
                mem_ref = (CFNumberRef)CFDictionaryGetValue(perf,
                    CFSTR("Alloc system memory"));
            }

            if (mem_ref && CFGetTypeID(mem_ref) == CFNumberGetTypeID()) {
                int64_t mem_val = 0;
                CFNumberGetValue(mem_ref, kCFNumberSInt64Type, &mem_val);
                *vram_used_mb = (uint64_t)(mem_val / (1024 * 1024));
            }

            found = 1;
        }

        CFRelease(props);
        IOObjectRelease(service);
        if (found) break;
    }

    IOObjectRelease(iter);
    return found ? 0 : -1;
}

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

int apple_detect(void) {
    if (detected) return 1;

    memset(&cached_device, 0, sizeof(cached_device));
    cached_device.id = 0;
    cached_device.vendor = GPU_VENDOR_APPLE;
    cached_device.vendor_index = 0;

    /* Chip name from sysctl */
    char name[128] = "Apple GPU";
    char buf[256];
    size_t len = sizeof(buf);
    if (sysctlbyname("machdep.cpu.brand_string", buf, &len, NULL, 0) == 0) {
        snprintf(name, sizeof(name), "%s (GPU)", buf);
    }
    strncpy(cached_device.name, name, 127);
    cached_device.name[127] = '\0';

    /* Total unified memory */
    long long memsize = 0;
    len = sizeof(memsize);
    if (sysctlbyname("hw.memsize", &memsize, &len, NULL, 0) == 0) {
        cached_device.vram_total_mb = (uint64_t)(memsize / (1024 * 1024));
    }

    /* Live stats from IOKit */
    float util = 0.0f;
    uint64_t vram_used = 0;
    if (iokit_get_gpu_stats(&util, &vram_used) == 0) {
        cached_device.utilization_pct = util;
        cached_device.vram_used_mb = vram_used;
    }

    /* Temperature: IOKit can provide this via AppleSMC but requires
       SMC key access which may need elevated privileges.
       Skip for now — report 0. */
    cached_device.temperature_c = 0;

    detected = 1;
    return 1;
}

int apple_enumerate(struct gpu_device *out, int max_devices) {
    if (!detected || max_devices < 1) return 0;

    /* Refresh live stats */
    float util = 0.0f;
    uint64_t vram_used = 0;
    if (iokit_get_gpu_stats(&util, &vram_used) == 0) {
        cached_device.utilization_pct = util;
        cached_device.vram_used_mb = vram_used;
    }

    memcpy(&out[0], &cached_device, sizeof(struct gpu_device));
    return 1;
}

int apple_topology(struct gpu_link *out, int max_links, int num_devices) {
    /* Single GPU — no topology */
    (void)out;
    (void)max_links;
    (void)num_devices;
    return 0;
}

int apple_refresh(struct gpu_device *out, int max_devices) {
    return apple_enumerate(out, max_devices);
}

void apple_shutdown(void) {
    detected = 0;
}

#else /* !__APPLE__ */

#include "metal.h"

int  apple_detect(void)                                              { return 0; }
int  apple_enumerate(struct gpu_device *out, int max_devices)        { (void)out; (void)max_devices; return 0; }
int  apple_topology(struct gpu_link *out, int max_links, int n)      { (void)out; (void)max_links; (void)n; return 0; }
int  apple_refresh(struct gpu_device *out, int max_devices)          { (void)out; (void)max_devices; return 0; }
void apple_shutdown(void)                                            {}

#endif /* __APPLE__ */
