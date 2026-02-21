package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/waplay/picoclaw-dashboard/websocket"
)

type HealthResponse struct {
	CPU     CPUInfo     `json:"cpu"`
	Memory  MemoryInfo  `json:"memory"`
	Disk    DiskInfo    `json:"disk"`
	Uptime  UptimeInfo  `json:"uptime"`
	Runtime RuntimeInfo `json:"runtime"`
}

type CPUInfo struct {
	Usage     float64   `json:"usage_percent"`
	Cores     int       `json:"cores"`
	Timestamp time.Time `json:"timestamp"`
}

type MemoryInfo struct {
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	Available   uint64  `json:"available_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Timestamp   time.Time `json:"timestamp"`
}

type DiskInfo struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	Free        uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Timestamp   time.Time `json:"timestamp"`
}

type UptimeInfo struct {
	Uptime    uint64    `json:"uptime_seconds"`
	BootTime  time.Time `json:"boot_time"`
	Timestamp time.Time `json:"timestamp"`
}

type RuntimeInfo struct {
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func GetHealth() (HealthResponse, error) {
	// CPU usage
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return HealthResponse{}, err
	}
	cpuUsage := 0.0
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	// Memory
	memStat, err := mem.VirtualMemory()
	if err != nil {
		return HealthResponse{}, err
	}

	// Disk (root partition)
	diskStat, err := disk.Usage("/")
	if err != nil {
		return HealthResponse{}, err
	}

	// Uptime
	hostStat, err := host.Info()
	if err != nil {
		return HealthResponse{}, err
	}

	return HealthResponse{
		CPU: CPUInfo{
			Usage:     cpuUsage,
			Cores:     runtime.NumCPU(),
			Timestamp: time.Now(),
		},
		Memory: MemoryInfo{
			Total:       memStat.Total,
			Used:        memStat.Used,
			Available:   memStat.Available,
			UsedPercent: memStat.UsedPercent,
			Timestamp:   time.Now(),
		},
		Disk: DiskInfo{
			Path:        "/",
			Total:       diskStat.Total,
			Used:        diskStat.Used,
			Free:        diskStat.Free,
			UsedPercent: diskStat.UsedPercent,
			Timestamp:   time.Now(),
		},
		Uptime: UptimeInfo{
			Uptime:    hostStat.Uptime,
			BootTime:  time.Unix(int64(hostStat.BootTime), 0),
			Timestamp: time.Now(),
		},
		Runtime: RuntimeInfo{
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		},
	}, nil
}

func SetupRoutes(hub *websocket.Hub) {
	// Health endpoint
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		health, err := GetHealth()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Broadcast to WebSocket clients
		hub.Broadcast(health)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.HandleWebSocket(hub, w, r)
	})

	// File API endpoints
	baseDir := "." // Default to working directory
	http.HandleFunc("/api/files", ListFiles(baseDir))
	http.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ReadFile(baseDir)(w, r)
		case http.MethodPut:
			WriteFile(baseDir)(w, r)
		case http.MethodDelete:
			DeleteFile(baseDir)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/directory", CreateDirectory(baseDir))
}
