package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	CPUPercent float64 `json:"cpuPercent"`
	Memory     Memory  `json:"memory"`
	Disk       Disk    `json:"disk"`
	Uptime     int64   `json:"uptime"`
}

type Memory struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
}

type Disk struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

func Snapshot() Info {
	return Info{CPUPercent: cpuPercent(), Memory: memory(), Disk: disk("/"), Uptime: uptime()}
}

func cpuPercent() float64 {
	a := readCPU()
	time.Sleep(120 * time.Millisecond)
	b := readCPU()
	idle := float64(b.idle - a.idle)
	total := float64(b.total - a.total)
	if total <= 0 {
		return 0
	}
	return (1 - idle/total) * 100
}

type cpuTimes struct{ total, idle uint64 }

func readCPU() cpuTimes {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return cpuTimes{}
	}
	fields := strings.Fields(s.Text())
	var total, idle uint64
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return cpuTimes{total: total, idle: idle}
}

func memory() Memory {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Memory{}
	}
	vals := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		vals[key] = v * 1024
	}
	total := vals["MemTotal"]
	avail := vals["MemAvailable"]
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return Memory{Total: total, Available: avail, Used: used, Percent: pct}
}

func disk(path string) Disk {
	return diskUsage(path)
}

func uptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return int64(v)
}
