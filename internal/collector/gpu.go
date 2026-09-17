package collector

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

typedef struct {
	char     model[128];
	uint64_t cores;
	double   device, tiler, renderer;
	uint64_t in_use, alloc;
} mm_gpu_stat;

static double mm_num(CFDictionaryRef d, const char *key) {
	CFStringRef k = CFStringCreateWithCString(NULL, key, kCFStringEncodingUTF8);
	CFTypeRef v = CFDictionaryGetValue(d, k);
	CFRelease(k);
	double out = 0;
	if (v && CFGetTypeID(v) == CFNumberGetTypeID()) {
		CFNumberGetValue((CFNumberRef)v, kCFNumberDoubleType, &out);
	}
	return out;
}

static CFTypeRef mm_prop(io_registry_entry_t e, const char *key) {
	CFStringRef k = CFStringCreateWithCString(NULL, key, kCFStringEncodingUTF8);
	CFTypeRef v = IORegistryEntryCreateCFProperty(e, k, kCFAllocatorDefault, 0);
	CFRelease(k);
	return v;
}

// Reads IOAccelerator performance statistics straight from the IORegistry,
// which is what `ioreg -rc IOAccelerator` prints, without spawning a process.
static int mm_gpu_stats(mm_gpu_stat *out, int max) {
	io_iterator_t iter;
	if (IOServiceGetMatchingServices(kIOMainPortDefault, IOServiceMatching("IOAccelerator"), &iter) != KERN_SUCCESS) {
		return -1;
	}
	int n = 0;
	io_registry_entry_t e;
	while ((e = IOIteratorNext(iter)) && n < max) {
		CFTypeRef perf = mm_prop(e, "PerformanceStatistics");
		if (perf && CFGetTypeID(perf) == CFDictionaryGetTypeID()) {
			mm_gpu_stat *s = &out[n++];
			memset(s, 0, sizeof(*s));
			CFDictionaryRef d = (CFDictionaryRef)perf;
			s->device   = mm_num(d, "Device Utilization %");
			s->tiler    = mm_num(d, "Tiler Utilization %");
			s->renderer = mm_num(d, "Renderer Utilization %");
			s->in_use   = (uint64_t)mm_num(d, "In use system memory");
			s->alloc    = (uint64_t)mm_num(d, "Alloc system memory");

			CFTypeRef model = mm_prop(e, "model");
			if (model && CFGetTypeID(model) == CFStringGetTypeID()) {
				CFStringGetCString((CFStringRef)model, s->model, sizeof(s->model), kCFStringEncodingUTF8);
			}
			if (model) CFRelease(model);

			CFTypeRef cores = mm_prop(e, "gpu-core-count");
			if (cores && CFGetTypeID(cores) == CFNumberGetTypeID()) {
				CFNumberGetValue((CFNumberRef)cores, kCFNumberSInt64Type, &s->cores);
			}
			if (cores) CFRelease(cores);
		}
		if (perf) CFRelease(perf);
		IOObjectRelease(e);
	}
	IOObjectRelease(iter);
	return n;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

type GPUStat struct {
	Name                string  `json:"name"`
	CoreCount           uint64  `json:"core_count"`
	DeviceUtilization   float64 `json:"device_utilization"`
	TilerUtilization    float64 `json:"tiler_utilization"`
	RendererUtilization float64 `json:"renderer_utilization"`
	MemInUse            uint64  `json:"mem_in_use"`
	MemAllocated        uint64  `json:"mem_allocated"`
}

const maxGPUs = 8

func collectGPU() ([]GPUStat, error) {
	var buf [maxGPUs]C.mm_gpu_stat
	n := int(C.mm_gpu_stats(&buf[0], maxGPUs))
	if n < 0 {
		return nil, errors.New("IOAccelerator lookup failed")
	}
	var stats []GPUStat
	for _, s := range buf[:n] {
		stats = append(stats, GPUStat{
			Name:                C.GoString((*C.char)(unsafe.Pointer(&s.model[0]))),
			CoreCount:           uint64(s.cores),
			DeviceUtilization:   float64(s.device),
			TilerUtilization:    float64(s.tiler),
			RendererUtilization: float64(s.renderer),
			MemInUse:            uint64(s.in_use),
			MemAllocated:        uint64(s.alloc),
		})
	}
	return stats, nil
}
