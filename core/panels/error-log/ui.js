import { html, useState, useEffect } from '/core/vendor/preact-htm.js';

export default function ErrorLogPanel({ data, error, connected, lastUpdate, api, cls }) {
  const [errors, setErrors] = useState(null);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(null);
  const [expanded, setExpanded] = useState({});

  const load = async () => {
    try {
      const r = await api.fetch('/api/errors?limit=50');
      if (!r.ok) { setFetchError('Failed to load error log'); setLoading(false); return; }
      const d = await r.json();
      setErrors(d.errors || []);
      setFetchError(null);
    } catch (e) {
      setFetchError('Error: ' + e.message);
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  if (loading) return html`<div class=${cls('loading')} style="padding:8px;color:var(--text-dim)">Loading…</div>`;
  if (fetchError) return html`<div class=${cls('error')} style="padding:8px;color:var(--red)">${fetchError}</div>`;

  const now = Date.now();
  const hour = 60 * 60 * 1000;
  const day = 24 * hour;

  const errorsInHour = (errors || []).filter(e => {
    try { return (now - new Date(e.time).getTime()) < hour; } catch { return false; }
  }).length;

  const errorsIn24h = (errors || []).filter(e => {
    try { return (now - new Date(e.time).getTime()) < day; } catch { return false; }
  }).length;

  const fmtTs = (ts) => {
    if (!ts) return '—';
    try {
      const d = new Date(ts);
      return d.toLocaleString();
    } catch (e) { return ts; }
  };

  const statusColor = (code) => {
    if (!code) return 'var(--text-dim)';
    if (code >= 500) return 'var(--red)';
    if (code >= 400) return '#f97316'; // orange
    return 'var(--text-dim)';
  };

  const toggleExpand = (i) => {
    setExpanded(prev => ({ ...prev, [i]: !prev[i] }));
  };

  return html`
    <div class=${cls('wrap')}>
      ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}

      <!-- Header -->
      <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px">
        <span style="font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:1px">
          Error Log
          ${errorsIn24h > 0
            ? html`<span style="margin-left:8px;font-size:10px;font-weight:700;padding:2px 7px;border-radius:10px;background:rgba(239,68,68,0.15);color:var(--red)">${errorsIn24h}</span>`
            : html`<span style="margin-left:8px;font-size:10px;font-weight:500;padding:2px 7px;border-radius:10px;background:rgba(34,197,94,0.1);color:var(--green)">0</span>`
          }
        </span>
        <span style="font-size:10px;color:var(--text-dim)">last 50</span>
      </div>

      <!-- Summary -->
      ${errorsIn24h > 0
        ? html`
          <div style="font-size:12px;color:var(--text-dim);margin-bottom:12px;padding:6px 10px;border-radius:6px;background:rgba(239,68,68,0.04);border:1px solid rgba(239,68,68,0.12)">
            <span style="color:var(--red);font-weight:600">${errorsInHour}</span> error${errorsInHour !== 1 ? 's' : ''} in last hour,
            <span style="color:var(--red);font-weight:600">${errorsIn24h}</span> in last 24h
          </div>
        `
        : html`
          <div style="text-align:center;padding:20px 0;color:var(--text-dim)">
            <div style="font-size:24px;margin-bottom:8px">✅</div>
            <div style="font-size:13px">No errors in the last 24h</div>
          </div>
        `
      }

      <!-- Error list -->
      ${errors && errors.length > 0 && html`
        <div style="display:flex;flex-direction:column;gap:4px">
          ${errors.map((e, i) => {
            const code = e.status;
            const color = statusColor(code);
            const hasStack = e.stack && e.stack.length > 0;
            const isExpanded = !!expanded[i];

            return html`
              <div style="border-radius:7px;background:rgba(255,255,255,0.03);border:1px solid rgba(255,255,255,0.07);overflow:hidden">
                <div style="display:flex;align-items:flex-start;gap:10px;padding:8px 10px">
                  <!-- Status code badge -->
                  <span style="flex-shrink:0;font-size:11px;font-weight:700;font-family:'JetBrains Mono',monospace;color:${color};padding:1px 5px;border-radius:4px;background:${code >= 500 ? 'rgba(239,68,68,0.1)' : code >= 400 ? 'rgba(249,115,22,0.1)' : 'rgba(255,255,255,0.05)'}">
                    ${code || '???'}
                  </span>

                  <!-- Main content -->
                  <div style="flex:1;min-width:0">
                    <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-bottom:2px">
                      ${e.method && html`<span style="font-size:10px;font-weight:700;color:var(--accent);font-family:'JetBrains Mono',monospace">${e.method}</span>`}
                      ${e.path && html`<span style="font-size:11px;font-family:'JetBrains Mono',monospace;color:var(--text-dim);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:300px">${e.path}</span>`}
                    </div>
                    ${e.message && html`<div style="font-size:12px;color:${color};margin-bottom:2px;word-break:break-word">${e.message}</div>`}
                    <div style="font-size:10px;color:var(--text-dim);opacity:0.7">${fmtTs(e.time)}</div>
                  </div>

                  <!-- Expand stack trace toggle -->
                  ${hasStack && html`
                    <button
                      onClick=${() => toggleExpand(i)}
                      style="flex-shrink:0;font-size:10px;padding:2px 7px;background:rgba(255,255,255,0.06);color:var(--text-dim);border:none;border-radius:4px;cursor:pointer;margin-top:1px"
                    >${isExpanded ? '▲ hide' : '▼ trace'}</button>
                  `}
                </div>

                <!-- Stack trace -->
                ${hasStack && isExpanded && html`
                  <div style="border-top:1px solid rgba(255,255,255,0.06);padding:8px 10px;background:rgba(0,0,0,0.2)">
                    <pre style="font-size:10px;font-family:'JetBrains Mono',monospace;color:var(--text-dim);margin:0;white-space:pre-wrap;word-break:break-all;line-height:1.5">${e.stack}</pre>
                  </div>
                `}
              </div>
            `;
          })}
        </div>
      `}
    </div>
  `;
}
