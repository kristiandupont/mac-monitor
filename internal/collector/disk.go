package collector

import (
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

type DiskStat struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskIOStat struct {
	Name       string `json:"name"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
}

var realFstypes = map[string]bool{
	"apfs": true, "hfs": true, "hfs+": true,
	"exfat": true, "ntfs": true, "msdos": true,
	"ext4": true, "ext3": true, "ext2": true, "xfs": true, "btrfs": true,
}

func collectDisk() ([]DiskStat, []DiskIOStat, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, nil, err
	}

	var stats []DiskStat
	for _, p := range partitions {
		if !strings.HasPrefix(p.Device, "/dev/") {
			continue
		}
		if !realFstypes[p.Fstype] {
			continue
		}
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage.Total < 5<<30 { // skip < 5 GB
			continue
		}
		stats = append(stats, DiskStat{
			MountPoint:  p.Mountpoint,
			Device:      p.Device,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	counters, err := disk.IOCounters()
	var ioStats []DiskIOStat
	if err == nil {
		for name, c := range counters {
			if strings.HasPrefix(name, "disk") {
				ioStats = append(ioStats, DiskIOStat{
					Name:       name,
					ReadBytes:  c.ReadBytes,
					WriteBytes: c.WriteBytes,
				})
			}
		}
	}

	return stats, ioStats, nil
}
