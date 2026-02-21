// PicoClaw Dashboard Client

class Dashboard {
    constructor() {
        this.ws = null;
        this.reconnectInterval = 5000;
        this.statusEl = document.getElementById('status');
        this.connected = false;
    }

    init() {
        this.connectWebSocket();
        // Also fetch initial data via REST as fallback
        this.fetchHealth();
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        console.log('Connecting to WebSocket:', wsUrl);

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            this.connected = true;
            this.setStatus('connected', 'Connected via WebSocket');
            console.log('✅ WebSocket connected');

            // Request initial data
            this.fetchHealth();
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.updateDashboard(data);
            } catch (e) {
                console.error('Error parsing WebSocket message:', e);
            }
        };

        this.ws.onclose = () => {
            this.connected = false;
            this.setStatus('disconnected', 'Disconnected - Reconnecting...');
            console.log('❌ WebSocket disconnected');

            setTimeout(() => this.connectWebSocket(), this.reconnectInterval);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    setStatus(status, text) {
        this.statusEl.className = `status ${status}`;
        this.statusEl.querySelector('.status-text').textContent = text;
    }

    async fetchHealth() {
        try {
            const response = await fetch('/api/health');
            if (response.ok) {
                const data = await response.json();
                this.updateDashboard(data);
            }
        } catch (e) {
            console.error('Error fetching health:', e);
        }
    }

    updateDashboard(data) {
        // Update CPU
        this.updateMetric('cpu', data.cpu.usage_percent, `${data.cpu.cores} cores`);

        // Update Memory
        this.updateMetric('mem', data.memory.used_percent, this.formatBytes(data.memory.used_bytes) + ' / ' + this.formatBytes(data.memory.total_bytes));

        // Update Disk
        this.updateMetric('disk', data.disk.used_percent, this.formatBytes(data.disk.used_bytes) + ' / ' + this.formatBytes(data.disk.total_bytes));

        // Update Uptime
        const uptime = this.formatUptime(data.uptime.uptime_seconds);
        document.getElementById('uptime-value').textContent = uptime;
        document.getElementById('uptime-meta').textContent = this.formatDateTime(data.uptime.boot_time);

        // Update Runtime Info
        document.getElementById('os-value').textContent = data.runtime.os;
        document.getElementById('arch-value').textContent = data.runtime.arch;
        document.getElementById('go-value').textContent = data.runtime.go_version;

        // Update timestamp
        const updated = new Date(data.cpu.timestamp || Date.now());
        document.getElementById('updated-value').textContent = this.formatTime(updated);
    }

    updateMetric(type, value, meta) {
        const valueEl = document.getElementById(`${type}-value`);
        const barEl = document.getElementById(`${type}-bar`);
        const metaEl = document.getElementById(`${type}-meta`);

        valueEl.textContent = value.toFixed(1);
        barEl.style.width = `${Math.min(value, 100)}%`;
        metaEl.textContent = meta;

        // Color based on usage
        barEl.classList.remove('warning', 'danger');
        if (value > 90) {
            barEl.classList.add('danger');
        } else if (value > 70) {
            barEl.classList.add('warning');
        }
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    formatUptime(seconds) {
        const days = Math.floor(seconds / 86400);
        const hours = Math.floor((seconds % 86400) / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);

        if (days > 0) {
            return `${days}d ${hours}h`;
        } else if (hours > 0) {
            return `${hours}h ${minutes}m`;
        } else {
            return `${minutes}m`;
        }
    }

    formatDateTime(isoString) {
        const date = new Date(isoString);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }

    formatTime(date) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }
}

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new Dashboard();
    window.dashboard.init();
});

// Poll every 30 seconds as fallback
setInterval(() => {
    if (!window.dashboard.connected) {
        window.dashboard.fetchHealth();
    }
}, 30000);
