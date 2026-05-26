/*
 * AMD ROCm SMI backend — dlopen-based, no compile-time linking.
 *
 * Loads librocm_smi64.so at runtime. Queries device name, VRAM,
 * utilization, temperature, and XGMI/PCIe topology.
 */

#ifdef __linux__

#include "rocm.h"
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <stdint.h>

/* ------------------------------------------------------------------ */
/*  ROCm SMI type definitions                                         */
/* ------------------------------------------------------------------ */

typedef enum {
    RSMI_STATUS_SUCCESS             = 0x0,
    RSMI_STATUS_INVALID_ARGS        = 0x1,
    RSMI_STATUS_NOT_SUPPORTED       = 0x2,
    RSMI_STATUS_FILE_ERROR          = 0x3,
    RSMI_STATUS_PERMISSION          = 0x4,
    RSMI_STATUS_OUT_OF_RESOURCES    = 0x5,
    RSMI_STATUS_INTERNAL_EXCEPTION  = 0x6,
    RSMI_STATUS_INPUT_OUT_OF_BOUNDS = 0x7,
    RSMI_STATUS_INIT_ERROR          = 0x8,
    RSMI_STATUS_NOT_FOUND           = 0xA,
    RSMI_STATUS_INSUFFICIENT_SIZE   = 0xB,
    RSMI_STATUS_NO_DATA             = 0xE,
    RSMI_STATUS_UNKNOWN_ERROR       = 0xFFFFFFFF
} rsmi_status_t;

typedef enum {
    RSMI_MEM_TYPE_VRAM     = 0,
    RSMI_MEM_TYPE_VIS_VRAM = 1,
    RSMI_MEM_TYPE_GTT      = 2
} rsmi_memory_type_t;

typedef enum {
    RSMI_TEMP_CURRENT = 0x0
} rsmi_temperature_metric_t;

typedef enum {
    RSMI_TEMP_TYPE_EDGE     = 0,
    RSMI_TEMP_TYPE_JUNCTION = 1,
    RSMI_TEMP_TYPE_MEMORY   = 2
} rsmi_temperature_type_t;

typedef enum {
    RSMI_IOLINK_TYPE_UNDEFINED  = 0,
    RSMI_IOLINK_TYPE_PCIEXPRESS = 1,
    RSMI_IOLINK_TYPE_XGMI       = 2
} rsmi_io_link_type_t;

#define RSMI_MAX_NUM_FREQUENCIES 33

typedef struct {
    uint32_t num_supported;
    uint32_t current;
    uint64_t frequency[RSMI_MAX_NUM_FREQUENCIES];
} rsmi_frequencies_t;

typedef struct {
    rsmi_frequencies_t transfer_rate;
    uint32_t lanes[RSMI_MAX_NUM_FREQUENCIES];
} rsmi_pcie_bandwidth_t;

/* ------------------------------------------------------------------ */
/*  dlopen function pointer typedefs                                  */
/* ------------------------------------------------------------------ */

typedef rsmi_status_t (*rsmi_init_t)(uint64_t flags);
typedef rsmi_status_t (*rsmi_shut_down_t)(void);
typedef rsmi_status_t (*rsmi_num_monitor_devices_t)(uint32_t *num);
typedef rsmi_status_t (*rsmi_dev_name_get_t)(uint32_t dv, char *name, size_t len);
typedef rsmi_status_t (*rsmi_dev_memory_total_get_t)(uint32_t dv, rsmi_memory_type_t type, uint64_t *total);
typedef rsmi_status_t (*rsmi_dev_memory_usage_get_t)(uint32_t dv, rsmi_memory_type_t type, uint64_t *used);
typedef rsmi_status_t (*rsmi_dev_busy_percent_get_t)(uint32_t dv, uint32_t *busy);
typedef rsmi_status_t (*rsmi_dev_temp_metric_get_t)(uint32_t dv, uint32_t sensor, rsmi_temperature_metric_t metric, int64_t *temp);
typedef rsmi_status_t (*rsmi_topo_get_link_type_t)(uint32_t src, uint32_t dst, uint64_t *hops, rsmi_io_link_type_t *type);
typedef rsmi_status_t (*rsmi_topo_get_link_weight_t)(uint32_t src, uint32_t dst, uint64_t *weight);
typedef rsmi_status_t (*rsmi_dev_pci_bandwidth_get_t)(uint32_t dv, rsmi_pcie_bandwidth_t *bw);
typedef rsmi_status_t (*rsmi_minmax_bandwidth_get_t)(uint32_t src, uint32_t dst, uint64_t *min_bw, uint64_t *max_bw);

/* ------------------------------------------------------------------ */
/*  Static function pointers                                          */
/* ------------------------------------------------------------------ */

static void *rocm_handle = NULL;

static rsmi_init_t                  fn_init;
static rsmi_shut_down_t             fn_shut_down;
static rsmi_num_monitor_devices_t   fn_num_devices;
static rsmi_dev_name_get_t          fn_name_get;
static rsmi_dev_memory_total_get_t  fn_mem_total;
static rsmi_dev_memory_usage_get_t  fn_mem_usage;
static rsmi_dev_busy_percent_get_t  fn_busy_pct;
static rsmi_dev_temp_metric_get_t   fn_temp_get;
static rsmi_topo_get_link_type_t    fn_topo_link_type;
static rsmi_topo_get_link_weight_t  fn_topo_link_weight;
static rsmi_dev_pci_bandwidth_get_t fn_pci_bw;
static rsmi_minmax_bandwidth_get_t  fn_minmax_bw;  /* optional, ROCm 5.0+ */

static uint32_t dev_count = 0;

/* ------------------------------------------------------------------ */
/*  Helper                                                            */
/* ------------------------------------------------------------------ */

#define LOAD_SYM(ptr, type, name) do {           \
    ptr = (type)dlsym(rocm_handle, name);        \
    if (!ptr) return 0;                          \
} while(0)

/* XGMI bandwidth estimate by link weight (heuristic) */
static float xgmi_bandwidth_estimate(uint64_t weight) {
    /*
     * ROCm link weight is relative — lower = faster.
     * weight 15 = direct XGMI, typically ~200 GB/s bidirectional for MI250X
     * weight 40 = indirect (2 hops)
     * weight 50+ = PCIe
     * These are approximate heuristics.
     */
    if (weight <= 15)  return 200.0f;
    if (weight <= 30)  return 100.0f;
    if (weight <= 50)  return 50.0f;
    return 32.0f;
}

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

int rocm_detect(void) {
    rocm_handle = dlopen("librocm_smi64.so", RTLD_LAZY);
    if (!rocm_handle) {
        /* Try with full path */
        rocm_handle = dlopen("/opt/rocm/lib/librocm_smi64.so", RTLD_LAZY);
    }
    if (!rocm_handle) return 0;

    /* Load required symbols */
    LOAD_SYM(fn_init,          rsmi_init_t,                  "rsmi_init");
    LOAD_SYM(fn_shut_down,     rsmi_shut_down_t,             "rsmi_shut_down");
    LOAD_SYM(fn_num_devices,   rsmi_num_monitor_devices_t,   "rsmi_num_monitor_devices");
    LOAD_SYM(fn_name_get,      rsmi_dev_name_get_t,          "rsmi_dev_name_get");
    LOAD_SYM(fn_mem_total,     rsmi_dev_memory_total_get_t,  "rsmi_dev_memory_total_get");
    LOAD_SYM(fn_mem_usage,     rsmi_dev_memory_usage_get_t,  "rsmi_dev_memory_usage_get");
    LOAD_SYM(fn_busy_pct,      rsmi_dev_busy_percent_get_t,  "rsmi_dev_busy_percent_get");
    LOAD_SYM(fn_temp_get,      rsmi_dev_temp_metric_get_t,   "rsmi_dev_temp_metric_get");
    LOAD_SYM(fn_topo_link_type,   rsmi_topo_get_link_type_t,   "rsmi_topo_get_link_type");
    LOAD_SYM(fn_topo_link_weight, rsmi_topo_get_link_weight_t, "rsmi_topo_get_link_weight");
    LOAD_SYM(fn_pci_bw,        rsmi_dev_pci_bandwidth_get_t, "rsmi_dev_pci_bandwidth_get");

    /* Optional: minmax bandwidth (ROCm 5.0+) */
    fn_minmax_bw = (rsmi_minmax_bandwidth_get_t)dlsym(rocm_handle, "rsmi_minmax_bandwidth_get");

    /* Initialize ROCm SMI */
    if (fn_init(0) != RSMI_STATUS_SUCCESS) {
        dlclose(rocm_handle);
        rocm_handle = NULL;
        return 0;
    }

    /* Get device count */
    if (fn_num_devices(&dev_count) != RSMI_STATUS_SUCCESS || dev_count == 0) {
        fn_shut_down();
        dlclose(rocm_handle);
        rocm_handle = NULL;
        return 0;
    }

    return (int)dev_count;
}

int rocm_enumerate(struct gpu_device *out, int max_devices) {
    if (!rocm_handle || dev_count == 0) return 0;

    int n = (int)dev_count;
    if (n > max_devices) n = max_devices;

    for (int i = 0; i < n; i++) {
        uint32_t idx = (uint32_t)i;
        memset(&out[i], 0, sizeof(struct gpu_device));
        out[i].id = i;
        out[i].vendor = GPU_VENDOR_AMD;
        out[i].vendor_index = i;

        /* Name */
        char name[256];
        if (fn_name_get(idx, name, sizeof(name)) == RSMI_STATUS_SUCCESS) {
            strncpy(out[i].name, name, 127);
            out[i].name[127] = '\0';
        } else {
            strncpy(out[i].name, "AMD GPU", 127);
        }

        /* VRAM (returned in bytes, convert to MB) */
        uint64_t total = 0, used = 0;
        if (fn_mem_total(idx, RSMI_MEM_TYPE_VRAM, &total) == RSMI_STATUS_SUCCESS) {
            out[i].vram_total_mb = total / (1024ULL * 1024ULL);
        }
        if (fn_mem_usage(idx, RSMI_MEM_TYPE_VRAM, &used) == RSMI_STATUS_SUCCESS) {
            out[i].vram_used_mb = used / (1024ULL * 1024ULL);
        }

        /* Utilization */
        uint32_t busy = 0;
        if (fn_busy_pct(idx, &busy) == RSMI_STATUS_SUCCESS) {
            out[i].utilization_pct = (float)busy;
        }

        /* Temperature (millidegrees → degrees) */
        int64_t temp = 0;
        if (fn_temp_get(idx, (uint32_t)RSMI_TEMP_TYPE_EDGE, RSMI_TEMP_CURRENT, &temp) == RSMI_STATUS_SUCCESS) {
            out[i].temperature_c = (int)(temp / 1000);
        }
    }

    return n;
}

int rocm_topology(struct gpu_link *out, int max_links, int num_devices) {
    if (!rocm_handle || dev_count < 2) return 0;

    int n = (int)dev_count;
    if (n > num_devices) n = num_devices;
    int link_count = 0;

    for (int i = 0; i < n && link_count < max_links; i++) {
        for (int j = i + 1; j < n && link_count < max_links; j++) {
            uint64_t hops = 0;
            rsmi_io_link_type_t link_type = RSMI_IOLINK_TYPE_UNDEFINED;

            if (fn_topo_link_type((uint32_t)i, (uint32_t)j, &hops, &link_type) != RSMI_STATUS_SUCCESS)
                continue;

            if (link_type == RSMI_IOLINK_TYPE_XGMI) {
                float bw = 0.0f;

                /* Try rsmi_minmax_bandwidth_get first (gives MB/s) */
                if (fn_minmax_bw) {
                    uint64_t min_bw = 0, max_bw = 0;
                    if (fn_minmax_bw((uint32_t)i, (uint32_t)j, &min_bw, &max_bw) == RSMI_STATUS_SUCCESS && max_bw > 0) {
                        bw = (float)max_bw / 1000.0f;  /* MB/s → GB/s */
                    }
                }

                /* Fallback: estimate from link weight */
                if (bw == 0.0f) {
                    uint64_t weight = 0;
                    if (fn_topo_link_weight((uint32_t)i, (uint32_t)j, &weight) == RSMI_STATUS_SUCCESS) {
                        bw = xgmi_bandwidth_estimate(weight);
                    } else {
                        bw = 200.0f; /* reasonable default for XGMI */
                    }
                }

                out[link_count].gpu_a = i;
                out[link_count].gpu_b = j;
                out[link_count].link_type = GPU_LINK_XGMI;
                out[link_count].bandwidth_gbps = bw;
                link_count++;

            } else if (link_type == RSMI_IOLINK_TYPE_PCIEXPRESS) {
                /* PCIe bandwidth from device i (use as estimate for the link) */
                float bw = 31.5f; /* default: gen4 x16 */
                rsmi_pcie_bandwidth_t pci_bw;
                if (fn_pci_bw((uint32_t)i, &pci_bw) == RSMI_STATUS_SUCCESS) {
                    uint32_t cur = pci_bw.transfer_rate.current;
                    if (cur < pci_bw.transfer_rate.num_supported) {
                        /* transfer rate in MT/s, lanes count */
                        uint64_t rate = pci_bw.transfer_rate.frequency[cur];
                        uint32_t lanes = pci_bw.lanes[cur];
                        /* PCIe uses 128b/130b encoding (gen3+) */
                        bw = (float)(rate * (uint64_t)lanes) / 1000000000.0f * (128.0f / 130.0f);
                    }
                }

                out[link_count].gpu_a = i;
                out[link_count].gpu_b = j;
                out[link_count].link_type = GPU_LINK_PCIE;
                out[link_count].bandwidth_gbps = bw;
                link_count++;
            }
        }
    }

    return link_count;
}

int rocm_refresh(struct gpu_device *out, int max_devices) {
    return rocm_enumerate(out, max_devices);
}

void rocm_shutdown(void) {
    if (rocm_handle) {
        fn_shut_down();
        dlclose(rocm_handle);
        rocm_handle = NULL;
        dev_count = 0;
    }
}

#else /* !__linux__ */

int  rocm_detect(void)                                              { return 0; }
int  rocm_enumerate(struct gpu_device *out, int max_devices)        { (void)out; (void)max_devices; return 0; }
int  rocm_topology(struct gpu_link *out, int max_links, int n)      { (void)out; (void)max_links; (void)n; return 0; }
int  rocm_refresh(struct gpu_device *out, int max_devices)          { (void)out; (void)max_devices; return 0; }
void rocm_shutdown(void)                                            {}

#endif /* __linux__ */
