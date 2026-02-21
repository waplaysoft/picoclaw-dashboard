package logs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Service struct {
	unit string // systemd unit name (например, "picoclaw")
}

func NewService(unit string) *Service {
	return &Service{
		unit: unit,
	}
}

// GetLogs - получает логи из journalctl
func (s *Service) GetLogs(ctx context.Context, filter LogFilter) ([]LogEntry, error) {
	// Базовые параметры
	args := []string{
		"-u", s.unit,
		"-o", "cat",         // Простой вывод
		"--no-pager",        // Не использовать пейджер
	}

	// Фильтр по времени
	if filter.Since != "" {
		sinceTime := parseRelativeTime(filter.Since)
		args = append(args, "--since", sinceTime)
	}

	// Количество строк (если указано)
	if filter.Lines > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", filter.Lines))
	}

	// Выполняем journalctl
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("journalctl error: %w: %s", err, stderr.String())
	}

	// Парсим логи
	entries := s.parseLogs(stdout.String(), filter)

	return entries, nil
}

// parseRelativeTime конвертирует относительное время в формат journalctl
// Например: 5m -> 5 minutes ago, 1h -> 1 hour ago, 24h -> 24 hours ago
func parseRelativeTime(since string) string {
	re := regexp.MustCompile(`^(\d+)([mhd])$`)
	matches := re.FindStringSubmatch(since)

	if matches == nil {
		// Если не совпало с форматом, возвращаем как есть
		return since
	}

	value := matches[1]
	unit := matches[2]

	switch unit {
	case "m":
		return value + " minutes ago"
	case "h":
		return value + " hours ago"
	case "d":
		return value + " days ago"
	default:
		return since
	}
}

// parseLogs парсит вывод journalctl и фильтрует по уровню/поиску
func (s *Service) parseLogs(output string, filter LogFilter) []LogEntry {
	lines := strings.Split(output, "\n")
	var entries []LogEntry

	// Регулярка для парсинга строки лога
	// Формат: YYYY/MM/DD HH:MM:SS [timestamp] [LEVEL] ...
	logPattern := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[.*?\] \[([A-Z]+)\] (.*)`)

	var currentEntry *LogEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := logPattern.FindStringSubmatch(line)
		if matches == nil {
			// Если есть текущая запись, добавляем строку к ней
			if currentEntry != nil {
				currentEntry.Message += "\n" + line
			}
			// Иначе пропускаем строку (это мусор в начале вывода)
			continue
		}

		// Если есть незавершенная запись, сохраняем её
		if currentEntry != nil {
			// Фильтр по уровню
			if filter.Level == "" || currentEntry.Level == filter.Level {
				// Фильтр по поиску
				if filter.Search == "" || strings.Contains(strings.ToLower(currentEntry.Message), strings.ToLower(filter.Search)) ||
					strings.Contains(strings.ToLower(currentEntry.Level), strings.ToLower(filter.Search)) {
					entries = append(entries, *currentEntry)
				}
			}
		}

		// Парсим timestamp
		timestamp, err := time.Parse("2006/01/02 15:04:05", matches[1])
		if err != nil {
			timestamp = time.Now()
		}

		level := matches[2]
		message := matches[3]

		// Начинаем новую запись
		currentEntry = &LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
		}
	}

	// Сохраняем последнюю запись
	if currentEntry != nil {
		// Фильтр по уровню
		if filter.Level == "" || currentEntry.Level == filter.Level {
			// Фильтр по поиску
			if filter.Search == "" || strings.Contains(strings.ToLower(currentEntry.Message), strings.ToLower(filter.Search)) ||
				strings.Contains(strings.ToLower(currentEntry.Level), strings.ToLower(filter.Search)) {
				entries = append(entries, *currentEntry)
			}
		}
	}

	// Сортируем по времени (старые первые)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Timestamp.After(entries[j].Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	return entries
}

// FollowLogs - открывает поток логов (tail -f)
func (s *Service) FollowLogs(ctx context.Context, callback func(LogEntry)) error {
	args := []string{
		"-u", s.unit,
		"-o", "cat",
		"--no-pager",
		"-f", // follow
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start error: %w", err)
	}

	// Читаем построчно и объединяем многострочные сообщения
	buf := make([]byte, 4096)
	var currentEntry *LogEntry

	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return ctx.Err()
		default:
			n, err := stdout.Read(buf)
			if err != nil {
				// Если есть незавершенная запись, отправляем её
				if currentEntry != nil {
					callback(*currentEntry)
				}
				return err
			}
			if n > 0 {
				chunk := string(buf[:n])
				// Разбиваем на строки по переносу
				lines := strings.Split(chunk, "\n")

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					entry := s.parseLogLine(line)

					// Если строка не совпала с форматом (continuation line)
					if entry == nil {
						// Добавляем к текущей записи
						if currentEntry != nil {
							currentEntry.Message += "\n" + line
						}
						continue
					}

					// Если это продолжение предыдущей записи (тот же timestamp)
					if currentEntry != nil &&
						entry.Timestamp.Equal(currentEntry.Timestamp) &&
						entry.Level == currentEntry.Level {
						// Добавляем к текущей записи
						currentEntry.Message += "\n" + entry.Message
					} else {
						// Отправляем предыдущую запись
						if currentEntry != nil {
							callback(*currentEntry)
						}
						// Начинаем новую запись
						currentEntry = entry
					}
				}
			}
		}
	}
}

// parseLogLine парсит одну строку лога
// Возвращает nil если строка не совпадает с форматом лога (это продолжение предыдущей записи)
func (s *Service) parseLogLine(line string) *LogEntry {
	logPattern := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[.*?\] \[([A-Z]+)\] (.*)`)
	matches := logPattern.FindStringSubmatch(line)

	if matches == nil {
		// Не совпало с форматом лога - это continuation line
		return nil
	}

	timestamp, err := time.Parse("2006/01/02 15:04:05", matches[1])
	if err != nil {
		timestamp = time.Now()
	}

	return &LogEntry{
		Timestamp: timestamp,
		Level:     matches[2],
		Message:   matches[3],
	}
}

// GetLogUnits возвращает список доступных systemd юнитов
func (s *Service) GetLogUnits(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--no-pager", "--all")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("systemctl error: %w", err)
	}

	lines := strings.Split(stdout.String(), "\n")
	var units []string

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 1 && strings.HasSuffix(fields[0], ".service") {
			units = append(units, strings.TrimSuffix(fields[0], ".service"))
		}
	}

	return units, nil
}
