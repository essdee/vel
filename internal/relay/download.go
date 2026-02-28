package relay

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"vel/internal/auth"
)

var userNames = map[int64]string{
	85720317:   "Karthi",
	2031224178: "Nithin",
}

const bridgeHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>OpenClaw Browser</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #0e0e12; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif; min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; }
  .container { max-width: 480px; width: 100%; padding: 40px 24px; text-align: center; }
  .logo { font-size: 48px; margin-bottom: 8px; }
  h1 { font-size: 22px; font-weight: 600; color: #fff; margin-bottom: 32px; }
  .status-area { margin-bottom: 32px; }
  .status-icon { font-size: 40px; margin-bottom: 12px; }
  .status-text { font-size: 18px; font-weight: 500; line-height: 1.4; }
  .status-sub { font-size: 14px; color: #888; margin-top: 8px; }
  .tab-name { color: #0af; font-size: 14px; margin-top: 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 400px; margin-left: auto; margin-right: auto; }

  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
  @keyframes glow { 0%, 100% { box-shadow: 0 0 20px rgba(34,197,94,0.2); } 50% { box-shadow: 0 0 40px rgba(34,197,94,0.4); } }
  @keyframes agent-glow { 0%, 100% { box-shadow: 0 0 20px rgba(0,170,255,0.2); } 50% { box-shadow: 0 0 40px rgba(0,170,255,0.5); } }

  .indicator { width: 12px; height: 12px; border-radius: 50%; display: inline-block; margin-right: 8px; vertical-align: middle; }
  .ind-connecting { background: #f59e0b; animation: pulse 1.5s ease-in-out infinite; }
  .ind-connected { background: #22c55e; animation: glow 3s ease-in-out infinite; }
  .ind-active { background: #0af; animation: agent-glow 1.5s ease-in-out infinite; }
  .ind-error { background: #ef4444; }

  .card { background: #16161c; border: 1px solid #2a2a35; border-radius: 12px; padding: 20px; margin-bottom: 16px; text-align: left; }
  .card.connected { animation: glow 3s ease-in-out infinite; }
  .card.active { animation: agent-glow 1.5s ease-in-out infinite; }

  details { background: #16161c; border: 1px solid #2a2a35; border-radius: 8px; padding: 12px 16px; margin-bottom: 10px; text-align: left; }
  summary { cursor: pointer; color: #aaa; font-size: 14px; user-select: none; }
  summary:hover { color: #ddd; }
  details p, details ul { color: #888; font-size: 13px; line-height: 1.6; margin-top: 8px; }
  details ul { padding-left: 20px; }
  details li { margin-bottom: 4px; }

  .tips { display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; margin-bottom: 24px; }
  .tip { background: #1a1a24; border: 1px solid #2a2a35; border-radius: 6px; padding: 6px 12px; font-size: 12px; color: #888; }

  .footer { margin-top: 24px; padding-top: 16px; border-top: 1px solid #1a1a24; }
  .footer-name { color: #666; font-size: 13px; margin-bottom: 12px; }
  .btn-disconnect { background: transparent; border: 1px solid #444; color: #aaa; padding: 8px 20px; border-radius: 6px; cursor: pointer; font-size: 13px; transition: all 0.2s; }
  .btn-disconnect:hover { border-color: #ef4444; color: #ef4444; }

  .error-text { color: #ef4444; font-size: 14px; margin-top: 8px; }
  .hidden { display: none; }
</style>
</head>
<body>
<div class="container">
  <div class="logo">🦞</div>
  <h1>OpenClaw Browser</h1>

  <div class="status-area card" id="statusCard">
    <div class="status-icon" id="statusIcon">🔄</div>
    <div class="status-text" id="statusText">Starting up...</div>
    <div class="status-sub" id="statusSub"></div>
    <div class="tab-name hidden" id="tabName"></div>
    <div class="error-text hidden" id="errorText"></div>
  </div>

  <div class="tips" id="tips">
    <span class="tip">💡 Keep this tab open</span>
    <span class="tip">🖥️ You can use other apps while waiting</span>
  </div>

  <details>
    <summary>💬 What's happening?</summary>
    <p>This browser is connected to your OpenClaw AI. When you ask it to do something in a browser, it will use this one. You can watch it work in real-time in the other tabs.</p>
  </details>

  <details>
    <summary>🔧 Having trouble?</summary>
    <ul>
      <li>Make sure you keep this tab open</li>
      <li>If it says "Disconnected", close everything and run the launcher again</li>
      <li>Check your internet connection</li>
      <li>Try closing other Chrome windows first</li>
    </ul>
  </details>

  <div class="footer">
    <div class="footer-name" id="footerName">Connected as __USER_NAME__</div>
    <button class="btn-disconnect" id="btnDisconnect" onclick="disconnect()">Disconnect</button>
  </div>
</div>

<script>
(function() {
  const RELAY_URL = '__RELAY_URL__';
  const RELAY_TOKEN = '__RELAY_TOKEN__';
  const USER_NAME = '__USER_NAME__';

  const $ = id => document.getElementById(id);
  let relayWS = null;
  let localWSMap = {};
  let detectInterval = null;
  let state = 'starting';

  function setStatus(icon, text, sub, cls) {
    $('statusIcon').textContent = icon;
    $('statusText').textContent = text;
    $('statusSub').textContent = sub || '';
    const card = $('statusCard');
    card.className = 'status-area card' + (cls ? ' ' + cls : '');
    $('tabName').classList.add('hidden');
    $('errorText').classList.add('hidden');
  }

  function setError(msg) {
    $('errorText').textContent = msg;
    $('errorText').classList.remove('hidden');
  }

  function showTab(name) {
    const el = $('tabName');
    el.textContent = '🔍 ' + name;
    el.classList.remove('hidden');
  }

  // Filter targets — exclude this bridge page and chrome internals
  function filterTargets(targets) {
    return targets.filter(t =>
      t.type === 'page' &&
      !t.url.startsWith('chrome://') &&
      !t.url.startsWith('chrome-extension://') &&
      !t.url.includes('openclaw-browser-') &&
      t.url !== ''
    );
  }

  async function fetchTargets() {
    try {
      const resp = await fetch('http://localhost:9222/json');
      if (!resp.ok) return null;
      return await resp.json();
    } catch(e) { return null; }
  }

  function connectLocalTarget(target) {
    if (localWSMap[target.id]) return;
    const wsUrl = target.webSocketDebuggerUrl;
    if (!wsUrl) return;
    try {
      const ws = new WebSocket(wsUrl);
      ws.onmessage = (e) => {
        if (relayWS && relayWS.readyState === 1) {
          relayWS.send(JSON.stringify({ type: 'cdp', targetId: target.id, data: JSON.parse(e.data) }));
        }
      };
      ws.onclose = () => { delete localWSMap[target.id]; };
      ws.onerror = () => { delete localWSMap[target.id]; };
      localWSMap[target.id] = ws;
    } catch(e) {}
  }

  function disconnectLocalTarget(id) {
    if (localWSMap[id]) { try { localWSMap[id].close(); } catch(e){} delete localWSMap[id]; }
  }

  async function refreshTargets() {
    const targets = await fetchTargets();
    if (!targets) return;
    const filtered = filterTargets(targets);
    // Send to relay
    if (relayWS && relayWS.readyState === 1) {
      relayWS.send(JSON.stringify({ type: 'targets', data: filtered }));
    }
    // Connect new, disconnect removed
    const ids = new Set(filtered.map(t => t.id));
    filtered.forEach(t => connectLocalTarget(t));
    Object.keys(localWSMap).forEach(id => { if (!ids.has(id)) disconnectLocalTarget(id); });
  }

  async function start() {
    setStatus('🔄', 'Starting up...', 'Checking browser...');

    // Wait for local CDP to be ready (Chrome may still be starting)
    let targets = null;
    for (let i = 0; i < 20; i++) {
      targets = await fetchTargets();
      if (targets) break;
      await new Promise(r => setTimeout(r, 1000));
    }
    if (!targets) {
      setStatus('❌', 'Could not connect to browser', 'Please close and relaunch');
      setError('Local browser not responding on port 9222');
      return;
    }

    setStatus('🔗', 'Connecting to OpenClaw...', '', '');
    state = 'connecting';

    // Connect to relay
    relayWS = new WebSocket(RELAY_URL + '?token=' + RELAY_TOKEN);

    relayWS.onopen = () => {
      state = 'connected';
      setStatus('✅', 'Connected! Your AI can now use this browser.', 'Waiting for your AI to start working...', 'connected');
      refreshTargets();
      detectInterval = setInterval(refreshTargets, 5000);
    };

    relayWS.onmessage = (e) => {
      const env = JSON.parse(e.data);
      switch (env.type) {
        case 'cdp': {
          const ws = localWSMap[env.targetId];
          if (ws && ws.readyState === 1) ws.send(JSON.stringify(env.data));
          break;
        }
        case 'connect': {
          state = 'active';
          setStatus('🤖', 'Your AI is working...', '', 'active');
          // Find and show tab name
          fetchTargets().then(targets => {
            if (targets) {
              const t = targets.find(x => x.id === env.targetId);
              if (t) showTab(t.title);
              connectLocalTarget(t || { id: env.targetId });
            }
          });
          break;
        }
        case 'disconnect': {
          disconnectLocalTarget(env.targetId);
          break;
        }
        case 'agent_disconnected': {
          state = 'connected';
          setStatus('✅', 'Connected! Your AI can now use this browser.', 'Your AI finished working', 'connected');
          break;
        }
        case 'ping': {
          relayWS.send(JSON.stringify({ type: 'pong' }));
          break;
        }
      }
    };

    relayWS.onclose = () => {
      state = 'disconnected';
      if (detectInterval) clearInterval(detectInterval);
      setStatus('🔌', 'Disconnected', 'Connection to OpenClaw lost. Reconnecting...');
      Object.keys(localWSMap).forEach(disconnectLocalTarget);
      localWSMap = {};
      setTimeout(start, 3000);
    };

    relayWS.onerror = () => {};
  }

  window.disconnect = function() {
    if (relayWS) { relayWS.close(); relayWS = null; }
    if (detectInterval) clearInterval(detectInterval);
    Object.keys(localWSMap).forEach(disconnectLocalTarget);
    localWSMap = {};
    state = 'disconnected';
    setStatus('🔌', 'Disconnected', 'You can close this tab now');
  };

  start();
})();
</script>
</body>
</html>`

func generateLinuxScript(userName, relayURL, token string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher — %s
# Generated: %s

RELAY_URL="%s"
RELAY_TOKEN="%s"
USER_NAME="%s"

# Write bridge page to temp file
BRIDGE_PAGE=$(mktemp /tmp/openclaw-browser-XXXXX.html)
cat > "$BRIDGE_PAGE" << 'HTMLEOF'
%s
HTMLEOF

# Replace placeholders
sed -i "s|__RELAY_URL__|$RELAY_URL|g" "$BRIDGE_PAGE"
sed -i "s|__RELAY_TOKEN__|$RELAY_TOKEN|g" "$BRIDGE_PAGE"
sed -i "s|__USER_NAME__|$USER_NAME|g" "$BRIDGE_PAGE"

# Find Chrome
CHROME=$(command -v google-chrome || command -v chromium-browser || command -v chromium)
if [ -z "$CHROME" ]; then
    echo "❌ Chrome not found! Please install Google Chrome or Chromium."
    exit 1
fi

echo "🦞 Launching OpenClaw Browser..."
"$CHROME" --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "file://$BRIDGE_PAGE" &
BROWSER_PID=$!

echo "✅ Browser launched! You can minimize this terminal."
echo "   Close the browser window to stop."
wait $BROWSER_PID
rm -f "$BRIDGE_PAGE"
echo "👋 Browser closed. Goodbye!"
`, userName, time.Now().Format("2006-01-02"), relayURL, token, userName, bridgeHTML)
}

func generateMacScript(userName, relayURL, token string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher — %s
# Generated: %s

RELAY_URL="%s"
RELAY_TOKEN="%s"
USER_NAME="%s"

BRIDGE_PAGE=$(mktemp /tmp/openclaw-browser-XXXXX.html)
cat > "$BRIDGE_PAGE" << 'HTMLEOF'
%s
HTMLEOF

sed -i '' "s|__RELAY_URL__|$RELAY_URL|g" "$BRIDGE_PAGE"
sed -i '' "s|__RELAY_TOKEN__|$RELAY_TOKEN|g" "$BRIDGE_PAGE"
sed -i '' "s|__USER_NAME__|$USER_NAME|g" "$BRIDGE_PAGE"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
if [ ! -f "$CHROME" ]; then
    CHROME=$(command -v chromium || command -v google-chrome || echo "")
fi
if [ -z "$CHROME" ]; then
    echo "❌ Chrome not found! Please install Google Chrome."
    exit 1
fi

echo "🦞 Launching OpenClaw Browser..."
"$CHROME" --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "file://$BRIDGE_PAGE" &
BROWSER_PID=$!

echo "✅ Browser launched! You can minimize this terminal."
wait $BROWSER_PID
rm -f "$BRIDGE_PAGE"
echo "👋 Browser closed. Goodbye!"
`, userName, time.Now().Format("2006-01-02"), relayURL, token, userName, bridgeHTML)
}

func generateWindowsScript(userName, relayURL, token string) string {
	// For Windows we need to escape the HTML differently — write via PowerShell
	// We use a .bat that invokes PowerShell to write the HTML file
	escapedHTML := strings.ReplaceAll(bridgeHTML, "'", "''")
	return fmt.Sprintf(`@echo off
REM OpenClaw Browser Launcher — %s
REM Generated: %s

set "RELAY_URL=%s"
set "RELAY_TOKEN=%s"
set "USER_NAME=%s"

set "BRIDGE_PAGE=%%TEMP%%\openclaw-browser-%%RANDOM%%.html"

REM Write bridge HTML via PowerShell
powershell -Command "$html = @'
%s
'@; $html = $html.Replace('__RELAY_URL__', '%%RELAY_URL%%').Replace('__RELAY_TOKEN__', '%%RELAY_TOKEN%%').Replace('__USER_NAME__', '%%USER_NAME%%'); [System.IO.File]::WriteAllText('%%BRIDGE_PAGE%%', $html, [System.Text.Encoding]::UTF8)"

REM Find Chrome
set "CHROME="
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files\Google\Chrome\Application\chrome.exe"
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
if "%%CHROME%%"=="" (
    echo Chrome not found! Please install Google Chrome.
    pause
    exit /b 1
)

echo Launching OpenClaw Browser...
start "" "%%CHROME%%" --remote-debugging-port=9222 --user-data-dir="%%USERPROFILE%%\OpenClawBrowser" --no-first-run "file:///%%BRIDGE_PAGE%%"
echo Browser launched! You can close this window.
pause
del /f "%%BRIDGE_PAGE%%" 2>nul
`, userName, time.Now().Format("2006-01-02"), relayURL, token, userName, escapedHTML)
}

// HandleDownload generates and serves platform-specific launcher scripts.
func (rl *Relay) HandleDownload(w http.ResponseWriter, r *http.Request) {
	user := auth.Check(r)
	if user == nil {
		http.Error(w, "Unauthorized", 401)
		return
	}

	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "linux"
	}

	userName := userNames[user.ID]
	if userName == "" {
		userName = fmt.Sprintf("User-%d", user.ID)
	}

	token := rl.sessions.GetOrCreateToken(user.ID)
	relayURL := "wss://w-ram.ai.essd.ee/relay/ws"

	var script, filename string
	switch platform {
	case "linux":
		script = generateLinuxScript(userName, relayURL, token)
		filename = fmt.Sprintf("OpenClaw Browser - %s - Ram.sh", userName)
	case "mac":
		script = generateMacScript(userName, relayURL, token)
		filename = fmt.Sprintf("OpenClaw Browser - %s - Ram.command", userName)
	case "windows":
		script = generateWindowsScript(userName, relayURL, token)
		filename = fmt.Sprintf("OpenClaw Browser - %s - Ram.bat", userName)
	default:
		http.Error(w, "Invalid platform. Use: linux, mac, windows", 400)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write([]byte(script))
}
