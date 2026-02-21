package api

import (
	"log"

	"github.com/waplay/picoclaw-dashboard/pkg/logs"
)

var (
	logService *logs.Service
	logHandler *logs.Handler
)

// InitLogsService инициализирует сервис логов
func InitLogsService(unit string) {
	logService = logs.NewService(unit)
	logHandler = logs.NewHandler(logService)
	log.Printf("📝 Logs service initialized for unit: %s", unit)
}

// SetupLogRoutes регистрирует роуты для API логов
func SetupLogRoutes() {
	if logHandler == nil {
		log.Println("⚠️  Log handler not initialized, call InitLogsService first")
		return
	}
	logHandler.RegisterRoutes(http.DefaultServeMux)
	log.Println("📝 Log routes registered")
}
