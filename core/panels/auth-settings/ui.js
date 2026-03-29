import { html, useState, useEffect } from '/core/vendor/preact-htm.js';

// Universal clipboard copy with Telegram WebApp + execCommand fallbacks
function copyTextToClipboard(text) {
  return new Promise(function(resolve, reject) {
    // Step 1: navigator.clipboard (modern browsers)
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(resolve).catch(tryTelegram);
    } else {
      tryTelegram();
    }

    function tryTelegram() {
      // Step 2: Telegram WebApp writeTextToClipboard (Bot API 6.9+)
      if (window.Telegram && window.Telegram.WebApp &&
          typeof window.Telegram.WebApp.writeTextToClipboard === 'function') {
        window.Telegram.WebApp.writeTextToClipboard(text, function(result) {
          if (result && result.isSuccess) { resolve(); } else { tryExecCommand(); }
        });
      } else {
        tryExecCommand();
      }
    }

    function tryExecCommand() {
      // Step 3: execCommand textarea fallback
      try {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.style.cssText = 'position:fixed;left:-9999px;top:-9999px;opacity:0';
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        var ok = document.execCommand('copy');
        document.body.removeChild(ta);
        if (ok) { resolve(); return; }
      } catch (e) { /* fall through */ }
      // Step 4: Show selectable manual-copy UI, then reject
      showManualCopyFallback(text);
      reject(new Error('Copy failed'));
    }
  });
}

function showManualCopyFallback(text) {
  var overlay = document.createElement('div');
  overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.72);z-index:9999;display:flex;align-items:center;justify-content:center;padding:16px;box-sizing:border-box';

  var box = document.createElement('div');
  box.style.cssText = 'background:#1e1e2e;border:1px solid rgba(255,255,255,0.15);border-radius:10px;padding:16px;max-width:400px;width:100%;box-sizing:border-box';

  var label = document.createElement('div');
  label.style.cssText = 'font-size:13px;color:#f87171;margin-bottom:8px;font-weight:600';
  label.textContent = 'Auto-copy failed. Select and copy manually:';

  var input = document.createElement('input');
  input.type = 'text';
  input.value = text;
  input.readOnly = true;
  input.style.cssText = 'width:100%;box-sizing:border-box;padding:8px 10px;background:#2a2a3e;border:1px solid rgba(255,255,255,0.2);border-radius:6px;color:#e2e8f0;font-family:monospace;font-size:12px;margin-bottom:10px;outline:none';

  var closeBtn = document.createElement('button');
  closeBtn.textContent = 'Close';
  closeBtn.style.cssText = 'padding:6px 14px;background:rgba(255,255,255,0.08);border:1px solid rgba(255,255,255,0.15);border-radius:6px;color:#e2e8f0;cursor:pointer;font-size:12px';
  closeBtn.onclick = function() { document.body.removeChild(overlay); };

  box.appendChild(label);
  box.appendChild(input);
  box.appendChild(closeBtn);
  overlay.appendChild(box);
  document.body.appendChild(overlay);

  setTimeout(function() { input.focus(); input.select(); }, 80);
}

export default function AuthSettingsPanel({ data, error, connected, lastUpdate, api, cls }) {
  const [users, setUsers] = useState([]);
  const [keys, setKeys] = useState([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(null);

  // Add user form
  const [showAddUser, setShowAddUser] = useState(false);
  const [newUser, setNewUser] = useState({ id: '', name: '', email: '', role: 'user', provider: 'telegram', provider_id: '' });

  // Create key form
  const [showCreateKey, setShowCreateKey] = useState(false);
  const [newKey, setNewKey] = useState({ name: '', role: 'viewer', scopes: '' });
  const [createdKey, setCreatedKey] = useState(null);

  // Magic link
  const [mlUserId, setMlUserId] = useState('');
  const [mlUrl, setMlUrl] = useState(null);
  const [mlError, setMlError] = useState(null);

  // Status messages
  const [statusMsg, setStatusMsg] = useState(null);

  const showStatus = (msg, isError) => {
    setStatusMsg({ text: msg, error: isError });
    setTimeout(() => setStatusMsg(null), 4000);
  };

  const load = async () => {
    try {
      const [usersRes, keysRes] = await Promise.all([
        api.fetch('/api/auth/users'),
        api.fetch('/api/auth/keys')
      ]);
      if (usersRes.ok) {
        const d = await usersRes.json();
        setUsers(d.users || []);
      }
      if (keysRes.ok) {
        const d = await keysRes.json();
        setKeys(d.keys || []);
      }
      setFetchError(null);
    } catch (e) {
      setFetchError('Failed to load: ' + e.message);
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  // ── Users ──
  const addUser = async () => {
    const body = {
      id: newUser.id.trim(),
      name: newUser.name.trim(),
      email: newUser.email.trim(),
      role: newUser.role
    };
    if (newUser.provider_id.trim()) {
      body.identities = [{ provider: newUser.provider, provider_id: newUser.provider_id.trim() }];
    }
    try {
      const res = await api.fetch('/api/auth/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      const d = await res.json();
      if (d.ok) {
        showStatus('User added', false);
        setShowAddUser(false);
        setNewUser({ id: '', name: '', email: '', role: 'user', provider: 'telegram', provider_id: '' });
        load();
      } else {
        showStatus(d.error || 'Failed', true);
      }
    } catch (e) {
      showStatus('Error: ' + e.message, true);
    }
  };

  const deleteUser = async (userId) => {
    if (!confirm(`Delete user "${userId}"?`)) return;
    try {
      const res = await api.fetch(`/api/auth/users?id=${encodeURIComponent(userId)}`, { method: 'DELETE' });
      const d = await res.json();
      if (d.ok) {
        showStatus('User deleted', false);
        load();
      } else {
        showStatus(d.error || 'Failed', true);
      }
    } catch (e) {
      showStatus('Error: ' + e.message, true);
    }
  };

  // ── Keys ──
  const createKey = async () => {
    const body = {
      name: newKey.name.trim(),
      role: newKey.role,
      scopes: newKey.scopes.trim() ? newKey.scopes.split(',').map(s => s.trim()).filter(Boolean) : undefined
    };
    try {
      const res = await api.fetch('/api/auth/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      const d = await res.json();
      if (d.ok) {
        setCreatedKey(d.key);
        setShowCreateKey(false);
        setNewKey({ name: '', role: 'viewer', scopes: '' });
        load();
      } else {
        showStatus(d.error || 'Failed', true);
      }
    } catch (e) {
      showStatus('Error: ' + e.message, true);
    }
  };

  const revokeKey = async (keyId) => {
    if (!confirm(`Revoke API key "${keyId}"?`)) return;
    try {
      const res = await api.fetch(`/api/auth/keys?id=${encodeURIComponent(keyId)}`, { method: 'DELETE' });
      const d = await res.json();
      if (d.ok) {
        showStatus('Key revoked', false);
        load();
      } else {
        showStatus(d.error || 'Failed', true);
      }
    } catch (e) {
      showStatus('Error: ' + e.message, true);
    }
  };

  // ── Magic Link ──
  const generateMagicLink = async () => {
    setMlUrl(null);
    setMlError(null);
    if (!mlUserId) { setMlError('Select a user'); return; }
    try {
      const res = await api.fetch('/api/auth/magic-link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: mlUserId, expires_minutes: 15 })
      });
      const d = await res.json();
      if (d.ok) {
        setMlUrl(d.url);
      } else {
        setMlError(d.error || 'Failed');
      }
    } catch (e) {
      setMlError('Error: ' + e.message);
    }
  };

  // ── Styles ──
  const S = {
    section: 'margin-bottom:20px',
    heading: 'font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:1px;margin-bottom:10px;display:flex;align-items:center;justify-content:space-between',
    table: 'width:100%;border-collapse:collapse;font-size:12px',
    th: 'text-align:left;padding:6px 8px;border-bottom:1px solid rgba(255,255,255,0.06);color:var(--text-dim);font-weight:500;font-size:11px',
    td: 'padding:6px 8px;border-bottom:1px solid rgba(255,255,255,0.03)',
    btn: 'padding:4px 10px;border:1px solid rgba(255,255,255,0.1);border-radius:6px;background:rgba(255,255,255,0.04);color:var(--text);cursor:pointer;font-size:11px',
    btnDanger: 'padding:4px 10px;border:1px solid rgba(239,68,68,0.3);border-radius:6px;background:rgba(239,68,68,0.08);color:#f87171;cursor:pointer;font-size:11px',
    btnPrimary: 'padding:4px 10px;border:1px solid rgba(201,168,76,0.3);border-radius:6px;background:rgba(201,168,76,0.1);color:#c9a84c;cursor:pointer;font-size:11px',
    input: 'padding:5px 8px;border:1px solid rgba(255,255,255,0.1);border-radius:6px;background:rgba(255,255,255,0.04);color:var(--text);font-size:12px;width:100%',
    select: 'padding:5px 8px;border:1px solid rgba(255,255,255,0.1);border-radius:6px;background:rgba(255,255,255,0.04);color:var(--text);font-size:12px',
    badge: (color) => `display:inline-block;padding:1px 6px;border-radius:4px;font-size:10px;font-weight:600;background:rgba(${color},0.12);color:rgba(${color},1)`,
    formRow: 'display:flex;gap:8px;margin-bottom:8px;align-items:center',
    modal: 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.7);display:flex;align-items:center;justify-content:center;z-index:1000',
    modalBox: 'background:#12121a;border:1px solid rgba(255,255,255,0.1);border-radius:12px;padding:24px;max-width:500px;width:90%',
    codeBox: 'padding:10px;background:rgba(0,0,0,0.4);border:1px solid rgba(255,255,255,0.08);border-radius:8px;font-family:monospace;font-size:12px;word-break:break-all;color:#c9a84c;user-select:all;margin:12px 0'
  };

  const roleBadge = (role) => {
    const colors = { admin: '239,68,68', user: '59,130,246', viewer: '107,114,128' };
    return html`<span style=${S.badge(colors[role] || '107,114,128')}>${role}</span>`;
  };

  if (loading) return html`<div style="padding:8px;color:var(--text-dim)">Loading…</div>`;
  if (fetchError) return html`<div style="padding:8px;color:var(--red)">${fetchError}</div>`;

  return html`
    <div class=${cls('wrap')}>
      ${statusMsg && html`
        <div style="margin-bottom:12px;padding:8px 12px;border-radius:8px;font-size:12px;
          background:${statusMsg.error ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)'};
          border:1px solid ${statusMsg.error ? 'rgba(239,68,68,0.2)' : 'rgba(34,197,94,0.2)'};
          color:${statusMsg.error ? '#f87171' : '#22c55e'}">
          ${statusMsg.text}
        </div>
      `}

      <!-- Users Section -->
      <div style=${S.section}>
        <div style=${S.heading}>
          <span>Users (${users.length})</span>
          <button style=${S.btnPrimary} onClick=${() => setShowAddUser(!showAddUser)}>
            ${showAddUser ? 'Cancel' : '+ Add User'}
          </button>
        </div>

        ${showAddUser && html`
          <div style="padding:12px;background:rgba(255,255,255,0.02);border:1px solid rgba(255,255,255,0.06);border-radius:8px;margin-bottom:12px">
            <div style=${S.formRow}>
              <input style=${S.input} placeholder="User ID" value=${newUser.id}
                onInput=${e => setNewUser({...newUser, id: e.target.value})} />
              <input style=${S.input} placeholder="Name" value=${newUser.name}
                onInput=${e => setNewUser({...newUser, name: e.target.value})} />
            </div>
            <div style=${S.formRow}>
              <input style=${S.input} placeholder="Email" value=${newUser.email}
                onInput=${e => setNewUser({...newUser, email: e.target.value})} />
              <select style=${S.select} value=${newUser.role}
                onChange=${e => setNewUser({...newUser, role: e.target.value})}>
                <option value="admin">admin</option>
                <option value="user">user</option>
                <option value="viewer">viewer</option>
              </select>
            </div>
            <div style=${S.formRow}>
              <select style=${S.select} value=${newUser.provider}
                onChange=${e => setNewUser({...newUser, provider: e.target.value})}>
                <option value="telegram">telegram</option>
              </select>
              <input style=${S.input} placeholder="Provider ID (e.g. Telegram user ID)" value=${newUser.provider_id}
                onInput=${e => setNewUser({...newUser, provider_id: e.target.value})} />
              <button style=${S.btnPrimary} onClick=${addUser}>Add</button>
            </div>
          </div>
        `}

        <table style=${S.table}>
          <thead>
            <tr>
              <th style=${S.th}>ID</th>
              <th style=${S.th}>Name</th>
              <th style=${S.th}>Email</th>
              <th style=${S.th}>Role</th>
              <th style=${S.th}>Identities</th>
              <th style=${S.th}></th>
            </tr>
          </thead>
          <tbody>
            ${users.map(u => html`
              <tr>
                <td style=${S.td}><span style="font-family:monospace;font-size:12px">${u.id}</span></td>
                <td style=${S.td}>${u.name}</td>
                <td style=${S.td}><span style="color:var(--text-dim);font-size:11px">${u.email || '—'}</span></td>
                <td style=${S.td}>${roleBadge(u.role)}</td>
                <td style=${S.td}><span style="color:var(--text-dim);font-size:11px">${(u.identities || []).length} linked</span></td>
                <td style=${S.td + ';text-align:right'}>
                  <button style=${S.btnDanger} onClick=${() => deleteUser(u.id)}>Delete</button>
                </td>
              </tr>
            `)}
            ${users.length === 0 && html`
              <tr><td style=${S.td} colspan="6"><span style="color:var(--text-dim)">No users configured</span></td></tr>
            `}
          </tbody>
        </table>
      </div>

      <!-- API Keys Section -->
      <div style=${S.section}>
        <div style=${S.heading}>
          <span>API Keys (${keys.length})</span>
          <button style=${S.btnPrimary} onClick=${() => setShowCreateKey(!showCreateKey)}>
            ${showCreateKey ? 'Cancel' : '+ Create Key'}
          </button>
        </div>

        ${showCreateKey && html`
          <div style="padding:12px;background:rgba(255,255,255,0.02);border:1px solid rgba(255,255,255,0.06);border-radius:8px;margin-bottom:12px">
            <div style=${S.formRow}>
              <input style=${S.input} placeholder="Key name/ID" value=${newKey.name}
                onInput=${e => setNewKey({...newKey, name: e.target.value})} />
              <select style=${S.select} value=${newKey.role}
                onChange=${e => setNewKey({...newKey, role: e.target.value})}>
                <option value="admin">admin</option>
                <option value="user">user</option>
                <option value="viewer">viewer</option>
              </select>
            </div>
            <div style=${S.formRow}>
              <input style=${S.input} placeholder="Scopes (comma-separated, e.g. GET /api/health, GET /api/status)" value=${newKey.scopes}
                onInput=${e => setNewKey({...newKey, scopes: e.target.value})} />
              <button style=${S.btnPrimary} onClick=${createKey}>Create</button>
            </div>
          </div>
        `}

        ${createdKey && html`
          <div style=${S.modal} onClick=${(e) => { if(e.target === e.currentTarget) setCreatedKey(null); }}>
            <div style=${S.modalBox}>
              <div style="font-size:14px;font-weight:600;margin-bottom:8px">🔑 API Key Created</div>
              <div style="font-size:12px;color:var(--text-dim);margin-bottom:8px">
                Copy this key now — it will <strong>never be shown again</strong>.
              </div>
              <div style=${S.codeBox}>${createdKey}</div>
              <div style="display:flex;gap:8px;justify-content:flex-end">
                <button style=${S.btnPrimary} onClick=${() => {
                  copyTextToClipboard(createdKey)
                    .then(() => showStatus('Copied!', false))
                    .catch(() => showStatus('Auto-copy failed — select the text above manually', true));
                }}>
                  Copy
                </button>
                <button style=${S.btn} onClick=${() => setCreatedKey(null)}>Close</button>
              </div>
            </div>
          </div>
        `}

        <table style=${S.table}>
          <thead>
            <tr>
              <th style=${S.th}>ID</th>
              <th style=${S.th}>Role</th>
              <th style=${S.th}>Scopes</th>
              <th style=${S.th}>Created</th>
              <th style=${S.th}></th>
            </tr>
          </thead>
          <tbody>
            ${keys.map(k => html`
              <tr>
                <td style=${S.td}><span style="font-family:monospace;font-size:12px">${k.id}</span></td>
                <td style=${S.td}>${roleBadge(k.role)}</td>
                <td style=${S.td}>
                  <span style="color:var(--text-dim);font-size:11px">
                    ${(k.scopes && k.scopes.length) ? k.scopes.join(', ') : '*'}
                  </span>
                </td>
                <td style=${S.td}><span style="color:var(--text-dim);font-size:11px">${k.created_at ? new Date(k.created_at).toLocaleDateString() : '—'}</span></td>
                <td style=${S.td + ';text-align:right'}>
                  <button style=${S.btnDanger} onClick=${() => revokeKey(k.id)}>Revoke</button>
                </td>
              </tr>
            `)}
            ${keys.length === 0 && html`
              <tr><td style=${S.td} colspan="5"><span style="color:var(--text-dim)">No API keys configured</span></td></tr>
            `}
          </tbody>
        </table>
      </div>

      <!-- Magic Link Section -->
      <div style=${S.section}>
        <div style=${S.heading}>
          <span>Magic Link</span>
        </div>
        <div style="padding:12px;background:rgba(255,255,255,0.02);border:1px solid rgba(255,255,255,0.06);border-radius:8px">
          <div style=${S.formRow}>
            <select style=${S.select} value=${mlUserId}
              onChange=${e => { setMlUserId(e.target.value); setMlUrl(null); setMlError(null); }}>
              <option value="">Select user…</option>
              ${users.map(u => html`<option value=${u.id}>${u.name} (${u.id})</option>`)}
            </select>
            <button style=${S.btnPrimary} onClick=${generateMagicLink}>Generate Link</button>
          </div>
          ${mlUrl && html`
            <div style=${S.codeBox}>${mlUrl}</div>
            <button style=${S.btnPrimary} onClick=${() => {
              copyTextToClipboard(mlUrl)
                .then(() => showStatus('Copied!', false))
                .catch(() => showStatus('Auto-copy failed — select the link above manually', true));
            }}>
              Copy Link
            </button>
            <span style="margin-left:8px;font-size:11px;color:var(--text-dim)">Expires in 15 minutes</span>
          `}
          ${mlError && html`<div style="margin-top:8px;font-size:12px;color:#f87171">${mlError}</div>`}
        </div>
      </div>
    </div>
  `;
}
