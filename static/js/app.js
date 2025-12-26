// NetControl Containers - Main Application JS

// Utility functions
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    
    if (days > 0) return `${days}d ${hours}h ${minutes}m`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
}

function formatDate(timestamp) {
    return new Date(timestamp * 1000).toLocaleString();
}

// API client
const api = {
    async get(url) {
        const response = await fetch(url);
        if (response.status === 401) {
            window.location.href = '/login';
            return null;
        }
        return response.json();
    },
    
    async post(url, data) {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (response.status === 401) {
            window.location.href = '/login';
            return null;
        }
        return response.json();
    },
    
    async delete(url) {
        const response = await fetch(url, { method: 'DELETE' });
        if (response.status === 401) {
            window.location.href = '/login';
            return null;
        }
        return response.json();
    }
};

// Logout function
async function logout() {
    await api.post('/api/logout');
    window.location.href = '/login';
}

// Toast notifications
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.innerHTML = `
        <span>${message}</span>
        <button onclick="this.parentElement.remove()">&times;</button>
    `;
    
    let container = document.getElementById('toastContainer');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toastContainer';
        container.style.cssText = 'position:fixed;top:20px;right:20px;z-index:9999;display:flex;flex-direction:column;gap:10px;';
        document.body.appendChild(container);
    }
    
    container.appendChild(toast);
    
    setTimeout(() => toast.remove(), 5000);
}

// Modal functions
function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.add('show');
    }
}

function hideModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.remove('show');
    }
}

// Confirm dialog
function confirmAction(message) {
    return new Promise((resolve) => {
        const result = window.confirm(message);
        resolve(result);
    });
}

// WebSocket manager
class WebSocketManager {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.handlers = {};
    }
    
    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${window.location.host}${this.url}`);
        
        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            if (this.handlers.open) this.handlers.open();
        };
        
        this.ws.onmessage = (event) => {
            if (this.handlers.message) this.handlers.message(event.data);
        };
        
        this.ws.onclose = () => {
            if (this.handlers.close) this.handlers.close();
            this.reconnect();
        };
        
        this.ws.onerror = (error) => {
            if (this.handlers.error) this.handlers.error(error);
        };
    }
    
    reconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            setTimeout(() => this.connect(), this.reconnectDelay * this.reconnectAttempts);
        }
    }
    
    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(typeof data === 'string' ? data : JSON.stringify(data));
        }
    }
    
    sendBinary(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(data);
        }
    }
    
    on(event, handler) {
        this.handlers[event] = handler;
        return this;
    }
    
    close() {
        if (this.ws) {
            this.ws.close();
        }
    }
}

// Export for use in other scripts
window.NetControl = {
    api,
    formatBytes,
    formatUptime,
    formatDate,
    showToast,
    showModal,
    hideModal,
    confirmAction,
    WebSocketManager,
    logout
};
