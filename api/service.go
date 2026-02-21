package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ServiceResponse — статус сервиса
type ServiceResponse struct {
	Active      bool      `json:"active"`
	Running     bool      `json:"running"`
	Loaded      bool      `json:"loaded"`
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"`
	ActiveSince time.Time `json:"active_since"`
	Timestamp   time.Time `json:"timestamp"`
}

// ServiceUnit — имя systemd сервиса (picoclaw)
const ServiceUnit = "picoclaw"

var (
	serviceStatusCache ServiceResponse
	serviceCacheTime   time.Time
)

// GetServiceStatus — получить статус сервиса picoclaw
func GetServiceStatus() (ServiceResponse, error) {
	// Проверяем состояние кэша (обновляем не чаще чем раз в 5 секунд)
	if time.Since(serviceCacheTime) < 5*time.Second {
		return serviceStatusCache, nil
	}

	// Используем systemctl show для получения подробной информации
	cmd := exec.Command("systemctl", "show", "--property=ActiveState,SubState,LoadState,UnitFileState,ActiveEnterTimestamp", ServiceUnit)
	output, err := cmd.Output()
	if err != nil {
		return ServiceResponse{}, fmt.Errorf("failed to get service status: %w", err)
	}

	response := ServiceResponse{
		Timestamp: time.Now(),
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "ActiveState":
			response.Active = value == "active"
		case "SubState":
			response.Running = value == "running"
		case "LoadState":
			response.Loaded = value == "loaded"
		case "UnitFileState":
			response.Enabled = value == "enabled"
		case "ActiveEnterTimestamp":
			if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", value); err == nil {
				response.ActiveSince = t
			}
		}
	}

	// Определяем текстовый статус
	if response.Active && response.Running {
		response.Status = "Running"
	} else if response.Active {
		response.Status = "Active"
	} else {
		response.Status = "Stopped"
	}

	// Обновляем кэш
	serviceStatusCache = response
	serviceCacheTime = time.Now()

	return response, nil
}

// ServiceAction — действие над сервисом
type ServiceAction struct {
	Action string `json:"action"` // start, stop, restart
}

// ControlService — управление сервисом
func ControlService(action string) error {
	var cmd *exec.Cmd

	switch action {
	case "start":
		cmd = exec.Command("sudo", "-n", "systemctl", "start", ServiceUnit)
	case "stop":
		cmd = exec.Command("sudo", "-n", "systemctl", "stop", ServiceUnit)
	case "restart":
		cmd = exec.Command("sudo", "-n", "systemctl", "restart", ServiceUnit)
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to %s service: %w\nOutput: %s", action, err, string(output))
	}

	log.Printf("🔧 Service action '%s' executed successfully for %s", action, ServiceUnit)

	// Сбрасываем кэш статуса после действия
	serviceCacheTime = time.Time{}

	return nil
}

// SetupServiceRoutes — регистрирует роуты для управления сервисом
func SetupServiceRoutes() {
	// GET /api/service — получить статус сервиса
	http.HandleFunc("/api/service", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		status, err := GetServiceStatus()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(status)
	})

	// POST /api/service/action — действие над сервисом
	http.HandleFunc("/api/service/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ServiceAction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := ControlService(req.Action); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Возвращаем обновлённый статус
		status, err := GetServiceStatus()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(status)
	})

	log.Println("🔧 Service routes registered")
}
