package collector

import (
	"context"
	"os/exec"
	"time"

	"howett.net/plist"
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

func collectGPU() ([]GPUStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ioreg", "-rc", "IOAccelerator", "-a").Output()
	if err != nil {
		return nil, err
	}

	var entries []map[string]interface{}
	if _, err := plist.Unmarshal(out, &entries); err != nil {
		return nil, err
	}

	var stats []GPUStat
	for _, e := range entries {
		perfRaw, ok := e["PerformanceStatistics"]
		if !ok {
			continue
		}
		perf, ok := perfRaw.(map[string]interface{})
		if !ok {
			continue
		}

		stat := GPUStat{
			Name:                plistString(e, "model"),
			CoreCount:           plistUint64(e, "gpu-core-count"),
			DeviceUtilization:   plistFloat(perf, "Device Utilization %"),
			TilerUtilization:    plistFloat(perf, "Tiler Utilization %"),
			RendererUtilization: plistFloat(perf, "Renderer Utilization %"),
			MemInUse:            plistUint64(perf, "In use system memory"),
			MemAllocated:        plistUint64(perf, "Alloc system memory"),
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func plistString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func plistFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case uint64:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

func plistUint64(m map[string]interface{}, key string) uint64 {
	switch v := m[key].(type) {
	case uint64:
		return v
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		return uint64(v)
	}
	return 0
}
