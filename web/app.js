function showTab(tabName, element) {
    document.querySelectorAll('.tab-content').forEach(tab => tab.style.display = 'none');
    document.querySelectorAll('.nav-item').forEach(item => item.classList.remove('active'));

    const tab = document.getElementById(`tab-${tabName}`);
    if (tab) tab.style.display = 'block';
    if (element) element.classList.add('active');

    // Update header title
    const titles = { 'dashboard': 'Дашборд', 'shares': 'Общие ресурсы', 'users': 'Пользователи', 'global': 'Настройки сервера' };
    const pageTitle = document.getElementById('page-title');
    if (pageTitle) pageTitle.innerText = titles[tabName] || 'Samba Panel';

    // Show apply button only on config tabs
    const applyBtn = document.getElementById('btn-apply');
    if (applyBtn) {
        const configTabs = ['shares', 'global'];
        applyBtn.style.display = configTabs.includes(tabName) ? 'block' : 'none';
    }

    if (tabName === 'shares') loadShares();
    if (tabName === 'users') loadUsers();
    if (tabName === 'global') loadGlobalConfig();
    if (tabName === 'logs') loadLogs();
    if (tabName === 'audit') loadAuditLogs();
    if (tabName === 'automation') loadAutomationSettings();
}

async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        if (response.status === 401) {
            window.location.href = '/login.html';
            return;
        }
        const data = await response.json();

        document.getElementById('session-count').innerText = Object.keys(data.sessions || {}).length;
        document.getElementById('file-count').innerText = Object.keys(data.open_files || {}).length;
        document.getElementById('samba-version').innerText = data.version || 'Samba Server';

        const sessionTable = document.getElementById('sessions-table-body');
        sessionTable.innerHTML = '';
        for (const id in data.sessions) {
            const s = data.sessions[id];
            sessionTable.innerHTML += `<tr><td><strong>${s.user}</strong></td><td>${s.remote_machine}</td><td><span class="mono">${s.protocol_version}</span></td></tr>`;
        }
        
        updateServiceStatus();
        loadDiskUsage();
    } catch (e) { console.error(e); }
}

async function updateServiceStatus() {
    try {
        const res = await fetch('/api/service/status');
        const status = await res.text();
        const topBadge = document.getElementById('samba-status-badge');
        
        if (status === 'active') {
            topBadge.innerText = 'SMB: ONLINE';
            topBadge.className = 'badge online';
        } else {
            topBadge.innerText = 'SMB: OFFLINE';
            topBadge.className = 'badge offline';
        }
    } catch (e) { console.error(e); }
}

async function controlService(action) {
    if (!confirm(`Вы уверены, что хотите выполнить команду "${action}" для сервиса Samba?`)) return;
    
    try {
        const res = await fetch('/api/service/control', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action })
        });
        if (res.ok) {
            updateServiceStatus();
        } else {
            const error = await res.text();
            alert('Ошибка управления сервисом: ' + error);
        }
    } catch (e) { console.error(e); }
}

async function loadShares() {
    try {
        const response = await fetch('/api/shares');
        if (response.status === 401) return;
        const data = await response.json();
        const table = document.getElementById('shares-table-body');
        table.innerHTML = '';
        
        data.forEach(share => {
            const recycleStatus = share.is_recycle ? 
                '<span class="badge online" style="font-size:0.6rem">Активна</span>' : 
                '<span class="badge" style="color:#64748b; font-size:0.6rem">Выкл</span>';
            
            table.innerHTML += `<tr>
                <td><strong>${share.name}</strong></td>
                <td><span class="mono">${share.path}</span></td>
                <td>${recycleStatus}</td>
                <td>
                    <button class="btn-action" onclick='openShareModal(${JSON.stringify(share)})'>Настроить</button>
                    <button class="btn-action" style="color: #dc2626;" onclick="deleteShare('${share.name}')">Удалить</button>
                </td>
            </tr>`;
        });
    } catch (e) { console.error(e); }
}

function openShareModal(share = null) {
    const modal = document.getElementById('share-modal');
    document.getElementById('modal-title').innerText = share ? 'Настройка ресурса' : 'Новый ресурс';
    
    document.getElementById('share-name').value = share ? share.name : '';
    document.getElementById('share-name').readOnly = !!share;
    document.getElementById('share-path').value = share ? share.path : '';
    document.getElementById('share-comment').value = share ? (share.params.comment || '') : '';
    document.getElementById('share-recycle').checked = share ? share.is_recycle : false;
    document.getElementById('share-audit').checked = share ? share.is_audit : false;
    document.getElementById('share-shadow').checked = share ? share.is_shadow_copy : false;
    document.getElementById('share-readonly').checked = share ? (share.params['read only'] !== 'no') : false;
    document.getElementById('share-guest').checked = share ? (share.params['guest ok'] !== 'no') : true;
    document.getElementById('share-browseable').checked = share ? (share.params['browseable'] !== 'no') : true;

    // Recycle fields
    document.getElementById('share-recycle-repo').value = share ? (share.params['recycle:repository'] || '') : '';
    document.getElementById('share-recycle-exclude').value = share ? (share.params['recycle:exclude'] || '') : '';
    document.getElementById('share-recycle-exclude-dir').value = share ? (share.params['recycle:exclude_dir'] || '') : '';

    // Audit fields
    if (share && share.is_audit) {
        const success = share.params['full_audit:success'] || '';
        document.getElementById('audit-unlink').checked = success.includes('unlink');
        document.getElementById('audit-rename').checked = success.includes('rename');
        document.getElementById('audit-mkdir').checked = success.includes('mkdir');
        document.getElementById('audit-open').checked = success.includes('open');
    } else {
        document.getElementById('audit-unlink').checked = true;
        document.getElementById('audit-rename').checked = true;
        document.getElementById('audit-mkdir').checked = true;
        document.getElementById('audit-open').checked = false;
    }

    // Advanced fields
    document.getElementById('share-create-mask').value = share ? (share.params['create mask'] || '0664') : '0664';
    document.getElementById('share-dir-mask').value = share ? (share.params['directory mask'] || '0775') : '0775';
    document.getElementById('share-inherit-acls').checked = share ? (share.params['inherit acls'] !== 'no') : true;
    document.getElementById('share-guest-only').checked = share ? (share.params['guest only'] === 'yes') : false;

    toggleRecycleInfo();
    toggleAuditInfo();
    modal.style.display = 'block';
}

function toggleRecycleInfo() {
    const isChecked = document.getElementById('share-recycle').checked;
    const isGuest = document.getElementById('share-guest').checked;
    const info = document.getElementById('recycle-info');
    info.style.display = isChecked ? 'block' : 'none';

    if (isChecked) {
        const repo = document.getElementById('share-recycle-repo');
        const exclude = document.getElementById('share-recycle-exclude');
        const excludeDir = document.getElementById('share-recycle-exclude-dir');

        if (!repo.value) repo.value = isGuest ? '.recycle/guest' : '.recycle/%U';
        if (!exclude.value) exclude.value = '*.tmp *.temp ~$* *.bak Thumbs.db';
        if (!excludeDir.value) excludeDir.value = '/tmp /cache .recycle';
    }
}

function toggleAuditInfo() {
    const isChecked = document.getElementById('share-audit').checked;
    document.getElementById('audit-info').style.display = isChecked ? 'block' : 'none';
}

document.getElementById('share-recycle').onchange = toggleRecycleInfo;
document.getElementById('share-guest').onchange = toggleRecycleInfo;
document.getElementById('share-audit').onchange = toggleAuditInfo;

function closeShareModal() {
    document.getElementById('share-modal').style.display = 'none';
}

document.getElementById('share-form').onsubmit = async (e) => {
    e.preventDefault();
    const share = {
        name: document.getElementById('share-name').value,
        path: document.getElementById('share-path').value,
        comment: document.getElementById('share-comment').value,
        is_recycle: document.getElementById('share-recycle').checked,
        is_audit: document.getElementById('share-audit').checked,
        is_shadow_copy: document.getElementById('share-shadow').checked,
        audit_open: document.getElementById('audit-open').checked,
        params: {
            'read only': document.getElementById('share-readonly').checked ? 'yes' : 'no',
            'guest ok': document.getElementById('share-guest').checked ? 'yes' : 'no',
            'browseable': document.getElementById('share-browseable').checked ? 'yes' : 'no',
            'create mask': document.getElementById('share-create-mask').value,
            'directory mask': document.getElementById('share-dir-mask').value,
            'force create mode': document.getElementById('share-create-mask').value,
            'force directory mode': document.getElementById('share-dir-mask').value,
            'inherit acls': document.getElementById('share-inherit-acls').checked ? 'yes' : 'no',
            'guest only': document.getElementById('share-guest-only').checked ? 'yes' : 'no',
            'recycle:repository': document.getElementById('share-recycle-repo').value,
            'recycle:exclude': document.getElementById('share-recycle-exclude').value,
            'recycle:exclude_dir': document.getElementById('share-recycle-exclude-dir').value
        }
    };

    const res = await fetch('/api/shares/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(share)
    });

    if (res.ok) {
        closeShareModal();
        loadShares();
    } else {
        alert('Ошибка при сохранении ресурса');
    }
};

async function deleteShare(name) {
    if (!confirm(`Вы уверены, что хотите удалить ресурс "${name}"?`)) return;
    
    const res = await fetch('/api/shares/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name })
    });

    if (res.ok) loadShares();
}

async function loadGlobalConfig() {
    const res = await fetch('/api/global');
    if (res.status === 401) return;
    const data = await res.json();
    const form = document.getElementById('global-form');
    for (const key in data.params) {
        const input = form.querySelector(`[name="${key}"]`);
        if (input) input.value = data.params[key];
    }
}

document.getElementById('global-form').onsubmit = async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    const params = {};
    formData.forEach((value, key) => params[key] = value);

    const res = await fetch('/api/global/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ params })
    });

    if (res.ok) alert('Глобальные настройки сохранены');
};

async function applyChanges() {
    const btn = document.getElementById('btn-apply');
    if (!btn) return;
    
    const originalText = btn.innerText;
    btn.innerText = 'Применяю...';
    btn.disabled = true;
    btn.style.opacity = '0.7';

    try {
        const res = await fetch('/api/service/apply', { method: 'POST' });
        if (res.ok) {
            alert('Настройки успешно применены!');
        } else {
            const error = await res.text();
            alert('Ошибка при проверке конфига:\n' + error);
        }
    } catch (e) {
        alert('Ошибка связи с сервером');
    } finally {
        btn.innerText = originalText;
        btn.disabled = false;
        btn.style.opacity = '1';
    }
}

async function loadUsers() {
    try {
        const response = await fetch('/api/users');
        if (response.status === 401) return;
        const data = await response.json();
        const table = document.getElementById('users-table-body');
        table.innerHTML = '';
        
        data.forEach(user => {
            table.innerHTML += `<tr>
                <td><strong>${user.username}</strong></td>
                <td><span class="mono">${user.uid}</span></td>
                <td>${user.full_name || '-'}</td>
                <td>
                    <button class="btn-action" onclick="openPasswordModal('${user.username}')">Пароль</button>
                    <button class="btn-action" style="color: #dc2626;" onclick="deleteUser('${user.username}')">Удалить</button>
                </td>
            </tr>`;
        });
    } catch (e) { console.error(e); }
}

function openUserModal() {
    document.getElementById('user-modal').style.display = 'block';
    document.getElementById('user-form').reset();
}

function closeUserModal() {
    document.getElementById('user-modal').style.display = 'none';
}

document.getElementById('user-form').onsubmit = async (e) => {
    e.preventDefault();
    const username = document.getElementById('user-username').value;
    const password = document.getElementById('user-password').value;

    const res = await fetch('/api/users/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
    });

    if (res.ok) {
        closeUserModal();
        loadUsers();
    } else {
        const error = await res.text();
        alert('Ошибка при сохранении пользователя: ' + error);
    }
};

function openPasswordModal(username) {
    document.getElementById('password-modal').style.display = 'block';
    document.getElementById('pass-username').value = username;
    document.getElementById('pass-user-display').innerText = username;
    document.getElementById('new-password').value = '';
}

function closePasswordModal() {
    document.getElementById('password-modal').style.display = 'none';
}

document.getElementById('password-form').onsubmit = async (e) => {
    e.preventDefault();
    const username = document.getElementById('pass-username').value;
    const password = document.getElementById('new-password').value;

    const res = await fetch('/api/users/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
    });

    if (res.ok) {
        alert('Пароль успешно изменен');
        closePasswordModal();
    } else {
        const error = await res.text();
        alert('Ошибка при смене пароля: ' + error);
    }
};

async function deleteUser(username) {
    if (!confirm(`Вы уверены, что хотите удалить пользователя Samba "${username}"?`)) return;

    const res = await fetch('/api/users/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username })
    });

    if (res.ok) {
        loadUsers();
    } else {
        const error = await res.text();
        alert('Ошибка при удалении пользователя: ' + error);
    }
}

async function loadDiskUsage() {
    const container = document.getElementById('disk-usage-container');
    if (!container) return;

    try {
        const res = await fetch('/api/disk/usage');
        const data = await res.json();
        
        container.innerHTML = '';
        data.forEach(disk => {
            const color = disk.percent > 90 ? '#ef4444' : (disk.percent > 75 ? '#f59e0b' : '#10b981');
            const sharesList = disk.shares && disk.shares.length > 0 ? 
                `<div style="font-size: 0.7rem; color: #64748b; margin-top: 0.5rem; border-top: 1px dashed #e2e8f0; padding-top: 0.5rem;">
                    <strong>Ресурсы:</strong> ${disk.shares.join(', ')}
                 </div>` : '';

            container.innerHTML += `
                <div style="background: #f8fafc; padding: 1rem; border-radius: 12px; border: 1px solid #e2e8f0;">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 0.5rem;">
                        <span style="font-weight: 600; font-size: 0.85rem; color: #1e293b; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px;" title="${disk.mount_point}">${disk.mount_point}</span>
                        <span style="font-size: 0.8rem; color: #64748b;">${disk.percent}%</span>
                    </div>
                    <div style="height: 8px; background: #e2e8f0; border-radius: 4px; overflow: hidden; margin-bottom: 0.5rem;">
                        <div style="width: ${disk.percent}%; height: 100%; background: ${color}; border-radius: 4px;"></div>
                    </div>
                    <div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: #64748b;">
                        <span>Использовано: ${disk.used}</span>
                        <span>Всего: ${disk.total}</span>
                    </div>
                    <div style="font-size: 0.7rem; color: #94a3b8; margin-top: 0.4rem;">Свободно: ${disk.free}</div>
                    ${sharesList}
                </div>
            `;
        });
    } catch (e) { console.error(e); }
}

async function loadLogs() {
    const output = document.getElementById('log-output');
    if (!output) return;

    try {
        const res = await fetch('/api/logs');
        const text = await res.text();
        
        // Сравниваем с текущим содержимым, чтобы не скроллить лишний раз
        if (output.innerText !== text) {
            output.innerText = text;
            output.scrollTop = output.scrollHeight;
        }
    } catch (e) { console.error(e); }
}

async function loadAuditLogs() {
    const table = document.getElementById('audit-table-body');
    if (!table) return;

    try {
        const res = await fetch('/api/audit');
        const data = await res.json();
        
        table.innerHTML = '';
        data.forEach(entry => {
            let actionColor = '#1e293b';
            if (entry.action === 'unlink') actionColor = '#ef4444';
            if (entry.action === 'rename') actionColor = '#f59e0b';
            if (entry.action === 'mkdir') actionColor = '#10b981';

            table.innerHTML += `
                <tr>
                    <td style="font-size: 0.75rem; color: #64748b;">${entry.timestamp}</td>
                    <td><strong>${entry.user}</strong></td>
                    <td class="mono" style="font-size: 0.75rem;">${entry.ip}</td>
                    <td><span class="badge" style="background: ${actionColor}; color: white; padding: 2px 6px;">${entry.action.toUpperCase()}</span></td>
                    <td style="font-size: 0.8rem; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="${entry.file}">${entry.file}</td>
                </tr>
            `;
        });
    } catch (e) { console.error(e); }
}

// Автообновление логов каждые 5 секунд
setInterval(() => {
    const logsTab = document.getElementById('tab-logs');
    if (logsTab && logsTab.style.display === 'block') loadLogs();
    
    const auditTab = document.getElementById('tab-audit');
    if (auditTab && auditTab.style.display === 'block') loadAuditLogs();
}, 5000);

async function clearRecycleBins() {
    if (!confirm('Вы уверены, что хотите безвозвратно удалить все файлы из сетевых корзин всех ресурсов?')) return;

    try {
        const res = await fetch('/api/maintenance/clear-recycle', { method: 'POST' });
        const results = await res.json();
        
        const successCount = results.filter(r => !r.error).length;
        const failCount = results.filter(r => r.error).length;
        
        let message = `Очистка завершена.\nУспешно очищено корзин: ${successCount}`;
        if (failCount > 0) message += `\nОшибок: ${failCount}`;
        
        alert(message);
        loadDiskUsage(); // Обновляем инфо о дисках
    } catch (e) {
        alert('Ошибка при очистке: ' + e.message);
    }
}

async function logout() {
    await fetch('/api/logout');
    window.location.href = '/login.html';
}

async function loadAutomationSettings() {
    try {
        const res = await fetch('/api/automation');
        const s = await res.json();
        document.getElementById('auto-recycle-days').value = s.recycle_days;
        document.getElementById('auto-snap-interval').value = s.snapshot_interval;
        document.getElementById('auto-snap-keep').value = s.snapshot_keep;
    } catch (e) { console.error(e); }
}

async function saveAutomationSettings() {
    const s = {
        recycle_days: parseInt(document.getElementById('auto-recycle-days').value),
        snapshot_interval: document.getElementById('auto-snap-interval').value,
        snapshot_keep: parseInt(document.getElementById('auto-snap-keep').value)
    };
    try {
        await fetch('/api/automation/save', {
            method: 'POST',
            body: JSON.stringify(s)
        });
        alert('Настройки автоматизации сохранены');
    } catch (e) { alert('Ошибка: ' + e.message); }
}


setInterval(updateStatus, 3000);

setInterval(updateStatus, 3000);
updateStatus();
