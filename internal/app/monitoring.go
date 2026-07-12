package app

import (
	"net/http"
	"sync"
	"time"

	"game-server-scripts/internal/system"
)

type MetricPoint struct {
	Time          string  `json:"time"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	DiskPercent   float64 `json:"diskPercent"`
}

type Monitor struct {
	mu     sync.Mutex
	points []MetricPoint
}

func NewMonitor() *Monitor {
	m := &Monitor{}
	m.sample()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.sample()
		}
	}()
	return m
}

func (m *Monitor) History() []MetricPoint {
	m.sample()
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MetricPoint, len(m.points))
	copy(out, m.points)
	return out
}

func (m *Monitor) sample() {
	info := system.Snapshot()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.points = append(m.points, MetricPoint{Time: time.Now().Format(time.RFC3339), CPUPercent: info.CPUPercent, MemoryPercent: info.Memory.Percent, DiskPercent: info.Disk.Percent})
	if len(m.points) > 240 {
		m.points = m.points[len(m.points)-240:]
	}
}

type NetworkCheck struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	OK        bool   `json:"ok"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

func CheckNetworkTargets() []NetworkCheck {
	targets := []NetworkCheck{
		{ID: "steam", Name: "Steam Store", URL: "https://store.steampowered.com"},
		{ID: "steamcmd", Name: "SteamCMD CDN", URL: "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz"},
		{ID: "paper", Name: "PaperMC", URL: "https://fill.papermc.io/v3/projects/paper"},
		{ID: "mojang", Name: "Mojang Meta", URL: "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"},
		{ID: "modrinth", Name: "Modrinth", URL: "https://api.modrinth.com/v2/tag/game_version"},
		{ID: "github", Name: "GitHub", URL: "https://github.com"},
		{ID: "dockerhub", Name: "Docker Hub", URL: "https://registry-1.docker.io/v2/"},
	}
	out := make([]NetworkCheck, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out[i] = checkTarget(targets[i])
		}(i)
	}
	wg.Wait()
	return out
}

func checkTarget(target NetworkCheck) NetworkCheck {
	start := time.Now()
	client := http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, target.URL, nil)
	if err != nil {
		target.Error = err.Error()
		return target
	}
	req.Header.Set("User-Agent", "game-server-scripts/1.0")
	resp, err := client.Do(req)
	target.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		target.Error = err.Error()
		return target
	}
	defer resp.Body.Close()
	target.Status = resp.StatusCode
	target.OK = resp.StatusCode >= 200 && resp.StatusCode < 500
	return target
}
