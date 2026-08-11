// OpenSQS UI JavaScript — wrapped in IIFE to avoid polluting global scope
(function() {
'use strict';

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

// Escape for use in JavaScript string literals (e.g., confirm() dialogs)
function escapeJsString(str) {
    if (str == null) return '';
    return String(str)
        .replace(/\\/g, '\\\\')
        .replace(/'/g, "\\'")
        .replace(/"/g, '\\"')
        .replace(/\n/g, '\\n')
        .replace(/\r/g, '\\r');
}

// Build a confirm-on-submit handler for a form element
function attachConfirmOnSubmit(form, message) {
    form.addEventListener('submit', function(e) {
        if (!confirm(message)) {
            e.preventDefault();
        }
    });
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
            .then(r => { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
            .then(queues => {
                const tbody = queueTable.querySelector('tbody');
                if (!tbody) return;
                tbody.innerHTML = '';
                queues.forEach(q => {
                    const tr = document.createElement('tr');
                    tr.setAttribute('data-queue', q.Name);
                    const safeName = escapeHtml(q.Name);
                    const safeURL = escapeHtml(q.URL);
                    const encodedName = encodeURIComponent(q.Name);
                    tr.innerHTML = `
                        <td data-label="Name"><a href="/queues/${encodedName}">${safeName}</a></td>
                        <td data-label="Type">${q.IsFifo ? '<span class="badge badge-fifo">FIFO</span>' : '<span class="badge badge-standard">Standard</span>'}</td>
                        <td data-label="Available" class="num">${q.Available}</td>
                        <td data-label="In-Flight" class="num">${q.InFlight}</td>
                        <td data-label="Delayed" class="num">${q.Delayed}</td>
                        <td data-label="URL" class="url-cell">${safeURL}</td>
                        <td data-label="Actions" class="action-cell">
                            <form method="POST" action="/queues/${encodedName}/purge" style="display:inline">
                                <button type="submit" class="btn btn-sm btn-warning">Purge</button>
                            </form>
                            <form method="POST" action="/queues/${encodedName}/delete" style="display:inline">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    `;
                    // Attach confirm dialogs safely (no inline JS)
                    const purgeForm = tr.querySelector('form[action$="/purge"]');
                    if (purgeForm) attachConfirmOnSubmit(purgeForm, 'Purge all messages from ' + q.Name + '?');
                    const deleteForm = tr.querySelector('form[action$="/delete"]');
                    if (deleteForm) attachConfirmOnSubmit(deleteForm, 'Delete queue ' + q.Name + '? This cannot be undone.');
                    tbody.appendChild(tr);
                });
            })
            .catch(() => {});
    }

    if (messageTable) {
        const queueName = window.location.pathname.split('/').pop();
        fetch(`/api/queues/${encodeURIComponent(queueName)}/messages`)
            .then(r => { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
            .then(msgs => {
                const tbody = messageTable.querySelector('tbody');
                if (!tbody) return;
                tbody.innerHTML = '';
                msgs.forEach(m => {
                    const tr = document.createElement('tr');
                    const safeMsgID = escapeHtml(m.MessageID);
                    const safeBody = escapeHtml(m.Body);
                    const encodedQueue = encodeURIComponent(queueName);
                    const encodedReceipt = encodeURIComponent(m.ReceiptHandle);
                    tr.innerHTML = `
                        <td data-label="Message ID" class="mono">${safeMsgID}</td>
                        <td data-label="Body" class="body-cell">${safeBody}</td>
                        <td data-label="Receive Count" class="num">${m.ReceiveCount}</td>
                        <td data-label="Sent">${escapeHtml(m.SentTimestamp)}</td>
                        <td data-label="Actions">
                            <form method="POST" action="/queues/${encodedQueue}/messages/${encodedReceipt}/delete" style="display:inline">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    `;
                    const deleteForm = tr.querySelector('form');
                    if (deleteForm) attachConfirmOnSubmit(deleteForm, 'Delete this message?');
                    tbody.appendChild(tr);
                });
            })
            .catch(() => {});
    }

    if (metricsDashboard) {
        fetch('/api/metrics')
            .then(r => { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
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

    // Auto-refresh credential table (secrets are never included in the API response)
    const credTable = document.getElementById('credential-table');
    if (credTable && !document.querySelector('.credential-created-card')) {
        fetch('/api/credentials')
            .then(r => { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
            .then(creds => {
                const tbody = credTable.querySelector('tbody');
                if (!tbody) return;
                if (!creds || creds.length === 0) {
                    tbody.innerHTML = '';
                    return;
                }
                tbody.innerHTML = '';
                creds.forEach(c => {
                    const tr = document.createElement('tr');
                    const encodedID = encodeURIComponent(c.ID);
                    tr.innerHTML = `
                        <td data-label="Label">${escapeHtml(c.Label)}</td>
                        <td data-label="Access Key ID" class="mono">${escapeHtml(c.AccessKeyID)}</td>
                        <td data-label="Region">${escapeHtml(c.Region)}</td>
                        <td data-label="Account ID">${escapeHtml(c.AccountID)}</td>
                        <td data-label="Created">${escapeHtml(c.CreatedAt)}</td>
                        <td data-label="Actions" class="action-cell">
                            <form method="POST" action="/credentials/${encodedID}/delete" style="display:inline">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    `;
                    const deleteForm = tr.querySelector('form');
                    if (deleteForm) attachConfirmOnSubmit(deleteForm, 'Delete credential ' + c.Label + '? This cannot be undone.');
                    tbody.appendChild(tr);
                });
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
// Only auto-start refresh on pages with dynamic content (queue/message/credential tables).
if (document.getElementById('queue-table') || document.getElementById('message-table') || document.getElementById('credential-table')) {
    startRefresh();
}

// --- Global helpers for inline onclick handlers ---

// copyToClipboard copies text from an element and shows feedback on the button.
window.copyToClipboard = function(elementId, btn) {
    const text = document.getElementById(elementId).textContent;
    navigator.clipboard.writeText(text).then(function() {
        const original = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(function() { btn.textContent = original; }, 2000);
    });
};

// toggleCreateForm shows/hides the create credential form.
window.toggleCreateForm = function() {
    const form = document.getElementById('create-form');
    if (form) form.style.display = form.style.display === 'none' ? 'block' : 'none';
};

})(); // end IIFE
