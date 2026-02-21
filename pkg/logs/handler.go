package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes регистрирует роуты для API логов
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/logs", h.getLogs)
	mux.HandleFunc("GET /api/logs/units", h.getUnits)
	mux.HandleFunc("GET /api/logs/stream", h.streamLogs)
}

func (h *Handler) getLogs(w http.ResponseWriter, r *http.Request) {
	// Параметры запроса
	query := r.URL.Query()

	// Количество строк (по умолчанию 100)
	lines := 100
	if l := query.Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			lines = n
		}
	}

	// Фильтры
	filter := LogFilter{
		Lines:  lines,
		Level:  query.Get("level"),
		Since:  query.Get("since"),
		Search: query.Get("search"),
	}

	// Таймаут для запроса
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Получаем логи
	entries, err := h.service.GetLogs(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	response := LogResponse{
		Entries: entries,
		Total:   len(entries),
		Unit:    h.service.unit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) getUnits(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	units, err := h.service.GetLogUnits(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"units": units,
	})
}

// streamLogs - SSE endpoint для стриминга логов в реальном времени
func (h *Handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что это SSE запрос
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(w, "Accept: text/event-stream required", http.StatusNotAcceptable)
		return
	}

	// Фильтры
	query := r.URL.Query()
	filter := LogFilter{
		Level:  query.Get("level"),
		Search: query.Get("search"),
	}

	// Заголовки SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Флашер для немедленной отправки
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Контекст для отмены
	ctx := r.Context()

	// Функция для отправки события
	sendEvent := func(entry LogEntry) {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Запускаем чтение логов
	go func() {
		err := h.service.FollowLogs(ctx, func(entry LogEntry) {
			// Применяем фильтры
			if filter.Level != "" && entry.Level != filter.Level {
				return
			}
			if filter.Search != "" && !contains(entry.Message, filter.Search) {
				return
			}
			sendEvent(entry)
		})

		if err != nil {
			sendEvent(LogEntry{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   "Log stream error: " + err.Error(),
			})
		}
	}()

	// Ждем пока клиент не закроет соединение
	<-ctx.Done()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
