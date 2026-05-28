package collector

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type NetStat struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

type Snapshot struct {
	Timestamp   int64     `json:"ts"`
	CPUPercent  float64   `json:"cpu_percent"`
	CPUPerCore  []float64 `json:"cpu_per_core"`
	MemTotal    uint64    `json:"mem_total"`
	MemUsed     uint64    `json:"mem_used"`
	MemPercent  float64   `json:"mem_percent"`
	SwapTotal   uint64    `json:"swap_total"`
	SwapUsed    uint64    `json:"swap_used"`
	SwapPercent float64   `json:"swap_percent"`
	Load1       float64   `json:"load_1"`
	Load5       float64   `json:"load_5"`
	Load15      float64   `json:"load_15"`
	NetStats    []NetStat `json:"net_stats"`
	GPUStats    []GPUStat `json:"gpu_stats"`
}

func Collect() (*Snapshot, error) {
	snap := &Snapshot{Timestamp: time.Now().Unix()}

	cpuPct, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}
	if len(cpuPct) > 0 {
		snap.CPUPercent = cpuPct[0]
	}

	perCore, err := cpu.Percent(0, true)
	if err != nil {
		return nil, err
	}
	snap.CPUPerCore = perCore

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	snap.MemTotal = vm.Total
	snap.MemUsed = vm.Used
	snap.MemPercent = vm.UsedPercent

	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}
	snap.SwapTotal = swap.Total
	snap.SwapUsed = swap.Used
	snap.SwapPercent = swap.UsedPercent

	avg, err := load.Avg()
	if err != nil {
		return nil, err
	}
	snap.Load1 = avg.Load1
	snap.Load5 = avg.Load5
	snap.Load15 = avg.Load15

	counters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}
	for _, c := range counters {
		snap.NetStats = append(snap.NetStats, NetStat{
			Name:        c.Name,
			BytesSent:   c.BytesSent,
			BytesRecv:   c.BytesRecv,
			PacketsSent: c.PacketsSent,
			PacketsRecv: c.PacketsRecv,
		})
	}

	gpuStats, err := collectGPU()
	if err != nil {
		// GPU collection is best-effort — don't fail the whole snapshot
		gpuStats = nil
	}
	snap.GPUStats = gpuStats

	return snap, nil
}
