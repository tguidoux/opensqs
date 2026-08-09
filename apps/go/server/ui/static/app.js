// Theme toggle
(function() {
    const saved = localStorage.getItem('opensqs-theme');
    if (saved) {
        document.documentElement.setAttribute('data-theme', saved);
    }
})();

document.getElementById('theme-toggle')?.addEventListener('click', function() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('opensqs-theme', next);
});

// Auto-refresh
let refreshInterval = 5000;
let refreshTimer = null;
const refreshStates = [
    {label: '2s', ms: 2000},
    {label: '5s', ms: 5000},
    {label: '10s', ms: 10000},
    {label: 'Off', ms: 0},
];
let refreshStateIdx = 1;

// Escape HTML to prevent XSS when inserting user-controlled data into innerHTML
function escapeHtml(str) {
    if (str == null) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function updateRefreshUI() {
    const state = refreshStates[refreshStateIdx];
    const label = document.querySelector('.refresh-label');
    const dot = document.querySelector('.refresh-dot');
    if (label) label.textContent = state.label;
    if (dot) dot.classList.toggle('active', state.ms > 0);
}

function startRefresh() {
    if (refreshTimer) clearInterval(refreshTimer);
    const state = refreshStates[refreshStateIdx];
    if (state.ms === 0) return;
    refreshTimer = setInterval(doRefresh, state.ms);
}

function doRefresh() {
    const queueTable = document.getElementById('queue-table');
    const messageTable = document.getElementById('message-table');
    const metricsDashboard = document.getElementById('metrics-dashboard');

    if (queueTable) {
        fetch('/api/queues')
            .then(r => r.json())
            .then(queues => {
                const tbody = queueTable.querySelector('tbody');
                if (!tbody) return;
                tbody.innerHTML = '';
                queues.forEach(q => {
                    const tr = document.createElement('tr');
                    tr.setAttribute('data-queue', q.Name);
                    const safeName = escapeHtml(q.Name);
                    const safeURL = escapeHtml(q.URL);
                    tr.innerHTML = `
                        <td data-label="Name"><a href="/queues/${encodeURIComponent(q.Name)}">${safeName}</a></td>
                        <td data-label="Type">${q.IsFifo ? '<span class="badge badge-fifo">FIFO</span>' : '<span class="badge badge-standard">Standard</span>'}</td>
                        <td data-label="Available" class="num">${q.Available}</td>
                        <td data-label="In-Flight" class="num">${q.InFlight}</td>
                        <td data-label="Delayed" class="num">${q.Delayed}</td>
                        <td data-label="URL" class="url-cell">${safeURL}</td>
                        <td data-label="Actions" class="action-cell">
                            <form method="POST" action="/queues/${encodeURIComponent(q.Name)}/purge" style="display:inline" onsubmit="return confirm('Purge all messages from ${safeName}?')">
                                <button type="submit" class="btn btn-sm btn-warning">Purge</button>
                            </form>
                            <form method="POST" action="/queues/${encodeURIComponent(q.Name)}/delete" style="display:inline" onsubmit="return confirm('Delete queue ${safeName}? This cannot be undone.')">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    `;
                    tbody.appendChild(tr);
                });
            })
            .catch(() => {});
    }

    if (messageTable) {
        const queueName = window.location.pathname.split('/').pop();
        fetch(`/api/queues/${encodeURIComponent(queueName)}/messages`)
            .then(r => r.json())
            .then(msgs => {
                const tbody = messageTable.querySelector('tbody');
                if (!tbody) return;
                tbody.innerHTML = '';
                msgs.forEach(m => {
                    const tr = document.createElement('tr');
                    const safeMsgID = escapeHtml(m.MessageID);
                    const safeBody = escapeHtml(m.Body);
                    tr.innerHTML = `
                        <td data-label="Message ID" class="mono">${safeMsgID}</td>
                        <td data-label="Body" class="body-cell">${safeBody}</td>
                        <td data-label="Receive Count" class="num">${m.ReceiveCount}</td>
                        <td data-label="Sent">${escapeHtml(m.SentTimestamp)}</td>
                        <td data-label="Actions">
                            <form method="POST" action="/queues/${encodeURIComponent(queueName)}/messages/${encodeURIComponent(m.ReceiptHandle)}/delete" style="display:inline" onsubmit="return confirm('Delete this message?')">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    `;
                    tbody.appendChild(tr);
                });
            })
            .catch(() => {});
    }

    if (metricsDashboard) {
        fetch('/api/metrics')
            .then(r => r.json())
            .then(data => {
                refreshMetricsTable('api-requests-table', data.APIRequests, (item) => `
                    <td>${escapeHtml(item.Action)}</td>
                    <td><span class="badge badge-standard">${escapeHtml(item.Protocol)}</span></td>
                    <td class="num">${item.Count}</td>
                `, 3);

                refreshMetricsTable('queue-sizes-table', data.QueueSizes, (item) => `
                    <td>${escapeHtml(item.Queue)}</td>
                    <td><span class="badge ${item.Type === 'available' ? 'badge-standard' : 'badge-fifo'}">${escapeHtml(item.Type)}</span></td>
                    <td class="num">${item.Size}</td>
                `, 3);

                refreshMetricsTable('messages-sent-table', data.MessagesSent, (item) => `
                    <td>${escapeHtml(item.Queue)}</td>
                    <td class="num">${item.Count}</td>
                `, 2);

                refreshMetricsTable('messages-received-table', data.MessagesReceived, (item) => `
                    <td>${escapeHtml(item.Queue)}</td>
                    <td class="num">${item.Count}</td>
                `, 2);

                refreshMetricsTable('messages-deleted-table', data.MessagesDeleted, (item) => `
                    <td>${escapeHtml(item.Queue)}</td>
                    <td class="num">${item.Count}</td>
                `, 2);

                refreshMetricsTable('move-tasks-table', data.MoveTaskMessages, (item) => `
                    <td class="url-cell">${escapeHtml(item.SourceARN)}</td>
                    <td class="url-cell">${escapeHtml(item.DestinationARN)}</td>
                    <td class="num">${item.Count}</td>
                `, 3);

                const activeEl = document.querySelector('.metric-value');
                if (activeEl) activeEl.textContent = data.MoveTaskActive;

                const rawEl = document.getElementById('raw-metrics');
                if (rawEl && data.Raw) rawEl.textContent = data.Raw;
            })
            .catch(() => {});
    }
}

function refreshMetricsTable(tableId, items, renderRow, colspan) {
    const table = document.getElementById(tableId);
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;
    if (!items || items.length === 0) {
        tbody.innerHTML = `<tr><td colspan="${colspan}" class="empty-state">No data yet.</td></tr>`;
        return;
    }
    tbody.innerHTML = '';
    items.forEach(item => {
        const tr = document.createElement('tr');
        tr.innerHTML = renderRow(item);
        tbody.appendChild(tr);
    });
}

document.getElementById('refresh-toggle')?.addEventListener('click', function() {
    refreshStateIdx = (refreshStateIdx + 1) % refreshStates.length;
    updateRefreshUI();
    startRefresh();
});

updateRefreshUI();
startRefresh();
