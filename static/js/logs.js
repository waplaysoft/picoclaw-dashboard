// API для работы с логами
class LogsAPI {
    constructor() {
        this.baseUrl = '/api/logs';
    }

    /**
     * Получить логи с фильтрами
     * @param {Object} filters - {lines, level, since, search}
     * @returns {Promise<LogResponse>}
     */
    async getLogs(filters = {}) {
        const params = new URLSearchParams();

        if (filters.lines) params.append('lines', filters.lines);
        if (filters.level) params.append('level', filters.level);
        if (filters.since) params.append('since', filters.since);
        if (filters.search) params.append('search', filters.search);

        const response = await fetch(`${this.baseUrl}?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Failed to fetch logs: ${response.statusText}`);
        }
        return response.json();
    }

    /**
     * Получить список доступных systemd юнитов
     */
    async getUnits() {
        const response = await fetch('/api/logs/units');
        if (!response.ok) {
            throw new Error(`Failed to fetch units: ${response.statusText}`);
        }
        return response.json();
    }

    /**
     * Открыть SSE поток логов
     * @param {Function} onMessage - callback для каждого лога
     * @param {Function} onError - callback для ошибок
     * @param {Object} filters - {level, search}
     * @returns {EventSource}
     */
    streamLogs(onMessage, onError, filters = {}) {
        const params = new URLSearchParams();
        if (filters.level) params.append('level', filters.level);
        if (filters.search) params.append('search', filters.search);

        const eventSource = new EventSource(`${this.baseUrl}/stream?${params.toString()}`);

        eventSource.onmessage = (event) => {
            try {
                const entry = JSON.parse(event.data);
                onMessage(entry);
            } catch (e) {
                console.error('Failed to parse log entry:', e);
            }
        };

        eventSource.onerror = (err) => {
            console.error('SSE error:', err);
            if (onError) onError(err);
        };

        return eventSource;
    }
}

/**
 * Форматирует timestamp в красивую строку
 */
function formatTimestamp(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        fractionalSecondDigits: 3
    });
}

/**
 * Возвращает цвет для уровня лога
 */
function getLogLevelColor(level) {
    const colors = {
        'INFO': '#00a8ff',
        'WARN': '#fbc531',
        'ERROR': '#e84118',
        'DEBUG': '#8c7ae6',
        'FATAL': '#c23616'
    };
    return colors[level] || '#7f8fa6';
}

/**
 * Возвращает CSS класс для уровня лога
 */
function getLogLevelClass(level) {
    return `log-level-${level.toLowerCase()}`;
}

// Глобальный инстанс API
const logsAPI = new LogsAPI();
