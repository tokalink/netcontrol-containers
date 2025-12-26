package services

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemInfo struct {
	Hostname   string     `json:"hostname"`
	Platform   string     `json:"platform"`
	OS         string     `json:"os"`
	Arch       string     `json:"arch"`
	Uptime     uint64     `json:"uptime"`
	BootTime   uint64     `json:"boot_time"`
	CPUInfo    CPUInfo    `json:"cpu_info"`
	MemoryInfo MemoryInfo `json:"memory_info"`
	DiskInfo   []DiskInfo `json:"disk_info"`
}

type CPUInfo struct {
	ModelName    string    `json:"model_name"`
	Cores        int       `json:"cores"`
	Threads      int       `json:"threads"`
	Mhz          float64   `json:"mhz"`
	UsagePercent []float64 `json:"usage_percent"`
	TotalUsage   float64   `json:"total_usage"`
}

type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	Available   uint64  `json:"available"`
}

type DiskInfo struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

func GetSystemInfo() (*SystemInfo, error) {
	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	cpuInfo, err := GetCPUInfo()
	if err != nil {
		return nil, err
	}

	memInfo, err := GetMemoryInfo()
	if err != nil {
		return nil, err
	}

	diskInfo, err := GetDiskInfo()
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		Hostname:   hostInfo.Hostname,
		Platform:   hostInfo.Platform,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Uptime:     hostInfo.Uptime,
		BootTime:   hostInfo.BootTime,
		CPUInfo:    *cpuInfo,
		MemoryInfo: *memInfo,
		DiskInfo:   diskInfo,
	}, nil
}

func GetCPUInfo() (*CPUInfo, error) {
	cpuInfos, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	cpuPercent, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, err
	}

	totalPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	modelName := "Unknown"
	mhz := 0.0
	if len(cpuInfos) > 0 {
		modelName = cpuInfos[0].ModelName
		mhz = cpuInfos[0].Mhz
	}

	return &CPUInfo{
		ModelName:    modelName,
		Cores:        runtime.NumCPU(),
		Threads:      len(cpuPercent),
		Mhz:          mhz,
		UsagePercent: cpuPercent,
		TotalUsage:   totalPercent[0],
	}, nil
}

func GetMemoryInfo() (*MemoryInfo, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &MemoryInfo{
		Total:       memInfo.Total,
		Used:        memInfo.Used,
		Free:        memInfo.Free,
		UsedPercent: memInfo.UsedPercent,
		Available:   memInfo.Available,
	}, nil
}

func GetDiskInfo() ([]DiskInfo, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var disks []DiskInfo
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			continue
		}

		disks = append(disks, DiskInfo{
			Device:      partition.Device,
			Mountpoint:  partition.Mountpoint,
			Fstype:      partition.Fstype,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	return disks, nil
}

func GetQuickStats() (map[string]interface{}, error) {
	cpuPercent, err := cpu.Percent(time.Millisecond*500, false)
	if err != nil {
		return nil, err
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var totalDisk, usedDisk uint64
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			continue
		}
		totalDisk += usage.Total
		usedDisk += usage.Used
	}

	diskPercent := 0.0
	if totalDisk > 0 {
		diskPercent = float64(usedDisk) / float64(totalDisk) * 100
	}

	return map[string]interface{}{
		"cpu_percent":    cpuPercent[0],
		"memory_percent": memInfo.UsedPercent,
		"memory_used":    memInfo.Used,
		"memory_total":   memInfo.Total,
		"disk_percent":   diskPercent,
		"disk_used":      usedDisk,
		"disk_total":     totalDisk,
	}, nil
}
