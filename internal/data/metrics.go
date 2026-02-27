package data

import (
	"math"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type SystemMetrics struct {
	CPU       *CPUMetrics       `json:"cpu"`
	Memory    *MemoryMetrics    `json:"memory"`
	Disk      *DiskMetrics      `json:"disk"`
	Uptime    uint64            `json:"uptime"`
	OS        string            `json:"os"`
	Hostname  string            `json:"hostname"`
	Processes *ProcessesMetrics `json:"processes"`
	Ts        int64             `json:"ts"`
}

type CPUMetrics struct {
	Load  float64 `json:"load"`
	Cores int     `json:"cores"`
}

type MemoryMetrics struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	Pct       float64 `json:"pct"`
}

type DiskMetrics struct {
	Total uint64  `json:"total"`
	Used  uint64  `json:"used"`
	Free  uint64  `json:"free"`
	Pct   float64 `json:"pct"`
	Mount string  `json:"mount"`
}

type ProcessesMetrics struct {
	Total    int `json:"total"`
	Running  int `json:"running"`
	Sleeping int `json:"sleeping"`
}

func GetSystemMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{Ts: time.Now().UnixMilli()}

	// CPU
	percentages, err := cpu.Percent(0, false)
	if err == nil && len(percentages) > 0 {
		metrics.CPU = &CPUMetrics{
			Load:  math.Round(percentages[0]*10) / 10,
			Cores: runtime.NumCPU(),
		}
	}

	// Memory
	v, err := mem.VirtualMemory()
	if err == nil {
		metrics.Memory = &MemoryMetrics{
			Total:     v.Total,
			Used:      v.Used,
			Free:      v.Free,
			Available: v.Available,
			Pct:       math.Round(float64(v.Used)/float64(v.Total)*1000) / 10,
		}
	}

	// Disk
	d, err := disk.Usage("/")
	if err == nil {
		metrics.Disk = &DiskMetrics{
			Total: d.Total,
			Used:  d.Used,
			Free:  d.Total - d.Used,
			Pct:   math.Round(d.UsedPercent*10) / 10,
			Mount: "/",
		}
	}

	// Uptime & Host
	uptime, _ := host.Uptime()
	metrics.Uptime = uptime

	info, err := host.Info()
	if err == nil {
		metrics.Hostname = info.Hostname
		metrics.OS = info.Platform + " " + info.PlatformVersion
	}

	// Processes
	procs, err := process.Processes()
	if err == nil {
		total := len(procs)
		running := 0
		sleeping := 0
		for _, p := range procs {
			status, err := p.Status()
			if err != nil {
				continue
			}
			for _, s := range status {
				switch s {
				case "R", process.Running:
					running++
				case "S", process.Sleep:
					sleeping++
				}
			}
		}
		metrics.Processes = &ProcessesMetrics{Total: total, Running: running, Sleeping: sleeping}
	}

	hostname, _ := os.Hostname()
	if metrics.Hostname == "" {
		metrics.Hostname = hostname
	}

	return metrics, nil
}
