function showTab(tabName, element) {
    document.querySelectorAll('.tab-content').forEach(tab => tab.style.display = 'none');
    document.querySelectorAll('.nav-item').forEach(item => item.classList.remove('active'));

    const tab = document.getElementById(`tab-${tabName}`);
    if (tab) tab.style.display = 'block';
    if (element) element.classList.add('active');

    // Update header title
    const titles = { 
        'dashboard': 'Дашборд', 
        'shares': 'Общие ресурсы', 
        'users': 'Пользователи', 
        'global': 'Настройки сервера',
        'logs': 'Логи системы',
        'audit': 'Журнал аудита',
        'automation': 'Автоматизация'
    };
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

        const sessionEl = document.getElementById('session-count');
        const fileEl = document.getElementById('file-count');
        const versionEl = document.getElementById('samba-version');
        
        if (sessionEl) sessionEl.innerText = Object.keys(data.sessions || {}).length;
        if (fileEl) fileEl.innerText = Object.keys(data.open_files || {}).length;
        if (versionEl) versionEl.innerText = data.version || 'Samba Server';

        const sessionTable = document.getElementById('sessions-table-body');
        if (sessionTable) {
            sessionTable.innerHTML = '';
            for (const id in data.sessions) {
                const s = data.sessions[id];
                sessionTable.innerHTML += `<tr><td><strong>${s.user}</strong></td><td>${s.remote_machine}</td><td><span class="mono">${s.protocol_version}</span></td></tr>`;
            }
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
        
        if (topBadge) {
            if (status === 'active') {
                topBadge.innerHTML = '<span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%;"></span> SMB: ONLINE';
                topBadge.className = 'badge online';
            } else {
                topBadge.innerHTML = '<span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%;"></span> SMB: OFFLINE';
                topBadge.className = 'badge offline';
            }
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
        if (!table) return;
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
                    <button class="btn-action btn-outline" onclick='openShareModal(${JSON.stringify(share)})'><i data-lucide="edit-3" style="width:14px"></i></button>
                    <button class="btn-action btn-outline" style="color: #ef4444;" onclick="deleteShare('${share.name}')"><i data-lucide="trash-2" style="width:14px"></i></button>
                </td>
            </tr>`;
        });
        if (window.lucide) lucide.createIcons();
    } catch (e) { console.error(e); }
}

function openShareModal(share = null) {
    const modal = document.getElementById('share-modal');
    if (!modal) return;
    
    const title = document.getElementById('modal-title');
    if (title) title.innerText = share ? 'Настройка ресурса' : 'Новый ресурс';
    
    const setVal = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; };
    const setCheck = (id, val) => { const el = document.getElementById(id); if (el) el.checked = val; };

    setVal('share-name', share ? share.name : '');
    const nameEl = document.getElementById('share-name');
    if (nameEl) nameEl.readOnly = !!share;
    
    setVal('share-path', share ? share.path : '');
    setVal('share-comment', share ? (share.params.comment || '') : '');
    setCheck('share-recycle', share ? share.is_recycle : false);
    setCheck('share-audit', share ? share.is_audit : false);
    setCheck('share-shadow', share ? share.is_shadow_copy : false);
    setCheck('share-readonly', share ? (share.params['read only'] !== 'no') : false);
    setCheck('share-guest', share ? (share.params['guest ok'] !== 'no') : true);
    setCheck('share-browseable', share ? (share.params['browseable'] !== 'no') : true);

    // Recycle fields
    setVal('share-recycle-repo', share ? (share.params['recycle:repository'] || '') : '');
    setVal('share-recycle-exclude', share ? (share.params['recycle:exclude'] || '') : '');
    setVal('share-recycle-exclude-dir', share ? (share.params['recycle:exclude_dir'] || '') : '');

    // Audit fields
    if (share && share.is_audit) {
        const success = share.params['full_audit:success'] || '';
        setCheck('audit-unlink', success.includes('unlink'));
        setCheck('audit-rename', success.includes('rename'));
        setCheck('audit-mkdir', success.includes('mkdir'));
        setCheck('audit-open', success.includes('open'));
    } else {
        setCheck('audit-unlink', true);
        setCheck('audit-rename', true);
        setCheck('audit-mkdir', true);
        setCheck('audit-open', false);
    }

    // Advanced fields
    setVal('share-create-mask', share ? (share.params['create mask'] || '0664') : '0664');
    setVal('share-dir-mask', share ? (share.params['directory mask'] || '0775') : '0775');
    setCheck('share-inherit-acls', share ? (share.params['inherit acls'] !== 'no') : true);
    setCheck('share-guest-only', share ? (share.params['guest only'] === 'yes') : false);

    toggleRecycleInfo();
    toggleAuditInfo();
    showModalTab('general');
    modal.style.display = 'block';
    if (window.lucide) lucide.createIcons();
}

function showModalTab(tabId) {
    document.querySelectorAll('.modal-tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.m-tab-content').forEach(c => c.style.display = 'none');
    
    const tabs = document.querySelectorAll('.modal-tab');
    if (tabId === 'general') { if(tabs[0]) tabs[0].classList.add('active'); const el = document.getElementById('m-tab-general'); if(el) el.style.display = 'block'; }
    if (tabId === 'access') { if(tabs[1]) tabs[1].classList.add('active'); const el = document.getElementById('m-tab-access'); if(el) el.style.display = 'block'; }
    if (tabId === 'automation-tab') { if(tabs[2]) tabs[2].classList.add('active'); const el = document.getElementById('m-tab-automation-tab'); if(el) el.style.display = 'block'; }
}

function toggleRecycleInfo() {
    const recycleEl = document.getElementById('share-recycle');
    if (!recycleEl) return;
    
    const isChecked = recycleEl.checked;
    const guestEl = document.getElementById('share-guest');
    const isGuest = guestEl ? guestEl.checked : false;
    const info = document.getElementById('recycle-info');
    if (info) info.style.display = isChecked ? 'block' : 'none';

    if (isChecked) {
        const repo = document.getElementById('share-recycle-repo');
        const exclude = document.getElementById('share-recycle-exclude');
        const excludeDir = document.getElementById('share-recycle-exclude-dir');

        if (repo && !repo.value) repo.value = isGuest ? '.recycle/guest' : '.recycle/%U';
        if (exclude && !exclude.value) exclude.value = '*.tmp *.temp ~$* *.bak Thumbs.db';
        if (excludeDir && !excludeDir.value) excludeDir.value = '/tmp /cache .recycle';
    }
}

function toggleAuditInfo() {
    const el = document.getElementById('share-audit');
    const info = document.getElementById('audit-info');
    if (el && info) info.style.display = el.checked ? 'block' : 'none';
}

const initEvents = () => {
    const bind = (id, event, fn) => {
        const el = document.getElementById(id);
        if (el) el[event] = fn;
    };

    bind('share-recycle', 'onchange', () => {
        toggleRecycleInfo();
        const shadowInfo = document.getElementById('shadow-info');
        if (shadowInfo && document.getElementById('share-recycle').checked) {
            shadowInfo.style.display = 'none';
        }
    });

    bind('share-shadow', 'onchange', () => {
        const shadowInfo = document.getElementById('shadow-info');
        const shadowCheck = document.getElementById('share-shadow');
        if (shadowInfo && shadowCheck) {
            shadowInfo.style.display = shadowCheck.checked ? 'block' : 'none';
        }
    });

    bind('share-guest', 'onchange', toggleRecycleInfo);
    bind('share-audit', 'onchange', toggleAuditInfo);
    bind('share-form', 'onsubmit', async (e) => {
        e.preventDefault();
        const getVal = (id) => { const el = document.getElementById(id); return el ? el.value : ''; };
        const getCheck = (id) => { const el = document.getElementById(id); return el ? el.checked : false; };

        const share = {
            name: getVal('share-name'),
            path: getVal('share-path'),
            comment: getVal('share-comment'),
            is_recycle: getCheck('share-recycle'),
            is_audit: getCheck('share-audit'),
            is_shadow_copy: getCheck('share-shadow'),
            audit_open: getCheck('audit-open'),
            params: {
                'read only': getCheck('share-readonly') ? 'yes' : 'no',
                'guest ok': getCheck('share-guest') ? 'yes' : 'no',
                'browseable': getCheck('share-browseable') ? 'yes' : 'no',
                'create mask': getVal('share-create-mask'),
                'directory mask': getVal('share-dir-mask'),
                'force create mode': getVal('share-create-mask'),
                'force directory mode': getVal('share-dir-mask'),
                'inherit acls': getCheck('share-inherit-acls') ? 'yes' : 'no',
                'guest only': getCheck('share-guest-only') ? 'yes' : 'no',
                'recycle:repository': getVal('share-recycle-repo'),
                'recycle:exclude': getVal('share-recycle-exclude'),
                'recycle:exclude_dir': getVal('share-recycle-exclude-dir')
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
    });

    bind('user-form', 'onsubmit', async (e) => {
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
    });

    bind('password-form', 'onsubmit', async (e) => {
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
    });

    bind('global-form', 'onsubmit', async (e) => {
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
    });
};

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initEvents);
} else {
    initEvents();
}

function closeShareModal() {
    const modal = document.getElementById('share-modal');
    if (modal) modal.style.display = 'none';
}

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
    if (!form) return;
    for (const key in data.params) {
        const input = form.querySelector(`[name="${key}"]`);
        if (input) input.value = data.params[key];
    }
}

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
        if (!table) return;
        table.innerHTML = '';
        
        data.forEach(user => {
            table.innerHTML += `<tr>
                <td><strong>${user.username}</strong></td>
                <td><span class="mono">${user.uid}</span></td>
                <td>${user.full_name || '-'}</td>
                <td>
                    <button class="btn-action btn-outline" onclick="openPasswordModal('${user.username}')"><i data-lucide="key" style="width:14px"></i></button>
                    <button class="btn-action btn-outline" style="color: #ef4444;" onclick="deleteUser('${user.username}')"><i data-lucide="trash-2" style="width:14px"></i></button>
                </td>
            </tr>`;
        });
        if (window.lucide) lucide.createIcons();
    } catch (e) { console.error(e); }
}

function openUserModal() {
    const modal = document.getElementById('user-modal');
    if (modal) modal.style.display = 'block';
    const form = document.getElementById('user-form');
    if (form) form.reset();
}

function closeUserModal() {
    const modal = document.getElementById('user-modal');
    if (modal) modal.style.display = 'none';
}

function openPasswordModal(username) {
    const modal = document.getElementById('password-modal');
    if (modal) modal.style.display = 'block';
    const uInput = document.getElementById('pass-username');
    if (uInput) uInput.value = username;
    const uDisplay = document.getElementById('pass-user-display');
    if (uDisplay) uDisplay.innerText = username;
    const pInput = document.getElementById('new-password');
    if (pInput) pInput.value = '';
}

function closePasswordModal() {
    const modal = document.getElementById('password-modal');
    if (modal) modal.style.display = 'none';
}

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
            const color = disk.percent > 90 ? 'var(--status-offline)' : (disk.percent > 75 ? '#f59e0b' : 'var(--status-online)');
            const sharesList = disk.shares && disk.shares.length > 0 ? 
                `<div style="font-size: 0.7rem; color: #64748b; margin-top: 0.8rem; border-top: 1px dashed var(--border-color); padding-top: 0.5rem;">
                    <strong>Ресурсы:</strong> ${disk.shares.join(', ')}
                 </div>` : '';

            container.innerHTML += `
                <div class="stat-card" style="padding: 1.5rem; border-radius: 20px;">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 0.75rem; align-items: center;">
                        <span style="font-weight: 700; font-size: 0.9rem; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 200px;" title="${disk.mount_point}">${disk.mount_point}</span>
                        <span style="font-size: 0.8rem; font-weight: 800; color: ${color};">${disk.percent}%</span>
                    </div>
                    <div style="height: 10px; background: rgba(0,0,0,0.05); border-radius: 10px; overflow: hidden; margin-bottom: 1rem; border: 1px solid rgba(0,0,0,0.02);">
                        <div style="width: ${disk.percent}%; height: 100%; background: ${color}; border-radius: 10px; box-shadow: 0 0 10px ${color}44;"></div>
                    </div>
                    <div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: var(--text-secondary); font-weight: 500;">
                        <span>Использовано: <strong>${disk.used}</strong></span>
                        <span>Всего: ${disk.total}</span>
                    </div>
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
            let actionColor = 'var(--text-primary)';
            if (entry.action === 'unlink') actionColor = 'var(--status-offline)';
            if (entry.action === 'rename') actionColor = '#f59e0b';
            if (entry.action === 'mkdir') actionColor = 'var(--status-online)';

            table.innerHTML += `
                <tr>
                    <td style="font-size: 0.75rem; color: var(--text-secondary);">${entry.timestamp}</td>
                    <td><strong>${entry.user}</strong></td>
                    <td class="mono" style="font-size: 0.75rem;">${entry.ip}</td>
                    <td><span class="badge" style="background: ${actionColor}; color: white; border: none; padding: 2px 8px;">${entry.action.toUpperCase()}</span></td>
                    <td style="font-size: 0.8rem; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="${entry.file}">${entry.file}</td>
                </tr>
            `;
        });
        if (window.lucide) lucide.createIcons();
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
        loadDiskUsage();
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
        const rDays = document.getElementById('auto-recycle-days');
        const sInt = document.getElementById('auto-snap-interval');
        const sKeep = document.getElementById('auto-snap-keep');
        if (rDays) rDays.value = s.recycle_days;
        if (sInt) sInt.value = s.snapshot_interval;
        if (sKeep) sKeep.value = s.snapshot_keep;
    } catch (e) { console.error(e); }
}

async function saveAutomationSettings() {
    const rDays = document.getElementById('auto-recycle-days');
    const sInt = document.getElementById('auto-snap-interval');
    const sKeep = document.getElementById('auto-snap-keep');
    
    const s = {
        recycle_days: rDays ? parseInt(rDays.value) : 0,
        snapshot_interval: sInt ? sInt.value : 'none',
        snapshot_keep: sKeep ? parseInt(sKeep.value) : 0
    };
    try {
        await fetch('/api/automation/save', {
            method: 'POST',
            body: JSON.stringify(s)
        });
        alert('Настройки автоматизации сохранены');
    } catch (e) { alert('Ошибка: ' + e.message); }
}

function showSettingsSection(sectionId, element) {
    document.querySelectorAll('.settings-section').forEach(s => s.style.display = 'none');
    document.querySelectorAll('.settings-nav-item').forEach(i => i.classList.remove('active'));
    
    const section = document.getElementById(`sec-${sectionId}`);
    if (section) section.style.display = 'block';
    if (element) element.classList.add('active');
    
    if (sectionId === 'ad') checkADStatus();
    if (window.lucide) lucide.createIcons();
}

async function checkADStatus() {
    try {
        const res = await fetch('/api/ad/status');
        const data = await res.json();
        const badge = document.getElementById('ad-status-badge');
        const infoBox = document.getElementById('ad-info-box');
        const reqBox = document.getElementById('ad-requirements-box');
        
        if (badge) {
            if (data.joined) {
                badge.innerText = 'ПОДКЛЮЧЕНО';
                badge.className = 'badge online';
                badge.style.background = '';
                badge.style.color = '';
                if (reqBox) reqBox.style.display = 'none';
            } else {
                badge.innerText = 'НЕ В ДОМЕНЕ';
                badge.className = 'badge offline';
                badge.style.background = '';
                badge.style.color = '';
                if (reqBox) reqBox.style.display = 'flex';
            }
        }
        
        if (infoBox) {
            // Показываем подробности (info) только если сервер В ДОМЕНЕ (чтобы видеть ошибки связи)
            // Или если это не стандартная ошибка отсутствия подключения
            if (data.joined && data.info) {
                infoBox.innerText = data.info;
                infoBox.style.display = 'block';
                infoBox.style.background = 'rgba(16, 185, 129, 0.05)';
                infoBox.style.color = '#065f46';
                infoBox.style.border = '1px solid rgba(16, 185, 129, 0.2)';
            } else if (!data.joined && data.info && data.info.includes('Join to domain is not valid')) {
                // Если просто не в домене — не показываем технический мусор
                infoBox.style.display = 'none';
            } else if (data.info) {
                // Если какая-то другая ошибка — показываем
                infoBox.innerText = data.info;
                infoBox.style.display = 'block';
                infoBox.style.background = 'rgba(239, 68, 68, 0.05)';
                infoBox.style.color = '#991b1b';
                infoBox.style.border = '1px solid rgba(239, 68, 68, 0.2)';
            } else {
                infoBox.style.display = 'none';
            }
        }
    } catch (e) { console.error(e); }
}

async function joinAD() {
    const realm = document.getElementById('ad-realm').value;
    const admin = document.getElementById('ad-admin-user').value;
    const pass = document.getElementById('ad-admin-pass').value;
    
    if (!realm || !admin || !pass) {
        alert('Пожалуйста, заполните все поля для ввода в домен');
        return;
    }
    
    if (!confirm(`Вы действительно хотите ввести сервер в домен ${realm}? Это изменит конфигурацию Samba.`)) return;
    
    const btn = event.target.closest('button');
    const originalContent = btn.innerHTML;
    btn.innerHTML = '<i data-lucide="loader-2" style="width:16px; animation: spin 1s linear infinite;"></i> Выполняю...';
    btn.disabled = true;
    
    try {
        const res = await fetch('/api/ad/join', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ realm, admin, password: pass })
        });
        
        if (res.ok) {
            alert('Сервер успешно введен в домен!');
            checkADStatus();
        } else {
            const error = await res.text();
            alert('Ошибка при вводе в домен:\n' + error);
        }
    } catch (e) {
        alert('Ошибка связи с сервером: ' + e.message);
    } finally {
        btn.innerHTML = originalContent;
        btn.disabled = false;
        if (window.lucide) lucide.createIcons();
    }
}

setInterval(updateStatus, 3000);
updateStatus();
