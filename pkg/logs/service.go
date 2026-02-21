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
		args = append(args, "--since", filter.Since)
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

// parseLogs парсит вывод journalctl и фильтрует по уровню/поиску
func (s *Service) parseLogs(output string, filter LogFilter) []LogEntry {
	lines := strings.Split(output, "\n")
	var entries []LogEntry

	// Регулярка для парсинга строки лога
	// Формат: YYYY/MM/DD HH:MM:SS [timestamp] [LEVEL] ...
	logPattern := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[.*?\] \[([A-Z]+)\] (.*)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := logPattern.FindStringSubmatch(line)
		if matches == nil {
			// Если не совпало с паттерном, добавляем как INFO
			entries = append(entries, LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   line,
			})
			continue
		}

		// Парсим timestamp
		timestamp, err := time.Parse("2006/01/02 15:04:05", matches[1])
		if err != nil {
			timestamp = time.Now()
		}

		level := matches[2]
		message := matches[3]

		// Фильтр по уровню
		if filter.Level != "" && level != filter.Level {
			continue
		}

		// Фильтр по поиску
		if filter.Search != "" && !strings.Contains(strings.ToLower(message), strings.ToLower(filter.Search)) &&
			!strings.Contains(strings.ToLower(level), strings.ToLower(filter.Search)) {
			continue
		}

		entries = append(entries, LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
		})
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

	// Читаем построчно
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return ctx.Err()
		default:
			n, err := stdout.Read(buf)
			if err != nil {
				return err
			}
			if n > 0 {
				line := strings.TrimSpace(string(buf[:n]))
				if line != "" {
					entry := s.parseLogLine(line)
					if entry != nil {
						callback(*entry)
					}
				}
			}
		}
	}
}

// parseLogLine парсит одну строку лога
func (s *Service) parseLogLine(line string) *LogEntry {
	logPattern := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[.*?\] \[([A-Z]+)\] (.*)`)
	matches := logPattern.FindStringSubmatch(line)

	if matches == nil {
		return &LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   line,
		}
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
