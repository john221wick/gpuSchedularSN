/*
 * NVIDIA NVML backend — dlopen-based, no compile-time linking.
 *
 * Loads libnvidia-ml.so.1 at runtime. Queries device name, VRAM,
 * utilization, temperature, and NVLink/PCIe topology.
 */

#ifdef __linux__

#include "nvml.h"
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

/* ------------------------------------------------------------------ */
/*  NVML type definitions (matches nvml.h from CUDA toolkit)          */
/* ------------------------------------------------------------------ */

typedef struct nvmlDevice_st *nvmlDevice_t;

typedef enum {
    NVML_SUCCESS                    = 0,
    NVML_ERROR_UNINITIALIZED        = 1,
    NVML_ERROR_INVALID_ARGUMENT     = 2,
    NVML_ERROR_NOT_SUPPORTED        = 3,
    NVML_ERROR_NO_PERMISSION        = 4,
    NVML_ERROR_NOT_FOUND            = 6,
    NVML_ERROR_INSUFFICIENT_SIZE    = 7,
    NVML_ERROR_DRIVER_NOT_LOADED    = 9,
    NVML_ERROR_LIBRARY_NOT_FOUND    = 12,
    NVML_ERROR_FUNCTION_NOT_FOUND   = 13,
    NVML_ERROR_UNKNOWN              = 999
} nvmlReturn_t;

typedef struct {
    unsigned long long total;   /* bytes */
    unsigned long long free;    /* bytes */
    unsigned long long used;    /* bytes */
} nvmlMemory_t;

typedef struct {
    unsigned int gpu;           /* percent 0-100 */
    unsigned int memory;        /* percent 0-100 */
} nvmlUtilization_t;

typedef enum {
    NVML_TEMPERATURE_GPU = 0
} nvmlTemperatureSensors_t;

typedef enum {
    NVML_TOPOLOGY_INTERNAL   = 0,
    NVML_TOPOLOGY_SINGLE     = 10,
    NVML_TOPOLOGY_MULTIPLE   = 20,
    NVML_TOPOLOGY_HOSTBRIDGE = 30,
    NVML_TOPOLOGY_NODE       = 40,
    NVML_TOPOLOGY_SYSTEM     = 50
} nvmlGpuTopologyLevel_t;

typedef enum {
    NVML_FEATURE_DISABLED = 0,
    NVML_FEATURE_ENABLED  = 1
} nvmlEnableState_t;

#define NVML_DEVICE_PCI_BUS_ID_BUFFER_V2_LENGTH 16
#define NVML_DEVICE_PCI_BUS_ID_BUFFER_SIZE      32

typedef struct {
    char         busIdLegacy[NVML_DEVICE_PCI_BUS_ID_BUFFER_V2_LENGTH];
    unsigned int domain;
    unsigned int bus;
    unsigned int device;
    unsigned int pciDeviceId;
    unsigned int pciSubSystemId;
    char         busId[NVML_DEVICE_PCI_BUS_ID_BUFFER_SIZE];
} nvmlPciInfo_t;

#define NVML_DEVICE_NAME_BUFFER_SIZE 96
#define NVML_NVLINK_MAX_LINKS        18

/* ------------------------------------------------------------------ */
/*  dlopen function pointer typedefs                                  */
/* ------------------------------------------------------------------ */

typedef nvmlReturn_t (*nvmlInit_v2_t)(void);
typedef nvmlReturn_t (*nvmlShutdown_t)(void);
typedef nvmlReturn_t (*nvmlDeviceGetCount_v2_t)(unsigned int *);
typedef nvmlReturn_t (*nvmlDeviceGetHandleByIndex_v2_t)(unsigned int, nvmlDevice_t *);
typedef nvmlReturn_t (*nvmlDeviceGetName_t)(nvmlDevice_t, char *, unsigned int);
typedef nvmlReturn_t (*nvmlDeviceGetMemoryInfo_t)(nvmlDevice_t, nvmlMemory_t *);
typedef nvmlReturn_t (*nvmlDeviceGetUtilizationRates_t)(nvmlDevice_t, nvmlUtilization_t *);
typedef nvmlReturn_t (*nvmlDeviceGetTemperature_t)(nvmlDevice_t, nvmlTemperatureSensors_t, unsigned int *);
typedef nvmlReturn_t (*nvmlDeviceGetTopologyCommonAncestor_t)(nvmlDevice_t, nvmlDevice_t, nvmlGpuTopologyLevel_t *);
typedef nvmlReturn_t (*nvmlDeviceGetNvLinkState_t)(nvmlDevice_t, unsigned int, nvmlEnableState_t *);
typedef nvmlReturn_t (*nvmlDeviceGetNvLinkRemotePciInfo_v2_t)(nvmlDevice_t, unsigned int, nvmlPciInfo_t *);
typedef nvmlReturn_t (*nvmlDeviceGetPciInfo_v3_t)(nvmlDevice_t, nvmlPciInfo_t *);
typedef nvmlReturn_t (*nvmlDeviceGetNvLinkVersion_t)(nvmlDevice_t, unsigned int, unsigned int *);
typedef nvmlReturn_t (*nvmlDeviceGetMaxPcieLinkGeneration_t)(nvmlDevice_t, unsigned int *);
typedef nvmlReturn_t (*nvmlDeviceGetMaxPcieLinkWidth_t)(nvmlDevice_t, unsigned int *);

/* ------------------------------------------------------------------ */
/*  Static function pointers                                          */
/* ------------------------------------------------------------------ */

static void *nvml_handle = NULL;

static nvmlInit_v2_t                         fn_init;
static nvmlShutdown_t                        fn_shutdown;
static nvmlDeviceGetCount_v2_t               fn_get_count;
static nvmlDeviceGetHandleByIndex_v2_t       fn_get_handle;
static nvmlDeviceGetName_t                   fn_get_name;
static nvmlDeviceGetMemoryInfo_t             fn_get_memory;
static nvmlDeviceGetUtilizationRates_t       fn_get_util;
static nvmlDeviceGetTemperature_t            fn_get_temp;
static nvmlDeviceGetTopologyCommonAncestor_t fn_get_topo;
static nvmlDeviceGetNvLinkState_t            fn_get_nvlink_state;
static nvmlDeviceGetNvLinkRemotePciInfo_v2_t fn_get_nvlink_remote;
static nvmlDeviceGetPciInfo_v3_t             fn_get_pci;
static nvmlDeviceGetNvLinkVersion_t          fn_get_nvlink_version;
static nvmlDeviceGetMaxPcieLinkGeneration_t  fn_get_pcie_gen;
static nvmlDeviceGetMaxPcieLinkWidth_t       fn_get_pcie_width;

/* Cached device handles */
#define MAX_NVIDIA_GPUS 32
static nvmlDevice_t device_handles[MAX_NVIDIA_GPUS];
static unsigned int device_count = 0;

/* ------------------------------------------------------------------ */
/*  Helper: load symbol or fail                                       */
/* ------------------------------------------------------------------ */

#define LOAD_SYM(ptr, type, name) do {           \
    ptr = (type)dlsym(nvml_handle, name);        \
    if (!ptr) return 0;                          \
} while(0)

/* ------------------------------------------------------------------ */
/*  NVLink version → per-link bidirectional bandwidth (GB/s)          */
/* ------------------------------------------------------------------ */

static float nvlink_version_bandwidth(unsigned int version) {
    switch (version) {
        case 1: return 40.0f;   /* NVLink 1.0: 20 GB/s per direction */
        case 2: return 50.0f;   /* NVLink 2.0: 25 GB/s per direction */
        case 3: return 50.0f;   /* NVLink 3.0 (A100) */
        case 4: return 50.0f;   /* NVLink 4.0 (H100) */
        default: return 50.0f;  /* assume modern */
    }
}

/* PCIe gen → per-direction bandwidth (GB/s) for x16 */
static float pcie_gen_bandwidth(unsigned int gen, unsigned int width) {
    float per_lane;
    switch (gen) {
        case 1: per_lane = 0.25f;  break;
        case 2: per_lane = 0.5f;   break;
        case 3: per_lane = 0.985f; break;
        case 4: per_lane = 1.969f; break;
        case 5: per_lane = 3.938f; break;
        case 6: per_lane = 7.563f; break;
        default: per_lane = 1.969f; break;  /* assume gen4 */
    }
    return per_lane * (float)width;
}

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

int nvml_detect(void) {
    nvml_handle = dlopen("libnvidia-ml.so.1", RTLD_LAZY);
    if (!nvml_handle) {
        nvml_handle = dlopen("libnvidia-ml.so", RTLD_LAZY);
    }
    if (!nvml_handle) return 0;

    /* Load required symbols */
    LOAD_SYM(fn_init,              nvmlInit_v2_t,                         "nvmlInit_v2");
    LOAD_SYM(fn_shutdown,          nvmlShutdown_t,                        "nvmlShutdown");
    LOAD_SYM(fn_get_count,         nvmlDeviceGetCount_v2_t,               "nvmlDeviceGetCount_v2");
    LOAD_SYM(fn_get_handle,        nvmlDeviceGetHandleByIndex_v2_t,       "nvmlDeviceGetHandleByIndex_v2");
    LOAD_SYM(fn_get_name,          nvmlDeviceGetName_t,                   "nvmlDeviceGetName");
    LOAD_SYM(fn_get_memory,        nvmlDeviceGetMemoryInfo_t,             "nvmlDeviceGetMemoryInfo");
    LOAD_SYM(fn_get_util,          nvmlDeviceGetUtilizationRates_t,       "nvmlDeviceGetUtilizationRates");
    LOAD_SYM(fn_get_temp,          nvmlDeviceGetTemperature_t,            "nvmlDeviceGetTemperature");
    LOAD_SYM(fn_get_topo,          nvmlDeviceGetTopologyCommonAncestor_t, "nvmlDeviceGetTopologyCommonAncestor");
    LOAD_SYM(fn_get_pci,           nvmlDeviceGetPciInfo_v3_t,             "nvmlDeviceGetPciInfo_v3");

    /* Optional symbols — NVLink and PCIe bandwidth queries */
    fn_get_nvlink_state   = (nvmlDeviceGetNvLinkState_t)dlsym(nvml_handle, "nvmlDeviceGetNvLinkState");
    fn_get_nvlink_remote  = (nvmlDeviceGetNvLinkRemotePciInfo_v2_t)dlsym(nvml_handle, "nvmlDeviceGetNvLinkRemotePciInfo_v2");
    fn_get_nvlink_version = (nvmlDeviceGetNvLinkVersion_t)dlsym(nvml_handle, "nvmlDeviceGetNvLinkVersion");
    fn_get_pcie_gen       = (nvmlDeviceGetMaxPcieLinkGeneration_t)dlsym(nvml_handle, "nvmlDeviceGetMaxPcieLinkGeneration");
    fn_get_pcie_width     = (nvmlDeviceGetMaxPcieLinkWidth_t)dlsym(nvml_handle, "nvmlDeviceGetMaxPcieLinkWidth");

    /* Initialize NVML */
    if (fn_init() != NVML_SUCCESS) {
        dlclose(nvml_handle);
        nvml_handle = NULL;
        return 0;
    }

    /* Get device count */
    if (fn_get_count(&device_count) != NVML_SUCCESS) {
        fn_shutdown();
        dlclose(nvml_handle);
        nvml_handle = NULL;
        return 0;
    }

    if (device_count == 0) {
        fn_shutdown();
        dlclose(nvml_handle);
        nvml_handle = NULL;
        return 0;
    }

    if (device_count > MAX_NVIDIA_GPUS)
        device_count = MAX_NVIDIA_GPUS;

    /* Cache device handles */
    for (unsigned int i = 0; i < device_count; i++) {
        if (fn_get_handle(i, &device_handles[i]) != NVML_SUCCESS) {
            fn_shutdown();
            dlclose(nvml_handle);
            nvml_handle = NULL;
            return 0;
        }
    }

    return (int)device_count;
}

int nvml_enumerate(struct gpu_device *out, int max_devices) {
    if (!nvml_handle || device_count == 0) return 0;

    int n = (int)device_count;
    if (n > max_devices) n = max_devices;

    for (int i = 0; i < n; i++) {
        memset(&out[i], 0, sizeof(struct gpu_device));
        out[i].id = i;
        out[i].vendor = GPU_VENDOR_NVIDIA;
        out[i].vendor_index = i;

        /* Name */
        char name[NVML_DEVICE_NAME_BUFFER_SIZE];
        if (fn_get_name(device_handles[i], name, sizeof(name)) == NVML_SUCCESS) {
            strncpy(out[i].name, name, 127);
            out[i].name[127] = '\0';
        } else {
            strncpy(out[i].name, "NVIDIA GPU", 127);
        }

        /* Memory */
        nvmlMemory_t mem;
        if (fn_get_memory(device_handles[i], &mem) == NVML_SUCCESS) {
            out[i].vram_total_mb = (uint64_t)(mem.total / (1024ULL * 1024ULL));
            out[i].vram_used_mb  = (uint64_t)(mem.used  / (1024ULL * 1024ULL));
        }

        /* Utilization */
        nvmlUtilization_t util;
        if (fn_get_util(device_handles[i], &util) == NVML_SUCCESS) {
            out[i].utilization_pct = (float)util.gpu;
        }

        /* Temperature */
        unsigned int temp = 0;
        if (fn_get_temp(device_handles[i], NVML_TEMPERATURE_GPU, &temp) == NVML_SUCCESS) {
            out[i].temperature_c = (int)temp;
        }
    }

    return n;
}

int nvml_topology(struct gpu_link *out, int max_links, int num_devices) {
    if (!nvml_handle || device_count < 2) return 0;

    int n = (int)device_count;
    if (n > num_devices) n = num_devices;
    int link_count = 0;

    /*
     * Strategy:
     * 1. For each GPU, get its PCI bus ID
     * 2. For each pair (i,j) where i<j, check NVLink connections first
     * 3. Count active NVLink lanes between i and j
     * 4. If no NVLink, fall back to PCIe topology level
     */

    /* Get PCI info for all devices (for matching NVLink remote endpoints) */
    nvmlPciInfo_t pci_info[MAX_NVIDIA_GPUS];
    for (int i = 0; i < n; i++) {
        if (fn_get_pci(device_handles[i], &pci_info[i]) != NVML_SUCCESS) {
            memset(&pci_info[i], 0, sizeof(nvmlPciInfo_t));
        }
    }

    for (int i = 0; i < n && link_count < max_links; i++) {
        for (int j = i + 1; j < n && link_count < max_links; j++) {

            /* Try NVLink first */
            int nvlink_lanes = 0;
            float nvlink_bw_per_lane = 0.0f;

            if (fn_get_nvlink_state && fn_get_nvlink_remote && fn_get_nvlink_version) {
                for (unsigned int link = 0; link < NVML_NVLINK_MAX_LINKS; link++) {
                    nvmlEnableState_t active = NVML_FEATURE_DISABLED;
                    if (fn_get_nvlink_state(device_handles[i], link, &active) != NVML_SUCCESS)
                        continue;
                    if (active != NVML_FEATURE_ENABLED)
                        continue;

                    /* Check if this link connects to GPU j */
                    nvmlPciInfo_t remote_pci;
                    if (fn_get_nvlink_remote(device_handles[i], link, &remote_pci) != NVML_SUCCESS)
                        continue;

                    if (strcmp(remote_pci.busId, pci_info[j].busId) == 0) {
                        nvlink_lanes++;

                        /* Get NVLink version for bandwidth calculation */
                        if (nvlink_bw_per_lane == 0.0f) {
                            unsigned int version = 0;
                            if (fn_get_nvlink_version(device_handles[i], link, &version) == NVML_SUCCESS) {
                                nvlink_bw_per_lane = nvlink_version_bandwidth(version);
                            } else {
                                nvlink_bw_per_lane = 50.0f; /* assume NVLink 3+ */
                            }
                        }
                    }
                }
            }

            if (nvlink_lanes > 0) {
                out[link_count].gpu_a = i;
                out[link_count].gpu_b = j;
                out[link_count].link_type = GPU_LINK_NVLINK;
                out[link_count].bandwidth_gbps = nvlink_bw_per_lane * (float)nvlink_lanes;
                link_count++;
                continue;
            }

            /* No NVLink — check PCIe topology */
            nvmlGpuTopologyLevel_t topo_level;
            if (fn_get_topo(device_handles[i], device_handles[j], &topo_level) != NVML_SUCCESS)
                continue;

            /* Only report if GPUs share at least a host bridge */
            if (topo_level > NVML_TOPOLOGY_SYSTEM)
                continue;

            /* Estimate PCIe bandwidth */
            float pcie_bw = 31.5f; /* default: gen4 x16 */
            if (fn_get_pcie_gen && fn_get_pcie_width) {
                unsigned int gen = 0, width = 0;
                /* Use the minimum of the two GPUs' PCIe capabilities */
                unsigned int gen_i = 4, gen_j = 4, width_i = 16, width_j = 16;
                if (fn_get_pcie_gen(device_handles[i], &gen_i) != NVML_SUCCESS) gen_i = 4;
                if (fn_get_pcie_gen(device_handles[j], &gen_j) != NVML_SUCCESS) gen_j = 4;
                if (fn_get_pcie_width(device_handles[i], &width_i) != NVML_SUCCESS) width_i = 16;
                if (fn_get_pcie_width(device_handles[j], &width_j) != NVML_SUCCESS) width_j = 16;
                gen = (gen_i < gen_j) ? gen_i : gen_j;
                width = (width_i < width_j) ? width_i : width_j;
                pcie_bw = pcie_gen_bandwidth(gen, width);
            }

            out[link_count].gpu_a = i;
            out[link_count].gpu_b = j;
            out[link_count].link_type = GPU_LINK_PCIE;
            out[link_count].bandwidth_gbps = pcie_bw;
            link_count++;
        }
    }

    return link_count;
}

int nvml_refresh(struct gpu_device *out, int max_devices) {
    /* Same as enumerate — re-queries live stats */
    return nvml_enumerate(out, max_devices);
}

void nvml_shutdown(void) {
    if (nvml_handle) {
        fn_shutdown();
        dlclose(nvml_handle);
        nvml_handle = NULL;
        device_count = 0;
    }
}

#else /* !__linux__ */

/* NVML is Linux-only (and Windows, but we don't support that path here) */
int  nvml_detect(void)                                              { return 0; }
int  nvml_enumerate(struct gpu_device *out, int max_devices)        { (void)out; (void)max_devices; return 0; }
int  nvml_topology(struct gpu_link *out, int max_links, int n)      { (void)out; (void)max_links; (void)n; return 0; }
int  nvml_refresh(struct gpu_device *out, int max_devices)          { (void)out; (void)max_devices; return 0; }
void nvml_shutdown(void)                                            {}

#endif /* __linux__ */
