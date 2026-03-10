import { html, useState, useEffect } from '/core/vendor/preact-htm.js';

export default function VerifyStatusPanel({ data, error, connected, lastUpdate, api, cls }) {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(null);

  const load = async () => {
    try {
      const r = await api.fetch('/api/verify-status');
      if (!r.ok) { setFetchError('Failed to load verify status'); setLoading(false); return; }
      const d = await r.json();
      setStatus(d);
      setFetchError(null);
    } catch (e) {
      setFetchError('Error: ' + e.message);
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  if (loading) return html`<div class=${cls('loading')} style="padding:8px;color:var(--text-dim)">Loading…</div>`;
  if (fetchError) return html`<div class=${cls('error')} style="padding:8px;color:var(--red)">${fetchError}</div>`;

  const latest = status && status.latest;
  const history = (status && status.history) || [];
  const healthy = status ? status.healthy : true;

  // Parse latest
  let latestStatus = null, latestTs = null, passed = 0, failed = 0, skipped = 0, failedChecks = [];
  if (latest) {
    latestStatus = latest.status;
    latestTs = latest.timestamp;
    passed = latest.passed || 0;
    failed = latest.failed || 0;
    skipped = latest.skipped || 0;
    failedChecks = (latest.checks || []).filter(c => c.status === 'fail');
  }

  const fmtTs = (ts) => {
    if (!ts) return '—';
    try { return new Date(ts).toLocaleString(); } catch (e) { return ts; }
  };

  // Last 10 history dots
  const dots = history.slice(-10);

  return html`
    <div class=${cls('wrap')}>
      ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}

      <!-- Header -->
      <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px">
        <span style="font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:1px">
          Verify Status
        </span>
        ${latest
          ? html`<span style="font-size:18px">${healthy ? '✅' : '❌'}</span>`
          : html`<span style="font-size:12px;color:var(--text-dim)">No runs yet</span>`
        }
      </div>

      <!-- Latest result -->
      ${latest
        ? html`
          <div style="padding:10px 12px;border-radius:8px;background:${healthy ? 'rgba(34,197,94,0.06)' : 'rgba(239,68,68,0.06)'};border:1px solid ${healthy ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.2)'};margin-bottom:10px">
            <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:4px">
              <span style="font-size:13px;font-weight:600;color:${healthy ? 'var(--green)' : 'var(--red)'}">
                ${healthy ? '✓ All checks passed' : `✗ ${failed} check${failed !== 1 ? 's' : ''} failed`}
              </span>
            </div>
            <div style="font-size:11px;color:var(--text-dim);margin-bottom:6px">${fmtTs(latestTs)}</div>
            <div style="display:flex;gap:12px;font-size:11px">
              <span style="color:var(--green)">✓ ${passed} passed</span>
              ${failed > 0 && html`<span style="color:var(--red)">✗ ${failed} failed</span>`}
              ${skipped > 0 && html`<span style="color:var(--text-dim)">○ ${skipped} skipped</span>`}
            </div>
          </div>

          <!-- Failed checks -->
          ${failedChecks.length > 0 && html`
            <div style="margin-bottom:10px">
              <div style="font-size:11px;font-weight:600;color:var(--red);margin-bottom:4px;text-transform:uppercase;letter-spacing:0.5px">Failed</div>
              <div style="display:flex;flex-direction:column;gap:4px">
                ${failedChecks.map(c => html`
                  <div style="padding:6px 10px;border-radius:6px;background:rgba(239,68,68,0.05);border:1px solid rgba(239,68,68,0.15)">
                    <div style="font-size:12px;font-weight:600;font-family:'JetBrains Mono',monospace;color:var(--red)">${c.name}</div>
                    ${c.detail && html`<div style="font-size:11px;color:var(--text-dim);margin-top:2px">${c.detail}</div>`}
                    ${c.hint && html`<div style="font-size:11px;color:var(--accent);margin-top:2px">💡 ${c.hint}</div>`}
                  </div>
                `)}
              </div>
            </div>
          `}
        `
        : html`<div style="font-size:12px;color:var(--text-dim);padding:4px 0">No verify runs found. Run <code style="font-family:monospace;background:rgba(255,255,255,0.06);padding:1px 4px;border-radius:3px">./vel verify --json</code> to start.</div>`
      }

      <!-- History dots -->
      ${dots.length > 0 && html`
        <div>
          <div style="font-size:10px;font-weight:600;color:var(--text-dim);text-transform:uppercase;letter-spacing:0.5px;margin-bottom:6px">Last ${dots.length} runs</div>
          <div style="display:flex;gap:5px;flex-wrap:wrap">
            ${dots.map((run, i) => {
              const ok = run.status === 'ok';
              const ts = run.timestamp ? new Date(run.timestamp).toLocaleString() : '';
              return html`
                <div
                  title="${ts ? ts + ' — ' : ''}${ok ? 'passed' : 'failed'}"
                  style="width:10px;height:10px;border-radius:50%;background:${ok ? 'var(--green)' : 'var(--red)'};cursor:default;flex-shrink:0"
                ></div>
              `;
            })}
          </div>
        </div>
      `}
    </div>
  `;
}
