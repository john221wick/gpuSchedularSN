/*
 * Intel Level Zero + Sysman backend — dlopen-based, no compile-time linking.
 *
 * Loads libze_loader.so at runtime. Uses core Level Zero for device
 * discovery and properties, Sysman (zes_*) for monitoring (memory usage,
 * temperature) and fabric port enumeration (XeLink topology).
 *
 * Requires ZES_ENABLE_SYSMAN=1 in environment (set automatically).
 */

#ifdef __linux__

#include "levelzero.h"
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <stdint.h>

/* ------------------------------------------------------------------ */
/*  Level Zero type definitions                                       */
/* ------------------------------------------------------------------ */

typedef struct _ze_driver_handle_t  *ze_driver_handle_t;
typedef struct _ze_device_handle_t  *ze_device_handle_t;
typedef struct _zes_device_handle_t *zes_device_handle_t;
typedef struct _zes_mem_handle_t    *zes_mem_handle_t;
typedef struct _zes_temp_handle_t   *zes_temp_handle_t;
typedef struct _zes_fabric_port_handle_t *zes_fabric_port_handle_t;

typedef enum {
    ZE_RESULT_SUCCESS               = 0,
    ZE_RESULT_NOT_READY             = 1,
    ZE_RESULT_ERROR_UNINITIALIZED   = 0x78000001,
    ZE_RESULT_ERROR_UNSUPPORTED_FEATURE = 0x78000003,
    ZE_RESULT_ERROR_NOT_AVAILABLE   = 0x78000013,
    ZE_RESULT_ERROR_UNKNOWN         = 0x7FFFFFFE
} ze_result_t;

typedef uint32_t ze_init_flags_t;
#define ZE_INIT_FLAG_GPU_ONLY 0x01

typedef enum {
    ZE_DEVICE_TYPE_GPU  = 1,
    ZE_DEVICE_TYPE_CPU  = 2,
    ZE_DEVICE_TYPE_FPGA = 3,
    ZE_DEVICE_TYPE_MCA  = 4,
    ZE_DEVICE_TYPE_VPU  = 5
} ze_device_type_t;

typedef enum {
    ZE_STRUCTURE_TYPE_DEVICE_PROPERTIES        = 0x3,
    ZE_STRUCTURE_TYPE_DEVICE_MEMORY_PROPERTIES = 0x7
} ze_structure_type_t;

typedef enum {
    ZES_STRUCTURE_TYPE_MEM_STATE              = 0x7,
    ZES_STRUCTURE_TYPE_TEMP_PROPERTIES        = 0x8,
    ZES_STRUCTURE_TYPE_FABRIC_PORT_PROPERTIES = 0x9,
    ZES_STRUCTURE_TYPE_FABRIC_PORT_STATE      = 0xA
} zes_structure_type_t;

typedef uint32_t ze_device_property_flags_t;

#define ZE_MAX_DEVICE_NAME 256

typedef struct {
    uint8_t id[16];
} ze_device_uuid_t;

typedef struct {
    ze_structure_type_t         stype;
    void                       *pNext;
    ze_device_type_t            type;
    uint32_t                    vendorId;
    uint32_t                    deviceId;
    ze_device_property_flags_t  flags;
    uint32_t                    subdeviceId;
    uint32_t                    coreClockRate;
    uint64_t                    maxMemAllocSize;
    uint32_t                    maxHardwareContexts;
    uint32_t                    maxCommandQueuePriority;
    uint32_t                    numThreadsPerEU;
    uint32_t                    physicalEUSimdWidth;
    uint32_t                    numEUsPerSubslice;
    uint32_t                    numSubslicesPerSlice;
    uint32_t                    numSlices;
    uint64_t                    timerResolution;
    uint32_t                    timestampValidBits;
    uint32_t                    kernelTimestampValidBits;
    ze_device_uuid_t            uuid;
    char                        name[ZE_MAX_DEVICE_NAME];
} ze_device_properties_t;

typedef struct {
    ze_structure_type_t stype;
    void               *pNext;
    uint32_t            flags;
    uint32_t            maxClockRate;
    uint32_t            maxBusWidth;
    uint64_t            totalSize;
    char                name[ZE_MAX_DEVICE_NAME];
} ze_device_memory_properties_t;

/* Sysman memory state */
typedef enum {
    ZES_MEM_HEALTH_UNKNOWN  = 0,
    ZES_MEM_HEALTH_OK       = 1,
    ZES_MEM_HEALTH_DEGRADED = 2,
    ZES_MEM_HEALTH_CRITICAL = 3,
    ZES_MEM_HEALTH_REPLACE  = 4
} zes_mem_health_t;

typedef struct {
    zes_structure_type_t stype;
    const void          *pNext;
    zes_mem_health_t     health;
    uint64_t             free;
    uint64_t             size;
} zes_mem_state_t;

/* Sysman temperature */
typedef enum {
    ZES_TEMP_SENSORS_GLOBAL = 0,
    ZES_TEMP_SENSORS_GPU    = 1,
    ZES_TEMP_SENSORS_MEMORY = 2
} zes_temp_sensors_t;

typedef struct {
    zes_structure_type_t stype;
    void                *pNext;
    zes_temp_sensors_t   type;
    int32_t              onSubdevice;
    uint32_t             subdeviceId;
    double               maxTemperature;
    int32_t              isCriticalTempSupported;
    int32_t              isThreshold1Supported;
    int32_t              isThreshold2Supported;
} zes_temp_properties_t;

/* Sysman fabric ports (XeLink) */
typedef struct {
    uint32_t fabricId;
    uint32_t attachId;
    uint8_t  portNumber;
} zes_fabric_port_id_t;

typedef enum {
    ZES_FABRIC_PORT_STATUS_UNKNOWN  = 0,
    ZES_FABRIC_PORT_STATUS_HEALTHY  = 1,
    ZES_FABRIC_PORT_STATUS_DEGRADED = 2,
    ZES_FABRIC_PORT_STATUS_FAILED   = 3,
    ZES_FABRIC_PORT_STATUS_DISABLED = 4
} zes_fabric_port_status_t;

typedef struct {
    int64_t bitRate;
    int32_t width;
} zes_fabric_port_speed_t;

#define ZES_STRING_PROPERTY_SIZE 64

typedef struct {
    zes_structure_type_t       stype;
    void                      *pNext;
    char                       model[ZES_STRING_PROPERTY_SIZE];
    uint8_t                    onSubdevice;
    uint32_t                   subdeviceId;
    zes_fabric_port_id_t       portId;
    zes_fabric_port_speed_t    maxRxSpeed;
    zes_fabric_port_speed_t    maxTxSpeed;
} zes_fabric_port_properties_t;

typedef struct {
    zes_structure_type_t       stype;
    const void                *pNext;
    zes_fabric_port_status_t   status;
    uint32_t                   qualityIssues;
    uint32_t                   failureReasons;
    zes_fabric_port_id_t       remotePortId;
    int64_t                    rxSpeed;
    int64_t                    txSpeed;
} zes_fabric_port_state_t;

/* ------------------------------------------------------------------ */
/*  dlopen function pointer typedefs                                  */
/* ------------------------------------------------------------------ */

typedef ze_result_t (*pfn_zeInit)(ze_init_flags_t);
typedef ze_result_t (*pfn_zeDriverGet)(uint32_t *, ze_driver_handle_t *);
typedef ze_result_t (*pfn_zeDeviceGet)(ze_driver_handle_t, uint32_t *, ze_device_handle_t *);
typedef ze_result_t (*pfn_zeDeviceGetProperties)(ze_device_handle_t, ze_device_properties_t *);
typedef ze_result_t (*pfn_zeDeviceGetMemoryProperties)(ze_device_handle_t, uint32_t *, ze_device_memory_properties_t *);
typedef ze_result_t (*pfn_zesDeviceEnumMemoryModules)(zes_device_handle_t, uint32_t *, zes_mem_handle_t *);
typedef ze_result_t (*pfn_zesMemoryGetState)(zes_mem_handle_t, zes_mem_state_t *);
typedef ze_result_t (*pfn_zesDeviceEnumTemperatureSensors)(zes_device_handle_t, uint32_t *, zes_temp_handle_t *);
typedef ze_result_t (*pfn_zesTemperatureGetState)(zes_temp_handle_t, double *);
typedef ze_result_t (*pfn_zesTemperatureGetProperties)(zes_temp_handle_t, zes_temp_properties_t *);
typedef ze_result_t (*pfn_zesDeviceEnumFabricPorts)(zes_device_handle_t, uint32_t *, zes_fabric_port_handle_t *);
typedef ze_result_t (*pfn_zesFabricPortGetProperties)(zes_fabric_port_handle_t, zes_fabric_port_properties_t *);
typedef ze_result_t (*pfn_zesFabricPortGetState)(zes_fabric_port_handle_t, zes_fabric_port_state_t *);

/* ------------------------------------------------------------------ */
/*  Static function pointers                                          */
/* ------------------------------------------------------------------ */

static void *ze_handle = NULL;

static pfn_zeInit                          fn_init;
static pfn_zeDriverGet                     fn_driver_get;
static pfn_zeDeviceGet                     fn_device_get;
static pfn_zeDeviceGetProperties           fn_dev_props;
static pfn_zeDeviceGetMemoryProperties     fn_mem_props;
static pfn_zesDeviceEnumMemoryModules      fn_enum_mem;
static pfn_zesMemoryGetState               fn_mem_state;
static pfn_zesDeviceEnumTemperatureSensors fn_enum_temp;
static pfn_zesTemperatureGetState          fn_temp_state;
static pfn_zesTemperatureGetProperties     fn_temp_props;
static pfn_zesDeviceEnumFabricPorts        fn_enum_fabric;
static pfn_zesFabricPortGetProperties      fn_fabric_props;
static pfn_zesFabricPortGetState           fn_fabric_state;

/* Cached device handles */
#define MAX_INTEL_GPUS 32
static ze_device_handle_t gpu_handles[MAX_INTEL_GPUS];
static int                gpu_count = 0;

/* ------------------------------------------------------------------ */
/*  Helper                                                            */
/* ------------------------------------------------------------------ */

#define LOAD_SYM(ptr, type, name) do {           \
    ptr = (type)dlsym(ze_handle, name);          \
    if (!ptr) return 0;                          \
} while(0)

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

int ze_detect(void) {
    /* Must set before zeInit for Sysman to work */
    setenv("ZES_ENABLE_SYSMAN", "1", 0);

    ze_handle = dlopen("libze_loader.so", RTLD_LAZY);
    if (!ze_handle) {
        ze_handle = dlopen("libze_loader.so.1", RTLD_LAZY);
    }
    if (!ze_handle) return 0;

    /* Load required core symbols */
    LOAD_SYM(fn_init,       pfn_zeInit,                      "zeInit");
    LOAD_SYM(fn_driver_get, pfn_zeDriverGet,                 "zeDriverGet");
    LOAD_SYM(fn_device_get, pfn_zeDeviceGet,                 "zeDeviceGet");
    LOAD_SYM(fn_dev_props,  pfn_zeDeviceGetProperties,       "zeDeviceGetProperties");
    LOAD_SYM(fn_mem_props,  pfn_zeDeviceGetMemoryProperties, "zeDeviceGetMemoryProperties");

    /* Load Sysman symbols — optional (some drivers may not support all) */
    fn_enum_mem     = (pfn_zesDeviceEnumMemoryModules)dlsym(ze_handle, "zesDeviceEnumMemoryModules");
    fn_mem_state    = (pfn_zesMemoryGetState)dlsym(ze_handle, "zesMemoryGetState");
    fn_enum_temp    = (pfn_zesDeviceEnumTemperatureSensors)dlsym(ze_handle, "zesDeviceEnumTemperatureSensors");
    fn_temp_state   = (pfn_zesTemperatureGetState)dlsym(ze_handle, "zesTemperatureGetState");
    fn_temp_props   = (pfn_zesTemperatureGetProperties)dlsym(ze_handle, "zesTemperatureGetProperties");
    fn_enum_fabric  = (pfn_zesDeviceEnumFabricPorts)dlsym(ze_handle, "zesDeviceEnumFabricPorts");
    fn_fabric_props = (pfn_zesFabricPortGetProperties)dlsym(ze_handle, "zesFabricPortGetProperties");
    fn_fabric_state = (pfn_zesFabricPortGetState)dlsym(ze_handle, "zesFabricPortGetState");

    /* Initialize Level Zero */
    if (fn_init(ZE_INIT_FLAG_GPU_ONLY) != ZE_RESULT_SUCCESS) {
        dlclose(ze_handle);
        ze_handle = NULL;
        return 0;
    }

    /* Discover drivers */
    uint32_t driver_count = 0;
    if (fn_driver_get(&driver_count, NULL) != ZE_RESULT_SUCCESS || driver_count == 0) {
        dlclose(ze_handle);
        ze_handle = NULL;
        return 0;
    }

    ze_driver_handle_t *drivers = (ze_driver_handle_t *)calloc(driver_count, sizeof(ze_driver_handle_t));
    if (!drivers) {
        dlclose(ze_handle);
        ze_handle = NULL;
        return 0;
    }
    fn_driver_get(&driver_count, drivers);

    /* Enumerate GPU devices across all drivers */
    gpu_count = 0;
    for (uint32_t d = 0; d < driver_count && gpu_count < MAX_INTEL_GPUS; d++) {
        uint32_t dev_count = 0;
        if (fn_device_get(drivers[d], &dev_count, NULL) != ZE_RESULT_SUCCESS || dev_count == 0)
            continue;

        ze_device_handle_t *devs = (ze_device_handle_t *)calloc(dev_count, sizeof(ze_device_handle_t));
        if (!devs) continue;
        fn_device_get(drivers[d], &dev_count, devs);

        for (uint32_t i = 0; i < dev_count && gpu_count < MAX_INTEL_GPUS; i++) {
            ze_device_properties_t props;
            memset(&props, 0, sizeof(props));
            props.stype = ZE_STRUCTURE_TYPE_DEVICE_PROPERTIES;
            props.pNext = NULL;

            if (fn_dev_props(devs[i], &props) != ZE_RESULT_SUCCESS)
                continue;

            /* Only count GPU devices */
            if (props.type != ZE_DEVICE_TYPE_GPU)
                continue;

            /* Only count Intel vendor (0x8086) */
            if (props.vendorId != 0x8086)
                continue;

            gpu_handles[gpu_count] = devs[i];
            gpu_count++;
        }
        free(devs);
    }
    free(drivers);

    if (gpu_count == 0) {
        dlclose(ze_handle);
        ze_handle = NULL;
        return 0;
    }

    return gpu_count;
}

int ze_enumerate(struct gpu_device *out, int max_devices) {
    if (!ze_handle || gpu_count == 0) return 0;

    int n = gpu_count;
    if (n > max_devices) n = max_devices;

    for (int i = 0; i < n; i++) {
        memset(&out[i], 0, sizeof(struct gpu_device));
        out[i].id = i;
        out[i].vendor = GPU_VENDOR_INTEL;
        out[i].vendor_index = i;

        /* Device properties (name, etc) */
        ze_device_properties_t props;
        memset(&props, 0, sizeof(props));
        props.stype = ZE_STRUCTURE_TYPE_DEVICE_PROPERTIES;
        if (fn_dev_props(gpu_handles[i], &props) == ZE_RESULT_SUCCESS) {
            strncpy(out[i].name, props.name, 127);
            out[i].name[127] = '\0';
        } else {
            strncpy(out[i].name, "Intel GPU", 127);
        }

        /* Total VRAM from core memory properties */
        uint32_t mem_count = 0;
        if (fn_mem_props(gpu_handles[i], &mem_count, NULL) == ZE_RESULT_SUCCESS && mem_count > 0) {
            ze_device_memory_properties_t *mprops =
                (ze_device_memory_properties_t *)calloc(mem_count, sizeof(ze_device_memory_properties_t));
            if (mprops) {
                for (uint32_t m = 0; m < mem_count; m++) {
                    mprops[m].stype = ZE_STRUCTURE_TYPE_DEVICE_MEMORY_PROPERTIES;
                    mprops[m].pNext = NULL;
                }
                if (fn_mem_props(gpu_handles[i], &mem_count, mprops) == ZE_RESULT_SUCCESS) {
                    uint64_t total_bytes = 0;
                    for (uint32_t m = 0; m < mem_count; m++) {
                        total_bytes += mprops[m].totalSize;
                    }
                    out[i].vram_total_mb = total_bytes / (1024ULL * 1024ULL);
                }
                free(mprops);
            }
        }

        /* Used VRAM from Sysman memory modules */
        if (fn_enum_mem && fn_mem_state) {
            zes_device_handle_t sys_dev = (zes_device_handle_t)gpu_handles[i];
            uint32_t mod_count = 0;
            if (fn_enum_mem(sys_dev, &mod_count, NULL) == ZE_RESULT_SUCCESS && mod_count > 0) {
                zes_mem_handle_t *mods = (zes_mem_handle_t *)calloc(mod_count, sizeof(zes_mem_handle_t));
                if (mods) {
                    fn_enum_mem(sys_dev, &mod_count, mods);
                    uint64_t total_used = 0;
                    for (uint32_t m = 0; m < mod_count; m++) {
                        zes_mem_state_t state;
                        memset(&state, 0, sizeof(state));
                        state.stype = ZES_STRUCTURE_TYPE_MEM_STATE;
                        if (fn_mem_state(mods[m], &state) == ZE_RESULT_SUCCESS) {
                            if (state.size > state.free) {
                                total_used += (state.size - state.free);
                            }
                        }
                    }
                    out[i].vram_used_mb = total_used / (1024ULL * 1024ULL);
                    free(mods);
                }
            }
        }

        /* Temperature from Sysman */
        if (fn_enum_temp && fn_temp_state && fn_temp_props) {
            zes_device_handle_t sys_dev = (zes_device_handle_t)gpu_handles[i];
            uint32_t temp_count = 0;
            if (fn_enum_temp(sys_dev, &temp_count, NULL) == ZE_RESULT_SUCCESS && temp_count > 0) {
                zes_temp_handle_t *temps = (zes_temp_handle_t *)calloc(temp_count, sizeof(zes_temp_handle_t));
                if (temps) {
                    fn_enum_temp(sys_dev, &temp_count, temps);
                    /* Find GPU sensor, fall back to first available */
                    int found = 0;
                    for (uint32_t t = 0; t < temp_count; t++) {
                        zes_temp_properties_t tprops;
                        memset(&tprops, 0, sizeof(tprops));
                        tprops.stype = ZES_STRUCTURE_TYPE_TEMP_PROPERTIES;
                        if (fn_temp_props(temps[t], &tprops) == ZE_RESULT_SUCCESS) {
                            if (tprops.type == ZES_TEMP_SENSORS_GPU) {
                                double temp_val = 0.0;
                                if (fn_temp_state(temps[t], &temp_val) == ZE_RESULT_SUCCESS) {
                                    out[i].temperature_c = (int)temp_val;
                                    found = 1;
                                }
                                break;
                            }
                        }
                    }
                    if (!found && temp_count > 0) {
                        double temp_val = 0.0;
                        if (fn_temp_state(temps[0], &temp_val) == ZE_RESULT_SUCCESS) {
                            out[i].temperature_c = (int)temp_val;
                        }
                    }
                    free(temps);
                }
            }
        }

        /*
         * Utilization: Level Zero has no direct "busy %" API.
         * Would need zesEngineGetActivity with delta timestamps.
         * Report 0 for now — accurate util requires polling.
         */
        out[i].utilization_pct = 0.0f;
    }

    return n;
}

int ze_topology(struct gpu_link *out, int max_links, int num_devices) {
    if (!ze_handle || gpu_count < 2) return 0;
    if (!fn_enum_fabric || !fn_fabric_props || !fn_fabric_state) return 0;

    int n = gpu_count;
    if (n > num_devices) n = num_devices;

    /*
     * XeLink topology discovery:
     * 1. For each GPU, enumerate fabric ports
     * 2. Get each port's local portId and model string
     * 3. Get each port's remote portId from state
     * 4. Build fabricId → device index map
     * 5. Match remote fabricIds to find GPU-to-GPU connections
     */

    /* Map fabricId → device index */
    #define MAX_FABRIC_PORTS 128
    struct port_info {
        int      device_idx;
        zes_fabric_port_id_t local_id;
        zes_fabric_port_id_t remote_id;
        int64_t  max_bw;  /* bytes/sec from maxTxSpeed */
        int      is_xelink;
        int      is_healthy;
    };

    struct port_info ports[MAX_FABRIC_PORTS];
    int port_count = 0;

    for (int i = 0; i < n; i++) {
        zes_device_handle_t sys_dev = (zes_device_handle_t)gpu_handles[i];
        uint32_t fp_count = 0;
        if (fn_enum_fabric(sys_dev, &fp_count, NULL) != ZE_RESULT_SUCCESS || fp_count == 0)
            continue;

        zes_fabric_port_handle_t *fp_handles =
            (zes_fabric_port_handle_t *)calloc(fp_count, sizeof(zes_fabric_port_handle_t));
        if (!fp_handles) continue;
        fn_enum_fabric(sys_dev, &fp_count, fp_handles);

        for (uint32_t p = 0; p < fp_count && port_count < MAX_FABRIC_PORTS; p++) {
            zes_fabric_port_properties_t fprops;
            memset(&fprops, 0, sizeof(fprops));
            fprops.stype = ZES_STRUCTURE_TYPE_FABRIC_PORT_PROPERTIES;
            if (fn_fabric_props(fp_handles[p], &fprops) != ZE_RESULT_SUCCESS)
                continue;

            zes_fabric_port_state_t fstate;
            memset(&fstate, 0, sizeof(fstate));
            fstate.stype = ZES_STRUCTURE_TYPE_FABRIC_PORT_STATE;
            if (fn_fabric_state(fp_handles[p], &fstate) != ZE_RESULT_SUCCESS)
                continue;

            ports[port_count].device_idx = i;
            ports[port_count].local_id = fprops.portId;
            ports[port_count].remote_id = fstate.remotePortId;
            ports[port_count].max_bw = fprops.maxTxSpeed.bitRate * (int64_t)fprops.maxTxSpeed.width / 8;
            ports[port_count].is_xelink = (strstr(fprops.model, "XeLink") != NULL ||
                                           strstr(fprops.model, "xelink") != NULL ||
                                           strstr(fprops.model, "XELINK") != NULL);
            ports[port_count].is_healthy = (fstate.status == ZES_FABRIC_PORT_STATUS_HEALTHY ||
                                            fstate.status == ZES_FABRIC_PORT_STATUS_DEGRADED);
            port_count++;
        }
        free(fp_handles);
    }

    /* Match ports to build GPU-to-GPU links */
    /* Track which (i,j) pairs we've already added to avoid duplicates */
    int link_count = 0;
    int pair_bw[MAX_INTEL_GPUS][MAX_INTEL_GPUS];
    memset(pair_bw, 0, sizeof(pair_bw));

    for (int a = 0; a < port_count; a++) {
        if (!ports[a].is_healthy || !ports[a].is_xelink) continue;

        /* Find matching remote port */
        for (int b = 0; b < port_count; b++) {
            if (a == b) continue;
            if (ports[a].remote_id.fabricId == ports[b].local_id.fabricId &&
                ports[a].remote_id.attachId == ports[b].local_id.attachId &&
                ports[a].remote_id.portNumber == ports[b].local_id.portNumber) {

                int dev_a = ports[a].device_idx;
                int dev_b = ports[b].device_idx;
                if (dev_a == dev_b) break;

                /* Accumulate bandwidth for this pair */
                int lo = (dev_a < dev_b) ? dev_a : dev_b;
                int hi = (dev_a < dev_b) ? dev_b : dev_a;
                int64_t bw = ports[a].max_bw;
                if (bw > 0) {
                    pair_bw[lo][hi] += (int)(bw / (1024 * 1024 * 1024));  /* bytes/s → GB/s approx */
                }
                break;
            }
        }
    }

    /* Emit links */
    for (int i = 0; i < n && link_count < max_links; i++) {
        for (int j = i + 1; j < n && link_count < max_links; j++) {
            if (pair_bw[i][j] > 0) {
                out[link_count].gpu_a = i;
                out[link_count].gpu_b = j;
                out[link_count].link_type = GPU_LINK_XELINK;
                out[link_count].bandwidth_gbps = (float)pair_bw[i][j];
                link_count++;
            }
            /* TODO: PCIe fallback for non-XeLink pairs if needed */
        }
    }

    return link_count;
}

int ze_refresh(struct gpu_device *out, int max_devices) {
    return ze_enumerate(out, max_devices);
}

void ze_shutdown(void) {
    if (ze_handle) {
        /* Level Zero has no explicit shutdown — just close the library */
        dlclose(ze_handle);
        ze_handle = NULL;
        gpu_count = 0;
    }
}

#else /* !__linux__ */

int  ze_detect(void)                                              { return 0; }
int  ze_enumerate(struct gpu_device *out, int max_devices)        { (void)out; (void)max_devices; return 0; }
int  ze_topology(struct gpu_link *out, int max_links, int n)      { (void)out; (void)max_links; (void)n; return 0; }
int  ze_refresh(struct gpu_device *out, int max_devices)          { (void)out; (void)max_devices; return 0; }
void ze_shutdown(void)                                            {}

#endif /* __linux__ */
