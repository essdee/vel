import { html, useState, useEffect } from '/core/vendor/preact-htm.js';

export default function TestPanel({ data, error, connected, lastUpdate, api, config, cls }) {
  if (error) return html`<div class=${cls('error')}>${error.error}</div>`;
  if (!data) return html`<div class=${cls('loading')}>Loading...</div>`;

  return html`
    <div class=${cls('wrap')}>
      ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}
      <div class=${cls('message')}>${data.message}</div>
      <div class=${cls('ts')} style="color: var(--text-dim); font-size: 11px; margin-top: 4px;">
        ${new Date(data.ts).toLocaleTimeString()}
      </div>
    </div>
  `;
}
