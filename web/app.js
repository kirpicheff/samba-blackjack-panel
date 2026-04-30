function showTab(tabName, el) {
    document.querySelectorAll('.tab-content').forEach(t => t.style.display = 'none');
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    
    document.getElementById('tab-' + tabName).style.display = 'block';
    if (el) el.classList.add('active');

    if (tabName === 'shares') loadShares();
    if (tabName === 'users') loadUsers();
    if (tabName === 'global') loadGlobalConfig();
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
    document.getElementById('share-readonly').checked = share ? (share.params['read only'] !== 'no') : false;
    document.getElementById('share-guest').checked = share ? (share.params['guest ok'] !== 'no') : true;
    document.getElementById('share-browseable').checked = share ? (share.params['browseable'] !== 'no') : true;

    // Advanced fields
    document.getElementById('share-create-mask').value = share ? (share.params['create mask'] || '0664') : '0664';
    document.getElementById('share-dir-mask').value = share ? (share.params['directory mask'] || '0775') : '0775';
    document.getElementById('share-inherit-acls').checked = share ? (share.params['inherit acls'] !== 'no') : true;
    document.getElementById('share-guest-only').checked = share ? (share.params['guest only'] === 'yes') : false;

    toggleRecycleInfo();
    modal.style.display = 'block';
}

function toggleRecycleInfo() {
    const isChecked = document.getElementById('share-recycle').checked;
    document.getElementById('recycle-info').style.display = isChecked ? 'block' : 'none';
}

document.getElementById('share-recycle').addEventListener('change', toggleRecycleInfo);

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
        params: {
            'read only': document.getElementById('share-readonly').checked ? 'yes' : 'no',
            'guest ok': document.getElementById('share-guest').checked ? 'yes' : 'no',
            'browseable': document.getElementById('share-browseable').checked ? 'yes' : 'no',
            'create mask': document.getElementById('share-create-mask').value,
            'directory mask': document.getElementById('share-dir-mask').value,
            'force create mode': document.getElementById('share-create-mask').value,
            'force directory mode': document.getElementById('share-dir-mask').value,
            'inherit acls': document.getElementById('share-inherit-acls').checked ? 'yes' : 'no',
            'guest only': document.getElementById('share-guest-only').checked ? 'yes' : 'no'
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
                    <button class="btn-action">Пароль</button>
                    <button class="btn-action" style="color: #dc2626;">Удалить</button>
                </td>
            </tr>`;
        });
    } catch (e) { console.error(e); }
}

async function logout() {
    await fetch('/api/logout');
    window.location.href = '/login.html';
}

const style = document.createElement('style');
style.innerHTML = `
    .btn-action {
        padding: 4px 12px;
        background: #f8fafc;
        border: 1px solid #e2e8f0;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8rem;
        font-weight: 600;
    }
    .btn-action:hover { background: #f1f5f9; }
`;
document.head.appendChild(style);

setInterval(updateStatus, 3000);
updateStatus();
