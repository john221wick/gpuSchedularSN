/*
 * AMDGPU sysfs fallback backend.
 *
 * Scans DRM card device sysfs paths for AMD PCI vendor 0x1002 and reads
 * monitoring data exposed by the Linux amdgpu driver.
 */

#ifdef __linux__

#include "sysfs.h"
#include <ctype.h>
#include <dirent.h>
#include <errno.h>
#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <unistd.h>

#define AMD_PCI_VENDOR_ID "0x1002"
#define MAX_AMDGPU_SYSFS_DEVICES 32

struct amdgpu_sysfs_device {
    int card_index;
    char card_path[PATH_MAX];
    char device_path[PATH_MAX];
};

static struct amdgpu_sysfs_device sysfs_devices[MAX_AMDGPU_SYSFS_DEVICES];
static int sysfs_device_count = 0;

static const char *sysfs_root(void) {
    const char *root = getenv("GPUSCHED_AMDGPU_SYSFS_ROOT");
    if (root && root[0] != '\0') return root;
    return "/sys/class/drm";
}

static int path_join(char *out, size_t out_len, const char *a, const char *b) {
    int n = snprintf(out, out_len, "%s/%s", a, b);
    return n > 0 && (size_t)n < out_len;
}

static void trim(char *s) {
    size_t len = strlen(s);
    while (len > 0 && isspace((unsigned char)s[len - 1])) {
        s[--len] = '\0';
    }
    char *p = s;
    while (*p && isspace((unsigned char)*p)) p++;
    if (p != s) memmove(s, p, strlen(p) + 1);
}

static int read_first_line(const char *path, char *out, size_t out_len) {
    FILE *f = fopen(path, "r");
    if (!f) return 0;
    if (!fgets(out, (int)out_len, f)) {
        fclose(f);
        return 0;
    }
    fclose(f);
    trim(out);
    return 1;
}

static int read_u64_file(const char *dir, const char *name, uint64_t *value) {
    char path[PATH_MAX];
    char buf[128];
    if (!path_join(path, sizeof(path), dir, name)) return 0;
    if (!read_first_line(path, buf, sizeof(buf))) return 0;

    errno = 0;
    char *end = NULL;
    unsigned long long parsed = strtoull(buf, &end, 0);
    if (errno != 0 || end == buf) return 0;

    *value = (uint64_t)parsed;
    return 1;
}

static int read_i64_file(const char *dir, const char *name, int64_t *value) {
    char path[PATH_MAX];
    char buf[128];
    if (!path_join(path, sizeof(path), dir, name)) return 0;
    if (!read_first_line(path, buf, sizeof(buf))) return 0;

    errno = 0;
    char *end = NULL;
    long long parsed = strtoll(buf, &end, 0);
    if (errno != 0 || end == buf) return 0;

    *value = (int64_t)parsed;
    return 1;
}

static int is_card_name(const char *name, int *card_index) {
    if (strncmp(name, "card", 4) != 0) return 0;
    if (!isdigit((unsigned char)name[4])) return 0;

    int value = 0;
    for (const char *p = name + 4; *p; p++) {
        if (!isdigit((unsigned char)*p)) return 0;
        value = value * 10 + (*p - '0');
    }
    *card_index = value;
    return 1;
}

static int has_amd_vendor(const char *device_path) {
    char path[PATH_MAX];
    char vendor[64];
    if (!path_join(path, sizeof(path), device_path, "vendor")) return 0;
    if (!read_first_line(path, vendor, sizeof(vendor))) return 0;
    return strcasecmp(vendor, AMD_PCI_VENDOR_ID) == 0;
}

static int driver_is_amdgpu_or_unknown(const char *device_path) {
    char driver_link[PATH_MAX];
    char resolved[PATH_MAX];
    if (!path_join(driver_link, sizeof(driver_link), device_path, "driver")) return 1;

    ssize_t n = readlink(driver_link, resolved, sizeof(resolved) - 1);
    if (n < 0) {
        return 1;
    }
    resolved[n] = '\0';

    const char *base = strrchr(resolved, '/');
    base = base ? base + 1 : resolved;
    return strcmp(base, "amdgpu") == 0;
}

static int read_device_id(const char *device_path, char *out, size_t out_len) {
    char path[PATH_MAX];
    if (!path_join(path, sizeof(path), device_path, "device")) return 0;
    return read_first_line(path, out, out_len);
}

static int read_product_name(const char *device_path, char *out, size_t out_len) {
    char path[PATH_MAX];

    if (path_join(path, sizeof(path), device_path, "product_name") &&
        read_first_line(path, out, out_len) && out[0] != '\0') {
        return 1;
    }
    if (path_join(path, sizeof(path), device_path, "uevent")) {
        FILE *f = fopen(path, "r");
        if (f) {
            char line[256];
            while (fgets(line, sizeof(line), f)) {
                trim(line);
                if (strncmp(line, "PCI_ID=", 7) == 0) {
                    snprintf(out, out_len, "AMD Radeon GPU (%s)", line + 7);
                    fclose(f);
                    return 1;
                }
            }
            fclose(f);
        }
    }

    char dev_id[64];
    if (read_device_id(device_path, dev_id, sizeof(dev_id))) {
        snprintf(out, out_len, "AMD Radeon GPU (%s)", dev_id);
    } else {
        snprintf(out, out_len, "AMD Radeon GPU");
    }
    return 1;
}

static int read_temperature_c(const char *device_path) {
    char hwmon_path[PATH_MAX];
    if (!path_join(hwmon_path, sizeof(hwmon_path), device_path, "hwmon")) return 0;

    DIR *hwmon_dir = opendir(hwmon_path);
    if (!hwmon_dir) return 0;

    int temp_c = 0;
    struct dirent *hwmon_entry;
    while ((hwmon_entry = readdir(hwmon_dir)) != NULL && temp_c == 0) {
        if (strncmp(hwmon_entry->d_name, "hwmon", 5) != 0) continue;

        char sensor_dir_path[PATH_MAX];
        if (!path_join(sensor_dir_path, sizeof(sensor_dir_path), hwmon_path, hwmon_entry->d_name)) continue;

        DIR *sensor_dir = opendir(sensor_dir_path);
        if (!sensor_dir) continue;

        struct dirent *sensor_entry;
        while ((sensor_entry = readdir(sensor_dir)) != NULL) {
            const char *name = sensor_entry->d_name;
            size_t len = strlen(name);
            if (strncmp(name, "temp", 4) != 0 || len <= 10) continue;
            if (strcmp(name + len - 6, "_input") != 0) continue;

            int64_t millideg = 0;
            if (read_i64_file(sensor_dir_path, name, &millideg) && millideg > 0) {
                temp_c = (int)(millideg / 1000);
                break;
            }
        }
        closedir(sensor_dir);
    }

    closedir(hwmon_dir);
    return temp_c;
}

static void fill_device(struct gpu_device *out, int id, const struct amdgpu_sysfs_device *dev) {
    memset(out, 0, sizeof(*out));
    out->id = id;
    out->vendor = GPU_VENDOR_AMD;
    out->vendor_index = dev->card_index;

    char name[128];
    read_product_name(dev->device_path, name, sizeof(name));
    strncpy(out->name, name, sizeof(out->name) - 1);
    out->name[sizeof(out->name) - 1] = '\0';

    uint64_t total = 0, used = 0;
    if (!read_u64_file(dev->device_path, "mem_info_vram_total", &total) || total == 0) {
        read_u64_file(dev->device_path, "mem_info_gtt_total", &total);
    }
    if (!read_u64_file(dev->device_path, "mem_info_vram_used", &used) || used == 0) {
        read_u64_file(dev->device_path, "mem_info_gtt_used", &used);
    }
    out->vram_total_mb = total / (1024ULL * 1024ULL);
    out->vram_used_mb = used / (1024ULL * 1024ULL);

    uint64_t busy = 0;
    if (read_u64_file(dev->device_path, "gpu_busy_percent", &busy)) {
        if (busy > 100) busy = 100;
        out->utilization_pct = (float)busy;
    }

    out->temperature_c = read_temperature_c(dev->device_path);
}

int amdgpu_sysfs_detect(void) {
    sysfs_device_count = 0;

    const char *root = sysfs_root();
    DIR *dir = opendir(root);
    if (!dir) return 0;

    struct dirent *entry;
    while ((entry = readdir(dir)) != NULL && sysfs_device_count < MAX_AMDGPU_SYSFS_DEVICES) {
        int card_index = 0;
        if (!is_card_name(entry->d_name, &card_index)) continue;

        char card_path[PATH_MAX];
        char device_path[PATH_MAX];
        if (!path_join(card_path, sizeof(card_path), root, entry->d_name)) continue;
        if (!path_join(device_path, sizeof(device_path), card_path, "device")) continue;

        if (!has_amd_vendor(device_path)) continue;
        if (!driver_is_amdgpu_or_unknown(device_path)) continue;

        struct amdgpu_sysfs_device *dev = &sysfs_devices[sysfs_device_count++];
        dev->card_index = card_index;
        strncpy(dev->card_path, card_path, sizeof(dev->card_path) - 1);
        dev->card_path[sizeof(dev->card_path) - 1] = '\0';
        strncpy(dev->device_path, device_path, sizeof(dev->device_path) - 1);
        dev->device_path[sizeof(dev->device_path) - 1] = '\0';
    }

    closedir(dir);
    return sysfs_device_count;
}

int amdgpu_sysfs_enumerate(struct gpu_device *out, int max_devices) {
    if (sysfs_device_count == 0) return 0;

    int n = sysfs_device_count;
    if (n > max_devices) n = max_devices;
    for (int i = 0; i < n; i++) {
        fill_device(&out[i], i, &sysfs_devices[i]);
    }
    return n;
}

int amdgpu_sysfs_topology(struct gpu_link *out, int max_links, int num_devices) {
    if (sysfs_device_count < 2) return 0;

    int n = sysfs_device_count;
    if (n > num_devices) n = num_devices;

    int link_count = 0;
    for (int i = 0; i < n && link_count < max_links; i++) {
        for (int j = i + 1; j < n && link_count < max_links; j++) {
            out[link_count].gpu_a = i;
            out[link_count].gpu_b = j;
            out[link_count].link_type = GPU_LINK_PCIE;
            out[link_count].bandwidth_gbps = 32.0f;
            link_count++;
        }
    }
    return link_count;
}

int amdgpu_sysfs_refresh(struct gpu_device *out, int max_devices) {
    return amdgpu_sysfs_enumerate(out, max_devices);
}

void amdgpu_sysfs_shutdown(void) {
    sysfs_device_count = 0;
}

#else /* !__linux__ */

#include "sysfs.h"

int  amdgpu_sysfs_detect(void)                                              { return 0; }
int  amdgpu_sysfs_enumerate(struct gpu_device *out, int max_devices)        { (void)out; (void)max_devices; return 0; }
int  amdgpu_sysfs_topology(struct gpu_link *out, int max_links, int n)      { (void)out; (void)max_links; (void)n; return 0; }
int  amdgpu_sysfs_refresh(struct gpu_device *out, int max_devices)          { (void)out; (void)max_devices; return 0; }
void amdgpu_sysfs_shutdown(void)                                            {}

#endif /* __linux__ */
