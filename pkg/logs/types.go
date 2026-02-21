package logs

import "time"

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type LogRequest struct {
	Lines  int    `json:"lines"`   // Количество строк
	Level  string `json:"level"`   // Фильтр по уровню (INFO, WARN, ERROR)
	Since  string `json:"since"`   // Фильтр по времени (1h, 1d, etc.)
	Search string `json:"search"`  // Поиск по тексту
}

type LogResponse struct {
	Entries []LogEntry `json:"entries"`
	Total   int        `json:"total"`
	Unit    string     `json:"unit"`
}

type LogFilter struct {
	Lines  int
	Level  string
	Since  string
	Search string
}
