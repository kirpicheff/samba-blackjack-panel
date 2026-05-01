let i18n_data = {};
let currentLang = localStorage.getItem('panel_lang') || 'ru';
let currentFMPath = '/';

async function loadLanguage(lang) {
    if (i18n_data[lang]) return;
    try {
        const res = await fetch(`/locales/${lang}.json`);
        i18n_data[lang] = await res.json();
    } catch (e) {
        console.error(`Failed to load language: ${lang}`, e);
        i18n_data[lang] = i18n_data[lang] || {};
    }
}

function i18n(key, params = {}) {
    if (!i18n_data[currentLang]) return key;
    let text = i18n_data[currentLang][key] || key;
    for (const p in params) {
        text = text.replace(`{${p}}`, params[p]);
    }
    return text;
}

async function setLanguage(lang) {
    await loadLanguage(lang);
    currentLang = lang;
    localStorage.setItem('panel_lang', lang);
    updateStaticTranslations();
    
    // Подсветка кнопок переключения языка
    document.querySelectorAll('.lang-btn').forEach(btn => {
        const isMatch = btn.getAttribute('onclick').includes(`'${lang}'`);
        btn.style.background = isMatch ? 'rgba(255,255,255,0.2)' : 'transparent';
    });

    // Update titles and other dynamic parts
    const activeNav = document.querySelector('.nav-item.active');
    if (activeNav) {
        const tabMatch = activeNav.getAttribute('onclick').match(/'([^']+)'/);
        if (tabMatch) showTab(tabMatch[1], activeNav);
    }
    updateServiceStatus();
}

function updateStaticTranslations() {
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        el.innerText = i18n(key);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        el.placeholder = i18n(key);
    });
    if (window.lucide) {
        lucide.createIcons();
    }
}

async function initI18n() {
    await loadLanguage(currentLang);
    updateStaticTranslations();
    // After loading i18n, we can initialize the rest
    updateStatus();
}

function showTab(tabName, element) {
    document.querySelectorAll('.tab-content').forEach(tab => tab.style.display = 'none');
    document.querySelectorAll('.nav-item').forEach(item => item.classList.remove('active'));

    const tab = document.getElementById(`tab-${tabName}`);
    if (tab) tab.style.display = 'block';
    if (element) element.classList.add('active');

    // Update header title
    const titles = { 
        'dashboard': i18n('nav_dashboard'), 
        'shares': i18n('nav_shares'), 
        'users': i18n('nav_users'), 
        'groups': i18n('nav_groups'),
        'global': i18n('nav_global'),
        'logs': i18n('nav_logs'),
        'audit': i18n('nav_audit'),
        'automation': i18n('nav_automation'),
        'quotas': i18n('nav_quotas')
    };
    const pageTitle = document.getElementById('page-title');
    if (pageTitle) pageTitle.innerText = titles[tabName] || 'Samba Panel';
    
    const pageSubtitle = document.getElementById('page-subtitle');
    if (pageSubtitle && tabName === 'dashboard') {
        pageSubtitle.innerText = i18n('page_dashboard_subtitle');
    }

    // Show apply button only on config tabs
    const applyBtn = document.getElementById('btn-apply');
    if (applyBtn) {
        const configTabs = ['shares', 'global'];
        applyBtn.style.display = configTabs.includes(tabName) ? 'block' : 'none';
        applyBtn.innerText = i18n('btn_apply');
    }

    if (tabName === 'shares') loadShares();
    if (tabName === 'users') loadUsers();
    if (tabName === 'groups') loadGroups();
    if (tabName === 'global') loadGlobalConfig();
    if (tabName === 'logs') loadLogs();
    if (tabName === 'audit') loadAuditLogs();
    if (tabName === 'automation') loadAutomationSettings();
    if (tabName === 'files') loadFileManager();
    if (tabName === 'quotas') loadQuotas();
}

async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        if (response.status === 401) {
            window.location.href = '/login.html';
            return;
        }
        const data = await response.json();

        // Универсальное извлечение сессий
        let sessions = [];
        if (data.sessions) {
            sessions = Array.isArray(data.sessions) ? data.sessions : Object.values(data.sessions);
        } else if (data.processes && data.processes.session) {
            sessions = Array.isArray(data.processes.session) ? data.processes.session : [data.processes.session];
        }

        const openFiles = (data.open_files) ? 
            (Array.isArray(data.open_files) ? data.open_files : Object.values(data.open_files)) : 
            ((data.locks && data.locks.sharemode) ? data.locks.sharemode : []);
            
        const version = data.version || (data.processes && data.processes.Samba_version) || 'Samba Server';

        const sessionEl = document.getElementById('session-count');
        const fileEl = document.getElementById('file-count');
        const versionEl = document.getElementById('samba-version');
        
        if (sessionEl) sessionEl.innerText = sessions.length;
        if (fileEl) fileEl.innerText = openFiles.length;
        if (versionEl) versionEl.innerText = version;

        const sessionTable = document.getElementById('sessions-table-body');
        if (sessionTable) {
            sessionTable.innerHTML = '';
            sessions.forEach(s => {
                // Пытаемся найти пользователя под разными именами
                const user = s.Username || s.user || s.username || 'nobody';
                // Пытаемся найти хост
                const machine = s.Machine || s.remote_machine || s.machine || s.hostname || '-';
                // Пытаемся найти протокол
                const protocol = s.Protocol || s.protocol_version || s.protocol || '-';
                
                sessionTable.innerHTML += `<tr>
                    <td><strong>${user}</strong></td>
                    <td>${machine}</td>
                    <td><span class="mono">${protocol}</span></td>
                </tr>`;
            });

            if (sessions.length === 0) {
                sessionTable.innerHTML = `<tr><td colspan="3" style="text-align:center; padding: 2rem; opacity: 0.5;">${i18n('label_no_sessions') || 'No active sessions'}</td></tr>`;
            }
        }
        
        updateServiceStatus();
        loadDiskUsage();
        loadDiscoveryStatus();
    } catch (e) { console.error('Update status error:', e); }
}

async function updateServiceStatus() {
    try {
        const res = await fetch('/api/service/status');
        const status = await res.text();
        const topBadge = document.getElementById('samba-status-badge');
        
        if (topBadge) {
            if (status === 'active') {
                topBadge.innerHTML = `<span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%;"></span> ${i18n('smb_online')}`;
                topBadge.className = 'badge online';
            } else {
                topBadge.innerHTML = `<span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%;"></span> ${i18n('smb_offline')}`;
                topBadge.className = 'badge offline';
            }
        }
    } catch (e) { console.error(e); }
}

async function controlService(action) {
    if (!confirm(i18n('confirm_service_action', { action }))) return;
    
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
            alert(i18n('error_service_control') + error);
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
                `<span class="badge online" style="font-size:0.6rem">${i18n('recycle_active')}</span>` : 
                `<span class="badge" style="color:#64748b; font-size:0.6rem">${i18n('recycle_off')}</span>`;
            
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
    if (title) title.innerText = share ? i18n('modal_share_title_edit') : i18n('modal_share_title_new');
    
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
    setVal('share-hosts-allow', share ? (share.params['hosts allow'] || '') : '');
    setVal('share-hosts-deny', share ? (share.params['hosts deny'] || '') : '');

    // Reset FS permissions tab fields
    setVal('fs-owner', '');
    setVal('fs-group', '');
    setVal('fs-mode', '');
    setCheck('fs-recursive', false);
    const aclOut = document.getElementById('fs-acl-output');
    if (aclOut) aclOut.innerText = '';

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
    if (tabId === 'permissions') { 
        if(tabs[3]) tabs[3].classList.add('active'); 
        const el = document.getElementById('m-tab-permissions'); 
        if(el) el.style.display = 'block';
        const path = document.getElementById('share-path').value;
        if (path) {
            fillUserGroupSelects().then(() => loadPathPermissions(path));
        }
    }
}

async function fillUserGroupSelects() {
    try {
        const [uRes, gRes] = await Promise.all([
            fetch('/api/users'),
            fetch('/api/groups')
        ]);
        const users = await uRes.json();
        const groups = await gRes.json();

        const uSelect = document.getElementById('fs-owner');
        const gSelect = document.getElementById('fs-group');
        if (!uSelect || !gSelect) return;

        uSelect.innerHTML = '';
        gSelect.innerHTML = '';

        users.forEach(u => {
            const opt = document.createElement('option');
            opt.value = u.username;
            opt.innerText = u.username;
            uSelect.appendChild(opt);
        });

        groups.forEach(g => {
            const opt = document.createElement('option');
            opt.value = g.name;
            opt.innerText = g.name;
            gSelect.appendChild(opt);
        });
    } catch (e) { console.error(e); }
}

async function loadPathPermissions(path) {
    const aclOutput = document.getElementById('fs-acl-output');
    if (aclOutput) aclOutput.innerText = i18n('fs_perm_applying');

    try {
        const res = await fetch(`/api/fs/permissions?path=${encodeURIComponent(path)}`);
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        
        const setSelectVal = (id, val) => {
            const el = document.getElementById(id);
            if (!el) return;
            // Проверяем, есть ли такое значение в списке, если нет - добавляем (для системных юзеров типа root)
            let exists = Array.from(el.options).some(opt => opt.value === val);
            if (!exists && val) {
                const opt = document.createElement('option');
                opt.value = val;
                opt.innerText = val;
                el.appendChild(opt);
            }
            el.value = val;
        };

        setSelectVal('fs-owner', data.owner);
        setSelectVal('fs-group', data.group);
        
        const modeEl = document.getElementById('fs-mode');
        if (modeEl) modeEl.value = data.mode;
        updatePermChecks(data.mode);
        
        if (aclOutput) aclOutput.innerText = data.acls || i18n('fs_acl_empty');
    } catch (e) {
        if (aclOutput) aclOutput.innerText = i18n('fs_acl_error') + ': ' + e.message;
    }
}

function updatePermChecks(mode) {
    if (!mode) return;
    // Remove leading 0 if present, take last 3 digits
    const m = mode.length > 3 ? mode.slice(-3) : mode.padStart(3, '0');
    const u = parseInt(m[0]);
    const g = parseInt(m[1]);
    const o = parseInt(m[2]);

    document.querySelectorAll('.perm-check').forEach(cb => {
        const type = cb.dataset.type;
        const bit = parseInt(cb.dataset.bit);
        let val = 0;
        if (type === 'u') val = u;
        else if (type === 'g') val = g;
        else if (type === 'o') val = o;
        
        cb.checked = (val & bit) !== 0;
    });
}

function updateOctalFromChecks() {
    let u = 0, g = 0, o = 0;
    document.querySelectorAll('.perm-check').forEach(cb => {
        if (!cb.checked) return;
        const bit = parseInt(cb.dataset.bit);
        if (cb.dataset.type === 'u') u += bit;
        else if (cb.dataset.type === 'g') g += bit;
        else if (cb.dataset.type === 'o') o += bit;
    });
    const mode = `0${u}${g}${o}`;
    const modeInput = document.getElementById('fs-mode');
    if (modeInput) modeInput.value = mode;
}

async function savePathPermissions() {
    const path = document.getElementById('share-path').value;
    if (!path) { alert(i18n('fs_perm_error_path')); return; }

    const req = {
        path: path,
        owner: document.getElementById('fs-owner').value,
        group: document.getElementById('fs-group').value,
        mode: document.getElementById('fs-mode').value,
        recursive: document.getElementById('fs-recursive').checked
    };

    const btn = event.target;
    const originalText = btn.innerText;
    btn.innerText = i18n('fs_perm_applying');
    btn.disabled = true;

    try {
        const res = await fetch('/api/fs/permissions/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(req)
        });

        if (res.ok) {
            alert(i18n('fs_perm_success'));
            loadPathPermissions(path);
        } else {
            const err = await res.text();
            alert(i18n('error') + ': ' + err);
        }
    } catch (e) {
        alert(i18n('error_server_connection'));
    } finally {
        btn.innerText = originalText;
        btn.disabled = false;
    }
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
                'recycle:exclude_dir': getVal('share-recycle-exclude-dir'),
                'hosts allow': getVal('share-hosts-allow'),
                'hosts deny': getVal('share-hosts-deny')
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
            alert(i18n('error_save_share'));
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
            alert(i18n('error_save_user') + ': ' + error);
        }
    });

    bind('group-form', 'onsubmit', async (e) => {
        e.preventDefault();
        const name = document.getElementById('group-name').value;

        const res = await fetch('/api/groups/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });

        if (res.ok) {
            closeGroupModal();
            loadGroups();
        } else {
            const error = await res.text();
            alert(i18n('error_create_group') + ': ' + error);
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
            alert(i18n('password_changed_success'));
            closePasswordModal();
        } else {
            const error = await res.text();
            alert(i18n('error_change_password') + ': ' + error);
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

        if (res.ok) alert(i18n('save_success'));
    });
    
    setLanguage(currentLang);
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
    if (!confirm(i18n('confirm_delete_share', { name }))) return;
    
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
    btn.innerText = i18n('btn_applying');
    btn.disabled = true;
    btn.style.opacity = '0.7';

    try {
        const res = await fetch('/api/service/apply', { method: 'POST' });
        if (res.ok) {
            alert(i18n('apply_success'));
        } else {
            const error = await res.text();
            alert(i18n('error_config_validation') + ':\n' + error);
        }
    } catch (e) {
        alert(i18n('error_server_connection'));
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
    if (!confirm(i18n('confirm_delete_user', { username }))) return;

    const res = await fetch('/api/users/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username })
    });

    if (res.ok) {
        loadUsers();
    } else {
        const error = await res.text();
        alert(i18n('error_delete_user') + ': ' + error);
    }
}

async function loadGroups() {
    try {
        const response = await fetch('/api/groups');
        if (response.status === 401) return;
        const groups = await response.json();
        const table = document.getElementById('groups-table-body');
        if (!table) return;
        table.innerHTML = '';
        
        groups.forEach(group => {
            const members = group.members && group.members.length > 0 
                ? group.members.map(m => `<span class="badge" style="background: rgba(59, 130, 246, 0.1); color: var(--accent-blue); border:none; margin: 2px;">${m}</span>`).join('')
                : `<span style="color: #94a3b8; font-size: 0.8rem;">${i18n('label_no_members')}</span>`;

            table.innerHTML += `<tr>
                <td><strong>${group.name}</strong></td>
                <td><span class="mono">${group.gid}</span></td>
                <td style="max-width: 300px; white-space: normal;">${members}</td>
                <td>
                    <button class="btn-action btn-outline" onclick="openGroupMembersModal('${group.name}')" title="Участники"><i data-lucide="users" style="width:14px"></i></button>
                    <button class="btn-action btn-outline" style="color: #ef4444;" onclick="deleteGroup('${group.name}')" title="Удалить"><i data-lucide="trash-2" style="width:14px"></i></button>
                </td>
            </tr>`;
        });
        if (window.lucide) lucide.createIcons();
    } catch (e) { console.error(e); }
}

function openGroupModal() {
    const modal = document.getElementById('group-modal');
    if (modal) modal.style.display = 'block';
    const form = document.getElementById('group-form');
    if (form) form.reset();
}

function closeGroupModal() {
    const modal = document.getElementById('group-modal');
    if (modal) modal.style.display = 'none';
}

async function deleteGroup(name) {
    if (!confirm(i18n('confirm_delete_group', { name }))) return;

    const res = await fetch('/api/groups/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name })
    });

    if (res.ok) {
        loadGroups();
    } else {
        const error = await res.text();
        alert(i18n('error_delete_group') + ': ' + error);
    }
}

let currentActiveGroup = '';

async function openGroupMembersModal(groupName) {
    currentActiveGroup = groupName;
    const modal = document.getElementById('group-members-modal');
    const title = document.getElementById('group-members-title');
    if (modal) modal.style.display = 'block';
    if (title) title.innerText = groupName;

    // Загружаем список всех пользователей для выпадающего списка
    const usersRes = await fetch('/api/users');
    const allUsers = await usersRes.json();
    const select = document.getElementById('group-add-user-select');
    if (select) {
        select.innerHTML = `<option value="">${i18n('label_select_user')}</option>`;
        allUsers.forEach(u => {
            select.innerHTML += `<option value="${u.username}">${u.username} (${u.full_name || ''})</option>`;
        });
    }

    loadGroupMembers(groupName);
}

async function loadGroupMembers(groupName) {
    const res = await fetch('/api/groups');
    const groups = await res.json();
    const group = groups.find(g => g.name === groupName);
    const table = document.getElementById('group-members-table-body');
    if (!table || !group) return;

    table.innerHTML = '';
    if (group.members) {
        group.members.forEach(m => {
            table.innerHTML += `<tr>
                <td><strong>${m}</strong></td>
                <td>
                    <button class="btn-action btn-outline" style="color: #ef4444; padding: 4px 8px;" onclick="removeGroupMember('${group.name}', '${m}')">
                        <i data-lucide="user-minus" style="width:14px"></i> ${i18n('table_actions')}
                    </button>
                </td>
            </tr>`;
        });
    }
    if (window.lucide) lucide.createIcons();
}

function closeGroupMembersModal() {
    const modal = document.getElementById('group-members-modal');
    if (modal) modal.style.display = 'none';
}

async function addGroupMember() {
    const select = document.getElementById('group-add-user-select');
    const username = select.value;
    if (!username) return;

    const res = await fetch('/api/groups/member', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ group: currentActiveGroup, username, action: 'add' })
    });

    if (res.ok) {
        loadGroupMembers(currentActiveGroup);
        loadGroups();
    } else {
        const error = await res.text();
        alert(i18n('error') + ': ' + error);
    }
}

async function removeGroupMember(group, username) {
    if (!confirm(i18n('confirm_remove_member', { username, group }))) return;

    const res = await fetch('/api/groups/member', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ group, username, action: 'remove' })
    });

    if (res.ok) {
        loadGroupMembers(group);
        loadGroups();
    } else {
        const error = await res.text();
        alert(i18n('error') + ': ' + error);
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
                    <strong>${i18n('disk_resources')}:</strong> ${disk.shares.join(', ')}
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
                        <span>${i18n('disk_used')}: <strong>${disk.used}</strong></span>
                        <span>${i18n('disk_total')}: ${disk.total}</span>
                    </div>
                    ${sharesList}
                </div>
            `;
        });
    } catch (e) { console.error(e); }
}

let logSocket = null;

async function loadLogs() {
    const output = document.getElementById('log-output');
    if (!output) return;

    if (logSocket && logSocket.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws/logs`;
    
    if (logSocket) logSocket.close();
    
    logSocket = new WebSocket(wsUrl);
    
    // При первом подключении загружаем текущий хвост лога
    try {
        const initialRes = await fetch('/api/logs');
        output.innerText = await initialRes.text();
        output.scrollTop = output.scrollHeight;
    } catch(e) { console.error(e); }

    logSocket.onmessage = (event) => {
        output.innerText += event.data;
        output.scrollTop = output.scrollHeight;
        
        // Ограничиваем количество строк (например, последние 1000)
        const lines = output.innerText.split('\n');
        if (lines.length > 1000) {
            output.innerText = lines.slice(lines.length - 1000).join('\n');
        }
    };

    logSocket.onclose = () => {
        console.log('Log WebSocket closed');
        logSocket = null;
    };
    
    logSocket.onerror = (err) => {
        console.error('Log WebSocket error', err);
        logSocket = null;
    };
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

// Автообновление данных каждые 5 секунд
setInterval(() => {
    // Логи теперь через WebSocket, поэтому их не обновляем здесь
    const auditTab = document.getElementById('tab-audit');
    if (auditTab && auditTab.style.display === 'block') loadAuditLogs();
}, 5000);

async function clearRecycleBins() {
    if (!confirm(i18n('confirm_clear_recycle'))) return;

    try {
        const res = await fetch('/api/maintenance/clear-recycle', { method: 'POST' });
        const results = await res.json();
        
        const successCount = results.filter(r => !r.error).length;
        const failCount = results.filter(r => r.error).length;
        
        let message = i18n('msg_clear_recycle_success', { count: successCount });
        if (failCount > 0) message += `\n${i18n('msg_errors')}: ${failCount}`;
        
        alert(message);
        loadDiskUsage();
    } catch (e) {
        alert(i18n('error_clear_recycle') + ': ' + e.message);
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
        alert(i18n('automation_save_success'));
    } catch (e) { alert(i18n('error') + ': ' + e.message); }
}

function showSettingsSection(sectionId, element) {
    document.querySelectorAll('.settings-section').forEach(s => s.style.display = 'none');
    document.querySelectorAll('.settings-nav-item').forEach(i => i.classList.remove('active'));
    
    const section = document.getElementById(`sec-${sectionId}`);
    if (section) section.style.display = 'block';
    if (element) element.classList.add('active');
    
    if (sectionId === 'ad') checkADStatus();
    if (sectionId === 'discovery') loadDiscoveryManagement();
    if (sectionId === 'panel-admins') loadPanelAdmins();
    if (window.lucide) lucide.createIcons();
}

async function loadDiscoveryStatus() {
    try {
        const res = await fetch('/api/discovery/status');
        const services = await res.json();
        const container = document.getElementById('discovery-status-list');
        if (!container) return;

        container.innerHTML = '';
        services.forEach(s => {
            const statusClass = s.active ? 'online' : 'offline';
            const statusText = s.active ? i18n('status_active') : i18n('status_stopped');
            const label = s.name === 'wsdd2' ? 'Windows (WSDD)' : 'macOS/Linux (Avahi)';
            
            container.innerHTML += `
                <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem;">
                    <span style="color: var(--text-secondary);">${label}</span>
                    <span class="badge ${statusClass}" style="font-size: 0.65rem; padding: 2px 8px;">${statusText}</span>
                </div>
            `;
        });
    } catch (e) { console.error(e); }
}

async function loadDiscoveryManagement() {
    try {
        const res = await fetch('/api/discovery/status');
        const services = await res.json();
        const container = document.getElementById('discovery-management-container');
        if (!container) return;

        container.innerHTML = '';
        services.forEach(s => {
            const label = s.name === 'wsdd2' ? i18n('label_wsdd') : i18n('label_avahi');
            const desc = s.name === 'wsdd2' ? i18n('desc_wsdd') : i18n('desc_avahi');
            const statusText = s.active ? i18n('status_running') : i18n('status_stopped');
            const btnAction = s.active ? 'stop' : 'start';
            const btnText = s.active ? i18n('btn_stop') : i18n('btn_start');
            const btnClass = s.active ? 'btn-outline' : 'btn-primary';

            container.innerHTML += `
                <div class="stat-card" style="margin-bottom: 1rem; border: 1px solid var(--border-color);">
                    <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                        <div>
                            <h3 style="font-size: 0.95rem; font-weight: 700; margin-bottom: 4px;">${label}</h3>
                            <p style="font-size: 0.8rem; color: var(--text-secondary);">${desc}</p>
                            <div style="margin-top: 1rem; display: flex; align-items: center; gap: 8px;">
                                <span class="badge ${s.active ? 'online' : 'offline'}">${statusText}</span>
                                ${!s.installed ? `<span class="badge" style="background: #fee2e2; color: #ef4444; border:none;">${i18n('label_not_installed')}</span>` : ''}
                            </div>
                        </div>
                        <div style="display: flex; flex-direction: column; gap: 8px;">
                            <button type="button" class="btn-action ${btnClass}" onclick="controlDiscoveryService('${s.name}', '${btnAction}')" ${!s.installed ? 'disabled' : ''}>
                                ${btnText}
                            </button>
                            ${s.installed ? `
                                <button type="button" class="btn-action btn-outline" style="font-size: 0.75rem; padding: 4px;" onclick="controlDiscoveryService('${s.name}', 'restart')">
                                    ${i18n('btn_restart')}
                                </button>
                            ` : ''}
                        </div>
                    </div>
                </div>
            `;
        });
    } catch (e) { console.error(e); }
}

async function controlDiscoveryService(service, action) {
    try {
        const res = await fetch('/api/discovery/control', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ service, action })
        });
        if (res.ok) {
            loadDiscoveryManagement();
            loadDiscoveryStatus();
        } else {
            const err = await res.text();
            alert(i18n('error') + ': ' + err);
        }
    } catch (e) { console.error(e); }
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
                badge.innerText = i18n('label_joined');
                badge.className = 'badge online';
                badge.style.background = '';
                badge.style.color = '';
                if (reqBox) reqBox.style.display = 'none';
                runADHealthCheck(); // Автозапуск при входе в раздел
            } else {
                badge.innerText = i18n('label_not_joined');
                badge.className = 'badge offline';
                badge.style.background = '';
                badge.style.color = '';
                if (reqBox) reqBox.style.display = 'flex';
            }
        }

        const healthSec = document.getElementById('ad-health-section');
        if (healthSec) healthSec.style.display = 'block';
        
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
        alert(i18n('error_fill_all'));
        return;
    }
    
    if (!confirm(i18n('confirm_ad_join', { realm }))) return;
    
    const btn = event.target.closest('button');
    const originalContent = btn.innerHTML;
    btn.innerHTML = `<i data-lucide="loader-2" style="width:16px; animation: spin 1s linear infinite;"></i> ${i18n('label_executing')}`;
    btn.disabled = true;
    
    try {
        const res = await fetch('/api/ad/join', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ realm, admin, password: pass })
        });
        
        if (res.ok) {
            alert(i18n('msg_ad_join_success'));
            checkADStatus();
        } else {
            const error = await res.text();
            alert(i18n('error') + ': ' + error);
        }
    } catch (e) {
        alert(i18n('error_server_connection') + ': ' + e.message);
    } finally {
        btn.innerHTML = originalContent;
        btn.disabled = false;
        if (window.lucide) lucide.createIcons();
    }
}

async function runADHealthCheck() {
    const container = document.getElementById('ad-health-results');
    const lastUpdateEl = document.getElementById('ad-health-last-update');
    if (!container) return;

    // Временная индикация загрузки
    const btn = event?.target?.closest('button');
    if (btn) btn.classList.add('loading-spin');

    try {
        const res = await fetch('/api/ad/health');
        const data = await res.json();
        
        container.innerHTML = '';
        data.checks.forEach(check => {
            const colors = {
                'ok': { bg: 'rgba(16, 185, 129, 0.05)', text: '#065f46', border: 'rgba(16, 185, 129, 0.2)', icon: 'check-circle' },
                'warning': { bg: 'rgba(245, 158, 11, 0.05)', text: '#b45309', border: 'rgba(245, 158, 11, 0.2)', icon: 'alert-triangle' },
                'error': { bg: 'rgba(239, 68, 68, 0.05)', text: '#991b1b', border: 'rgba(239, 68, 68, 0.2)', icon: 'x-circle' }
            };
            const c = colors[check.status] || colors['error'];

            // Маппинг заголовков и сообщений с бэкенда
            const nameMap = {
                'Связь с контроллером': i18n('ad_check_conn'),
                'Синхронизация времени': i18n('ad_check_time'),
                'Доверительные отношения': i18n('ad_check_trust'),
                'Winbind RPC': i18n('ad_check_rpc'),
                'Kerberos Keytab': i18n('ad_check_keytab')
            };
            const msgMap = {
                'Время сервера совпадает с DC': i18n('ad_msg_time_ok'),
                'Не удалось проверить время через net ads': i18n('ad_msg_time_warn'),
                'Keytab файл присутствует и валиден': i18n('ad_msg_keytab_ok'),
                'Keytab не найден или не читается': i18n('ad_msg_keytab_err')
            };

            const localizedName = nameMap[check.name] || check.name;
            const localizedMsg = msgMap[check.message] || check.message;

            container.innerHTML += `
                <div style="padding: 1rem; background: ${c.bg}; border: 1px solid ${c.border}; border-radius: 12px; display: flex; align-items: flex-start; gap: 0.75rem;">
                    <i data-lucide="${c.icon}" style="color: ${c.text}; width: 18px; height: 18px; flex-shrink: 0; margin-top: 2px;"></i>
                    <div style="flex-grow: 1;">
                        <div style="display: flex; justify-content: space-between;">
                            <span style="font-weight: 700; font-size: 0.85rem; color: ${c.text};">${localizedName}</span>
                            <span style="font-size: 0.7rem; font-weight: 800; text-transform: uppercase; color: ${c.text}; opacity: 0.7;">${check.status}</span>
                        </div>
                        <p style="font-size: 0.75rem; color: ${c.text}; margin-top: 4px; opacity: 0.9; font-family: 'JetBrains Mono', monospace;">${localizedMsg}</p>
                    </div>
                </div>
            `;
        });

        if (lastUpdateEl) lastUpdateEl.innerText = i18n('label_last_check') + ': ' + data.last_update;
        if (window.lucide) lucide.createIcons();
    } catch (e) {
        console.error(e);
    } finally {
        if (btn) btn.classList.remove('loading-spin');
    }
}

async function loadPanelAdmins() {
    const res = await fetch('/api/panel/admins');
    const admins = await res.json();
    const table = document.getElementById('panel-admins-table-body');
    if (!table) return;

    table.innerHTML = '';
    admins.forEach(a => {
        const isSelf = a.username === localStorage.getItem('panel_user');
        const deleteBtn = (a.username !== 'admin' && !isSelf) 
            ? `<button class="btn-action btn-outline" style="color: #ef4444; padding: 4px 8px;" onclick="deletePanelAdmin('${a.username}')"><i data-lucide="trash-2" style="width:14px"></i></button>`
            : `<span style="color: var(--text-secondary); font-size: 0.7rem;">${i18n('label_protected')}</span>`;

        table.innerHTML += `<tr>
            <td><strong>${a.username} ${isSelf ? `<span style="color: var(--accent-blue)">(${i18n('label_you')})</span>` : ''}</strong></td>
            <td><span class="badge" style="background: rgba(59, 130, 246, 0.1); color: var(--accent-blue); border:none;">${a.role}</span></td>
            <td>${deleteBtn}</td>
        </tr>`;
    });
    if (window.lucide) lucide.createIcons();
}

async function changePanelPassword() {
    const oldPass = document.getElementById('admin-old-pass').value;
    const newPass = document.getElementById('admin-new-pass').value;
    if (!oldPass || !newPass) { alert(i18n('error_fill_both')); return; }

    const res = await fetch('/api/panel/admins/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ old_password: oldPass, new_password: newPass })
    });
    if (res.ok) {
        alert(i18n('password_changed_success'));
        document.getElementById('admin-old-pass').value = '';
        document.getElementById('admin-new-pass').value = '';
    } else {
        const err = await res.text();
        alert(i18n('error') + ': ' + err);
    }
}

function openAdminModal() { document.getElementById('admin-modal').style.display = 'block'; }
function closeAdminModal() { document.getElementById('admin-modal').style.display = 'none'; }

document.getElementById('admin-form')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('new-admin-user').value;
    const password = document.getElementById('new-admin-pass').value;
    const role = document.getElementById('new-admin-role').value;

    const res = await fetch('/api/panel/admins/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, role })
    });
    if (res.ok) {
        closeAdminModal();
        loadPanelAdmins();
    } else {
        const err = await res.text();
        alert(i18n('error') + ': ' + err);
    }
});

async function deletePanelAdmin(username) {
    if (!confirm(i18n('confirm_delete_admin', { username }))) return;
    const res = await fetch(`/api/panel/admins/delete?username=${username}`, { method: 'DELETE' });
    if (res.ok) loadPanelAdmins();
}

async function loadFileManager(path = null) {
    if (path !== null) currentFMPath = path;
    const container = document.getElementById('fm-table-body');
    const pathEl = document.getElementById('fm-current-path');
    if (!container || !pathEl) return;

    // Рендерим хлебные крошки
    pathEl.innerHTML = '';
    const parts = currentFMPath.split('/').filter(p => p);
    
    // Корень
    const rootSpan = document.createElement('span');
    rootSpan.innerHTML = '<i data-lucide="hard-drive" style="width:12px; height:12px; vertical-align:middle; margin-right:4px;"></i> /';
    rootSpan.className = 'breadcrumb-item';
    rootSpan.onclick = () => loadFileManager('/');
    pathEl.appendChild(rootSpan);

    let accumulatedPath = '';
    parts.forEach((part, index) => {
        accumulatedPath += '/' + part;
        const sep = document.createElement('span');
        sep.innerText = ' › ';
        sep.style.opacity = '0.4';
        sep.style.margin = '0 4px';
        pathEl.appendChild(sep);

        const partSpan = document.createElement('span');
        partSpan.innerText = part;
        partSpan.className = 'breadcrumb-item';
        if (index === parts.length - 1) {
            partSpan.classList.add('active');
        } else {
            const targetPath = accumulatedPath;
            partSpan.onclick = () => loadFileManager(targetPath);
        }
        pathEl.appendChild(partSpan);
    });
    if (window.lucide) lucide.createIcons();

    try {
        const res = await fetch(`/api/files/list?path=${encodeURIComponent(currentFMPath)}`);
        if (!res.ok) throw new Error(await res.text());
        const files = await res.json();

        container.innerHTML = '';
        if (!files || files.length === 0) {
            container.innerHTML = `<tr><td colspan="4" style="text-align:center; padding: 2rem; color: var(--text-secondary); opacity: 0.5;">${i18n('fs_acl_empty') || 'No files found'}</td></tr>`;
            return;
        }

        files.forEach(f => {
            const icon = f.is_dir ? 'folder' : 'file';
            const size = f.is_dir ? '-' : formatBytes(f.size);
            const fullPath = (currentFMPath === '/' ? '' : currentFMPath) + '/' + f.name;
            const nameClick = f.is_dir ? `onclick="loadFileManager('${fullPath.replace(/\\/g, '/')}')"` : '';
            
            container.innerHTML += `
                <tr>
                    <td ${nameClick} style="${f.is_dir ? 'cursor:pointer; color: var(--accent-blue); font-weight:600;' : ''}">
                        <i data-lucide="${icon}" style="width:16px; vertical-align: middle; margin-right: 8px; opacity: 0.7;"></i>
                        ${f.name}
                    </td>
                    <td class="mono" style="font-size: 0.8rem;">${size}</td>
                    <td class="mono" style="font-size: 0.8rem; color: var(--text-secondary);">${f.mod_time}</td>
                    <td>
                        <button class="btn-action btn-outline" onclick="fmRename('${f.name}')"><i data-lucide="edit-3" style="width:14px"></i></button>
                        <button class="btn-action btn-outline" onclick="openPathPermissionsFM('${f.name}')" title="${i18n('modal_tab_permissions')}"><i data-lucide="shield" style="width:14px"></i></button>
                        <button class="btn-action btn-outline" style="color: #ef4444;" onclick="fmDelete('${f.name}')"><i data-lucide="trash-2" style="width:14px"></i></button>
                    </td>
                </tr>
            `;
        });
        if (window.lucide) lucide.createIcons();
    } catch (e) {
        container.innerHTML = `<tr><td colspan="4" style="text-align:center; padding: 2rem; color: #ef4444;">${i18n('fm_error_load')}: ${e.message}</td></tr>`;
    }
}

function fmGoUp() {
    if (currentFMPath === '/' || currentFMPath === '') return;
    const parts = currentFMPath.split('/').filter(p => p);
    parts.pop();
    currentFMPath = '/' + parts.join('/');
    if (currentFMPath === '') currentFMPath = '/';
    loadFileManager();
}

async function fmNewFolder() {
    const name = prompt(i18n('fm_new_folder_prompt'));
    if (!name) return;

    try {
        const res = await fetch('/api/files/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: currentFMPath, name })
        });
        if (res.ok) loadFileManager();
        else alert(i18n('fm_error_mkdir') + ': ' + await res.text());
    } catch (e) { console.error(e); }
}

async function fmRename(oldName) {
    const newName = prompt(i18n('fm_rename_prompt', { name: oldName }), oldName);
    if (!newName || newName === oldName) return;

    try {
        const res = await fetch('/api/files/rename', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: currentFMPath, old_name: oldName, new_name: newName })
        });
        if (res.ok) loadFileManager();
        else alert(i18n('fm_error_rename') + ': ' + await res.text());
    } catch (e) { console.error(e); }
}

async function fmDelete(name) {
    if (!confirm(i18n('fm_delete_confirm', { name }))) return;

    try {
        const res = await fetch('/api/files/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: currentFMPath, name })
        });
        if (res.ok) loadFileManager();
        else alert(i18n('fm_error_delete') + ': ' + await res.text());
    } catch (e) { console.error(e); }
}

function openPathPermissionsFM(name) {
    const fullPath = (currentFMPath.endsWith('/') ? currentFMPath : currentFMPath + '/') + name;
    openShareModal({ name: name, path: fullPath, params: {} });
    showModalTab('permissions');
}

function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) modal.style.display = 'none';
}

initI18n();
setInterval(updateStatus, 3000);

async function loadQuotas() {
    const container = document.getElementById('quota-table-body');
    if (!container) return;

    try {
        const res = await fetch('/api/quotas/list');
        const data = await res.json();

        container.innerHTML = '';
        data.forEach(q => {
            const softMB = Math.round(q.soft_limit / 1024);
            const hardMB = Math.round(q.hard_limit / 1024);
            const usedMB = Math.round(q.used / 1024);
            
            let barColor = 'var(--accent-blue)';
            if (q.usage_pct > 80) barColor = '#f59e0b';
            if (q.usage_pct > 95) barColor = '#ef4444';

            container.innerHTML += `
                <tr>
                    <td><strong>${q.user}</strong></td>
                    <td style="width: 300px;">
                        <div style="display: flex; align-items: center; gap: 10px;">
                            <div style="flex-grow: 1; background: #eee; height: 8px; border-radius: 4px; overflow: hidden;">
                                <div style="width: ${Math.min(q.usage_pct, 100)}%; background: ${barColor}; height: 100%; transition: width 0.5s;"></div>
                            </div>
                            <span style="font-size: 0.75rem; color: var(--text-secondary); width: 80px;">${q.usage_pct}% (${usedMB.toFixed(1)} MB)</span>
                        </div>
                    </td>
                    <td class="mono">${softMB > 0 ? softMB + ' MB' : '-'}</td>
                    <td class="mono">${hardMB > 0 ? hardMB + ' MB' : '-'}</td>
                    <td style="text-align: right;">
                        <button class="btn-action btn-outline" onclick="editQuota('${q.user}', ${softMB}, ${hardMB})">
                            <i data-lucide="edit-3" style="width: 14px;"></i>
                        </button>
                    </td>
                </tr>
            `;
        });
        if (window.lucide) lucide.createIcons();
    } catch (e) { console.error(e); }
}

function editQuota(user, soft, hard) {
    document.getElementById('quota-user').value = user;
    document.getElementById('quota-soft').value = soft;
    document.getElementById('quota-hard').value = hard;
    document.getElementById('quota-modal-title').innerText = i18n('modal_quota_title').replace('{name}', user);
    document.getElementById('quota-modal').style.display = 'block';
}

async function saveQuota() {
    const user = document.getElementById('quota-user').value;
    const soft = parseInt(document.getElementById('quota-soft').value) || 0;
    const hard = parseInt(document.getElementById('quota-hard').value) || 0;

    try {
        const res = await fetch('/api/quotas/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user, soft_limit: soft, hard_limit: hard })
        });
        if (res.ok) {
            closeModal('quota-modal');
            loadQuotas();
            showToast(i18n('quota_save_success'));
        } else {
            alert(await res.text());
        }
    } catch (e) { console.error(e); }
}
