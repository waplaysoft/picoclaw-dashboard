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
        this.initTabs();
        // Also fetch initial data via REST as fallback
        this.fetchHealth();
    }

    initTabs() {
        const tabs = document.querySelectorAll('.nav-tab');
        tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const tabName = tab.dataset.tab;

                // Update tab buttons
                tabs.forEach(t => t.classList.remove('active'));
                tab.classList.add('active');

                // Update tab content
                document.querySelectorAll('.tab-content').forEach(content => {
                    content.classList.remove('active');
                });
                document.getElementById(`tab-${tabName}`).classList.add('active');

                // If files tab, load files
                if (tabName === 'files') {
                    window.fileExplorer.loadFiles();
                }
            });
        });
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

// File Explorer
class FileExplorer {
    constructor() {
        this.currentPath = '';
        this.filesListEl = document.getElementById('files-list');
        this.breadcrumbEl = document.getElementById('breadcrumb');
        this.editorModal = document.getElementById('file-editor-modal');
        this.newFolderModal = document.getElementById('new-folder-modal');
        this.newFileModal = document.getElementById('new-file-modal');
        this.currentEditingPath = null;
    }

    init() {
        // Button events
        document.getElementById('btn-new-file').addEventListener('click', () => this.showNewFileDialog());
        document.getElementById('btn-new-folder').addEventListener('click', () => this.showNewFolderDialog());

        // Modal events
        document.getElementById('editor-close').addEventListener('click', () => this.hideEditor());
        document.getElementById('editor-cancel').addEventListener('click', () => this.hideEditor());
        document.getElementById('editor-save').addEventListener('click', () => this.saveFile());

        document.getElementById('new-folder-close').addEventListener('click', () => this.hideNewFolderDialog());
        document.getElementById('new-folder-cancel').addEventListener('click', () => this.hideNewFolderDialog());
        document.getElementById('new-folder-create').addEventListener('click', () => this.createFolder());

        document.getElementById('new-file-close').addEventListener('click', () => this.hideNewFileDialog());
        document.getElementById('new-file-cancel').addEventListener('click', () => this.hideNewFileDialog());
        document.getElementById('new-file-create').addEventListener('click', () => this.createFile());

        // Close modal on backdrop click
        this.editorModal.addEventListener('click', (e) => {
            if (e.target === this.editorModal) this.hideEditor();
        });
        this.newFolderModal.addEventListener('click', (e) => {
            if (e.target === this.newFolderModal) this.hideNewFolderDialog();
        });
        this.newFileModal.addEventListener('click', (e) => {
            if (e.target === this.newFileModal) this.hideNewFileDialog();
        });

        // Enter key in modals
        document.getElementById('new-folder-name').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') this.createFolder();
        });
        document.getElementById('new-file-name').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') this.createFile();
        });
    }

    async loadFiles(path = '') {
        this.currentPath = path;
        this.updateBreadcrumb();

        this.filesListEl.innerHTML = '<div class="loading">Loading files...</div>';

        try {
            const url = path ? `/api/files?path=${encodeURIComponent(path)}` : '/api/files';
            const response = await fetch(url);

            if (!response.ok) {
                throw new Error('Failed to load files');
            }

            const files = await response.json();
            this.renderFiles(files);
        } catch (e) {
            console.error('Error loading files:', e);
            this.filesListEl.innerHTML = '<div class="empty">Error loading files</div>';
        }
    }

    updateBreadcrumb() {
        if (!this.currentPath) {
            this.breadcrumbEl.innerHTML = '<span class="breadcrumb-item" data-path="">🏠 Root</span>';
            return;
        }

        const parts = this.currentPath.split('/').filter(p => p);
        let html = '<span class="breadcrumb-item" data-path="">🏠 Root</span>';

        let path = '';
        parts.forEach((part, index) => {
            path += part;
            html += `<span class="breadcrumb-separator">/</span>`;
            html += `<span class="breadcrumb-item" data-path="${path}">${part}</span>`;
            if (index < parts.length - 1) path += '/';
        });

        this.breadcrumbEl.innerHTML = html;

        // Add click handlers
        this.breadcrumbEl.querySelectorAll('.breadcrumb-item').forEach(item => {
            item.addEventListener('click', () => this.loadFiles(item.dataset.path));
        });
    }

    renderFiles(files) {
        if (files.length === 0) {
            this.filesListEl.innerHTML = '<div class="empty">This folder is empty</div>';
            return;
        }

        // Sort: directories first, then files
        files.sort((a, b) => {
            if (a.type === b.type) return a.name.localeCompare(b.name);
            return a.type === 'directory' ? -1 : 1;
        });

        let html = '';
        files.forEach(file => {
            const icon = file.type === 'directory' ? '📁' : this.getFileIcon(file.name);
            const size = file.type === 'file' ? this.formatBytes(file.size) : '';
            const modified = new Date(file.modified).toLocaleDateString();

            html += `
                <div class="file-item" data-path="${file.path}" data-type="${file.type}">
                    <div class="file-icon">${icon}</div>
                    <div class="file-info">
                        <span class="file-name">${file.name}</span>
                    </div>
                    <div class="file-meta">
                        <span>${size}</span>
                        <span style="margin-left: 10px;">${modified}</span>
                    </div>
                    <div class="file-actions">
                        <button class="file-action-btn edit" title="Edit">✏️</button>
                        <button class="file-action-btn delete" title="Delete">🗑️</button>
                    </div>
                </div>
            `;
        });

        this.filesListEl.innerHTML = html;

        // Add click handlers
        this.filesListEl.querySelectorAll('.file-item').forEach(item => {
            const path = item.dataset.path;
            const type = item.dataset.type;

            // Click on file item (not action buttons)
            item.addEventListener('click', (e) => {
                if (!e.target.closest('.file-action-btn')) {
                    if (type === 'directory') {
                        this.loadFiles(path);
                    } else {
                        this.editFile(path);
                    }
                }
            });

            // Edit button
            item.querySelector('.edit').addEventListener('click', (e) => {
                e.stopPropagation();
                if (type === 'file') {
                    this.editFile(path);
                }
            });

            // Delete button
            item.querySelector('.delete').addEventListener('click', (e) => {
                e.stopPropagation();
                this.deleteFile(path, type);
            });
        });
    }

    getFileIcon(filename) {
        const ext = filename.split('.').pop().toLowerCase();

        const icons = {
            'go': '🐹',
            'js': '📜',
            'ts': '📘',
            'html': '🌐',
            'css': '🎨',
            'json': '📋',
            'md': '📝',
            'txt': '📄',
            'py': '🐍',
            'rs': '🦀',
            'c': '⚙️',
            'cpp': '⚙️',
            'h': '⚙️',
            'sh': '💻',
            'yaml': '📋',
            'yml': '📋',
            'xml': '📋',
            'jpg': '🖼️',
            'jpeg': '🖼️',
            'png': '🖼️',
            'gif': '🖼️',
            'svg': '🖼️',
            'mp4': '🎬',
            'mp3': '🎵',
            'pdf': '📕',
            'zip': '📦',
            'tar': '📦',
            'gz': '📦',
        };

        return icons[ext] || '📄';
    }

    async editFile(path) {
        this.filesListEl.innerHTML = '<div class="loading">Loading file...</div>';

        try {
            const url = `/api/file?path=${encodeURIComponent(path)}`;
            const response = await fetch(url);

            if (!response.ok) {
                throw new Error('Failed to load file');
            }

            const content = await response.text();
            this.showEditor(path, content);

            // Reload files list
            this.loadFiles(this.currentPath);
        } catch (e) {
            console.error('Error loading file:', e);
            this.filesListEl.innerHTML = '<div class="empty">Error loading file</div>';
            alert('Failed to load file');
        }
    }

    showEditor(path, content) {
        this.currentEditingPath = path;
        document.getElementById('editor-title').textContent = `Edit: ${path}`;
        document.getElementById('editor-content').value = content;
        this.editorModal.classList.add('active');
    }

    hideEditor() {
        this.editorModal.classList.remove('active');
        this.currentEditingPath = null;
    }

    async saveFile() {
        const content = document.getElementById('editor-content').value;

        try {
            const url = `/api/file?path=${encodeURIComponent(this.currentEditingPath)}`;
            const response = await fetch(url, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ content }),
            });

            if (!response.ok) {
                throw new Error('Failed to save file');
            }

            this.hideEditor();
            this.loadFiles(this.currentPath);
        } catch (e) {
            console.error('Error saving file:', e);
            alert('Failed to save file');
        }
    }

    showNewFolderDialog() {
        document.getElementById('new-folder-name').value = '';
        this.newFolderModal.classList.add('active');
        setTimeout(() => document.getElementById('new-folder-name').focus(), 100);
    }

    hideNewFolderDialog() {
        this.newFolderModal.classList.remove('active');
    }

    async createFolder() {
        const name = document.getElementById('new-folder-name').value.trim();

        if (!name) {
            alert('Please enter a folder name');
            return;
        }

        const path = this.currentPath ? `${this.currentPath}/${name}` : name;

        try {
            const url = `/api/directory?path=${encodeURIComponent(path)}`;
            const response = await fetch(url, {
                method: 'POST',
            });

            if (!response.ok) {
                throw new Error('Failed to create folder');
            }

            this.hideNewFolderDialog();
            this.loadFiles(this.currentPath);
        } catch (e) {
            console.error('Error creating folder:', e);
            alert('Failed to create folder');
        }
    }

    showNewFileDialog() {
        document.getElementById('new-file-name').value = '';
        this.newFileModal.classList.add('active');
        setTimeout(() => document.getElementById('new-file-name').focus(), 100);
    }

    hideNewFileDialog() {
        this.newFileModal.classList.remove('active');
    }

    async createFile() {
        const name = document.getElementById('new-file-name').value.trim();

        if (!name) {
            alert('Please enter a file name');
            return;
        }

        const path = this.currentPath ? `${this.currentPath}/${name}` : name;

        try {
            const url = `/api/file?path=${encodeURIComponent(path)}`;
            const response = await fetch(url, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ content: '' }),
            });

            if (!response.ok) {
                throw new Error('Failed to create file');
            }

            this.hideNewFileDialog();
            this.loadFiles(this.currentPath);
        } catch (e) {
            console.error('Error creating file:', e);
            alert('Failed to create file');
        }
    }

    async deleteFile(path, type) {
        if (!confirm(`Are you sure you want to delete "${path}"?`)) {
            return;
        }

        try {
            const url = `/api/file?path=${encodeURIComponent(path)}`;
            const response = await fetch(url, {
                method: 'DELETE',
            });

            if (!response.ok) {
                throw new Error('Failed to delete file');
            }

            this.loadFiles(this.currentPath);
        } catch (e) {
            console.error('Error deleting file:', e);
            alert('Failed to delete file');
        }
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }
}

// Initialize dashboard and file explorer when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new Dashboard();
    window.fileExplorer = new FileExplorer();
    window.fileExplorer.init();
    window.dashboard.init();
});

// Poll every 30 seconds as fallback
setInterval(() => {
    if (!window.dashboard.connected) {
        window.dashboard.fetchHealth();
    }
}, 30000);
