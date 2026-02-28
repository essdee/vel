package relay

import (
	"fmt"
	"net/http"
	"strings"
)

func deriveBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func deriveWSURL(r *http.Request) string {
	scheme := "wss"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd == "http" || fwd == "" {
			scheme = "ws"
		}
	}
	return fmt.Sprintf("%s://%s/relay/ws", scheme, r.Host)
}

// HandleBridge serves the raw bridge HTML template (with placeholders).
// No auth required — the template contains no secrets.
func (rl *Relay) HandleBridge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(bridgeHTML))
}

// HandleDownload generates platform-specific launcher scripts.
// No auth required — pairing happens at runtime.
func (rl *Relay) HandleDownload(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "linux"
	}

	baseURL := deriveBaseURL(r)
	wsURL := deriveWSURL(r)
	botUsername := "EmpRamBot"

	var script, filename string
	switch platform {
	case "linux":
		script = generateLinuxScript(baseURL, wsURL, botUsername)
		filename = "openclaw-browser.sh"
	case "mac":
		script = generateMacScript(baseURL, wsURL, botUsername)
		filename = "openclaw-browser.command"
	case "windows":
		script = generateWindowsScript(baseURL, wsURL, botUsername)
		filename = "openclaw-browser.bat"
	default:
		http.Error(w, "Invalid platform. Use: linux, mac, windows", 400)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write([]byte(script))
}

// bridgeHTML is the bridge page that runs from file://.
// It uses ONLY WebSocket — no HTTP fetch (avoids CORS on /json endpoint).
// Placeholders: __RELAY_URL__, __RELAY_TOKEN__, __BROWSER_WS_URL__
const bridgeHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>OpenClaw Browser</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #0e0e12; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
  .container { max-width: 480px; width: 100%; padding: 40px 24px; text-align: center; }
  .logo { font-size: 48px; margin-bottom: 8px; }
  h1 { font-size: 22px; font-weight: 600; color: #fff; margin-bottom: 32px; }
  .status-icon { font-size: 40px; margin-bottom: 12px; }
  .status-text { font-size: 18px; font-weight: 500; }
  .status-sub { font-size: 14px; color: #888; margin-top: 8px; }
  .card { background: #16161c; border: 1px solid #2a2a35; border-radius: 12px; padding: 20px; margin-bottom: 16px; }
  @keyframes glow { 0%,100% { box-shadow: 0 0 20px rgba(34,197,94,0.2); } 50% { box-shadow: 0 0 40px rgba(34,197,94,0.4); } }
  @keyframes aglow { 0%,100% { box-shadow: 0 0 20px rgba(0,170,255,0.2); } 50% { box-shadow: 0 0 40px rgba(0,170,255,0.5); } }
  .card.connected { animation: glow 3s ease-in-out infinite; }
  .card.active { animation: aglow 1.5s ease-in-out infinite; }
  .tab-name { color: #0af; font-size: 14px; margin-top: 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 400px; margin: 8px auto 0; }
  .tips { display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; margin-bottom: 24px; }
  .tip { background: #1a1a24; border: 1px solid #2a2a35; border-radius: 6px; padding: 6px 12px; font-size: 12px; color: #888; }
  .btn { background: transparent; border: 1px solid #444; color: #aaa; padding: 8px 20px; border-radius: 6px; cursor: pointer; font-size: 13px; }
  .btn:hover { border-color: #ef4444; color: #ef4444; }
  .hidden { display: none; }
</style>
</head>
<body>
<div class="container">
  <div class="logo">🦞</div>
  <h1>OpenClaw Browser</h1>
  <div class="card" id="statusCard">
    <div class="status-icon" id="statusIcon">🔗</div>
    <div class="status-text" id="statusText">Connecting...</div>
    <div class="status-sub" id="statusSub"></div>
    <div class="tab-name hidden" id="tabName"></div>
  </div>
  <div class="tips">
    <span class="tip">💡 Keep this tab open</span>
    <span class="tip">🖥️ You can use other apps</span>
  </div>
  <button class="btn" onclick="disconnect()">Disconnect</button>
</div>
<script>
(function() {
  const RELAY_URL = '__RELAY_URL__';
  const RELAY_TOKEN = '__RELAY_TOKEN__';
  const BROWSER_WS = '__BROWSER_WS_URL__';
  const $ = id => document.getElementById(id);

  let relayWS = null;
  let browserWS = null;
  let sessionMap = {};  // sessionId -> targetId
  let targetMap = {};   // targetId -> sessionId
  let targets = [];
  let cdpId = 1;
  let refreshInterval = null;

  function setStatus(icon, text, sub, cls) {
    $('statusIcon').textContent = icon;
    $('statusText').textContent = text;
    $('statusSub').textContent = sub || '';
    $('statusCard').className = 'card' + (cls ? ' ' + cls : '');
    $('tabName').classList.add('hidden');
  }

  function showTab(name) {
    const el = $('tabName');
    el.textContent = '🔍 ' + name;
    el.classList.remove('hidden');
  }

  // Discover targets via CDP Target.getTargets() over browser WS
  function refreshTargets() {
    if (!browserWS || browserWS.readyState !== 1) return;
    browserWS.send(JSON.stringify({ id: cdpId++, method: 'Target.getTargets' }));
  }

  // Attach to a target using flatten mode (multiplexed over browser WS)
  function attachTarget(targetId) {
    if (targetMap[targetId]) return; // already attached
    const id = cdpId++;
    browserWS.send(JSON.stringify({
      id: id,
      method: 'Target.attachToTarget',
      params: { targetId: targetId, flatten: true }
    }));
  }

  function detachTarget(targetId) {
    const sessionId = targetMap[targetId];
    if (!sessionId) return;
    browserWS.send(JSON.stringify({
      id: cdpId++,
      method: 'Target.detachFromTarget',
      params: { sessionId: sessionId }
    }));
    delete targetMap[targetId];
    delete sessionMap[sessionId];
  }

  function filterTargets(list) {
    return list.filter(t =>
      t.type === 'page' &&
      !t.url.startsWith('chrome://') &&
      !t.url.startsWith('chrome-extension://') &&
      !t.url.includes('openclaw-bridge-') &&
      !t.url.startsWith('devtools://') &&
      t.url !== '' &&
      t.url !== 'about:blank'
    );
  }

  function connectBrowser() {
    // If WS URL is placeholder, wait for launcher to inject it and reload
    if (BROWSER_WS === '__SELF_DISCOVER__') {
      setStatus('⏳', 'Waiting for CDP...', 'Launcher will reload this page shortly');
      return;
    }
    setStatus('🔗', 'Connecting to browser...', '');
    browserWS = new WebSocket(BROWSER_WS);

    browserWS.onopen = () => {
      console.log('[bridge] connected to browser CDP');
      // Enable target discovery events
      browserWS.send(JSON.stringify({ id: cdpId++, method: 'Target.setDiscoverTargets', params: { discover: true } }));
      refreshTargets();
      refreshInterval = setInterval(refreshTargets, 5000);
      connectRelay();
    };

    browserWS.onmessage = (e) => {
      const msg = JSON.parse(e.data);

      // Handle Target.getTargets response
      if (msg.result && msg.result.targetInfos) {
        targets = filterTargets(msg.result.targetInfos);
        // Send target list to relay
        if (relayWS && relayWS.readyState === 1) {
          const relayTargets = targets.map(t => ({
            id: t.targetId,
            title: t.title,
            url: t.url,
            type: t.type,
            webSocketDebuggerUrl: 'ws://localhost:9222/devtools/page/' + t.targetId
          }));
          relayWS.send(JSON.stringify({ type: 'targets', data: relayTargets }));
        }
        return;
      }

      // Handle attachToTarget response (get sessionId)
      if (msg.result && msg.result.sessionId) {
        // Find which target this was for by checking pending
        // We'll handle via targetCreated/attachedToTarget events instead
      }

      // Handle Target.attachedToTarget event (flatten mode)
      if (msg.method === 'Target.attachedToTarget') {
        const sessionId = msg.params.sessionId;
        const targetId = msg.params.targetInfo.targetId;
        sessionMap[sessionId] = targetId;
        targetMap[targetId] = sessionId;
        console.log('[bridge] attached to target', targetId, 'session', sessionId);
        return;
      }

      // Handle Target.detachedFromTarget
      if (msg.method === 'Target.detachedFromTarget') {
        const sessionId = msg.params.sessionId;
        const targetId = sessionMap[sessionId];
        if (targetId) delete targetMap[targetId];
        delete sessionMap[sessionId];
        return;
      }

      // Handle Target.targetCreated / targetDestroyed (from setDiscoverTargets)
      if (msg.method === 'Target.targetCreated' || msg.method === 'Target.targetInfoChanged' || msg.method === 'Target.targetDestroyed') {
        refreshTargets(); // refresh the full list
        return;
      }

      // Handle CDP messages from attached targets (sessionId present = from a target)
      if (msg.sessionId) {
        const targetId = sessionMap[msg.sessionId];
        if (targetId && relayWS && relayWS.readyState === 1) {
          relayWS.send(JSON.stringify({ type: 'cdp', targetId: targetId, data: msg }));
        }
        return;
      }
    };

    browserWS.onclose = () => {
      console.log('[bridge] browser WS closed');
      if (refreshInterval) clearInterval(refreshInterval);
      sessionMap = {};
      targetMap = {};
      setStatus('❌', 'Browser disconnected', 'Reconnecting in 3s...');
      setTimeout(connectBrowser, 3000);
    };

    browserWS.onerror = () => {};
  }

  function connectRelay() {
    setStatus('🔗', 'Connecting to relay...', '');
    relayWS = new WebSocket(RELAY_URL + '?token=' + RELAY_TOKEN);

    relayWS.onopen = () => {
      setStatus('✅', 'Connected!', 'Waiting for your AI...', 'connected');
      refreshTargets(); // send current targets
    };

    relayWS.onmessage = (e) => {
      const env = JSON.parse(e.data);

      switch (env.type) {
        case 'cdp': {
          // Forward CDP command to correct target via sessionId
          const sessionId = targetMap[env.targetId];
          if (sessionId && browserWS && browserWS.readyState === 1) {
            const cdpMsg = env.data;
            cdpMsg.sessionId = sessionId;
            browserWS.send(JSON.stringify(cdpMsg));
          }
          break;
        }
        case 'connect': {
          // Agent wants to connect to a target
          setStatus('🤖', 'AI is working...', '', 'active');
          const t = targets.find(x => x.targetId === env.targetId);
          if (t) showTab(t.title);
          attachTarget(env.targetId);
          break;
        }
        case 'disconnect': {
          detachTarget(env.targetId);
          break;
        }
        case 'agent_disconnected': {
          setStatus('✅', 'Connected!', 'AI finished', 'connected');
          break;
        }
        case 'ping': {
          relayWS.send(JSON.stringify({ type: 'pong' }));
          break;
        }
      }
    };

    relayWS.onclose = () => {
      setStatus('🔌', 'Relay disconnected', 'Reconnecting...');
      setTimeout(connectRelay, 3000);
    };

    relayWS.onerror = () => {};
  }

  window.disconnect = function() {
    if (relayWS) { relayWS.close(); relayWS = null; }
    if (browserWS) { browserWS.close(); browserWS = null; }
    if (refreshInterval) clearInterval(refreshInterval);
    sessionMap = {};
    targetMap = {};
    setStatus('🔌', 'Disconnected', 'You can close this tab');
  };

  connectBrowser();
})();
</script>
</body>
</html>`

func generateLinuxScript(baseURL, wsURL, botUsername string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher

SERVER="%s"
WS_URL="%s"
BOT="%s"

CHROME=$(command -v google-chrome || command -v chromium-browser || command -v chromium 2>/dev/null)
if [ -z "$CHROME" ]; then
    echo "❌ Chrome/Chromium not found."
    exit 1
fi

# Check saved token
TOKEN_FILE="$HOME/.openclaw-relay-token"
RELAY_TOKEN=""
if [ -f "$TOKEN_FILE" ]; then
    RELAY_TOKEN=$(cat "$TOKEN_FILE")
    echo "🔑 Found saved session. Reconnecting..."
fi

if [ -z "$RELAY_TOKEN" ]; then
    echo "🦞 Getting pairing code..."
    PAIR_RESP=$(curl -s "$SERVER/relay/pair/new" 2>/dev/null)
    if [ -z "$PAIR_RESP" ]; then
        echo "❌ Could not reach server at $SERVER"
        exit 1
    fi
    CODE=$(echo "$PAIR_RESP" | grep -o '"code":"[^"]*"' | head -1 | cut -d'"' -f4)
    PTOKEN=$(echo "$PAIR_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "   Your pairing code:  $CODE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "   Send to Ram on Telegram: pair $CODE"
    echo "   Or tap: https://t.me/${BOT}?start=pair_${CODE}"
    echo ""
    echo -n "⏳ Waiting for pairing"

    for i in $(seq 1 150); do
        STATUS=$(curl -s "$SERVER/relay/pair/status?token=$PTOKEN" 2>/dev/null)
        if echo "$STATUS" | grep -q '"activated":true'; then
            RELAY_TOKEN=$(echo "$STATUS" | grep -o '"relayToken":"[^"]*"' | head -1 | cut -d'"' -f4)
            break
        fi
        echo -n "."
        sleep 2
    done
    echo ""

    if [ -z "$RELAY_TOKEN" ]; then
        echo "❌ Pairing timed out."
        exit 1
    fi
    echo "$RELAY_TOKEN" > "$TOKEN_FILE"
    echo "✅ Paired!"
fi

# Prepare bridge HTML before launching Chrome
# We need the browser WS URL, but we don't have it yet.
# The bridge will discover it via fetch to localhost:9222/json/version.
# So we inject relay URL/token now, and use __SELF_DISCOVER__ for browser WS.
BRIDGE=$(mktemp /tmp/openclaw-bridge-XXXXX.html)
curl -s "$SERVER/relay/bridge" > "$BRIDGE"
sed -i "s|__RELAY_URL__|$WS_URL|g" "$BRIDGE"
sed -i "s|__RELAY_TOKEN__|$RELAY_TOKEN|g" "$BRIDGE"
# Browser WS URL will be discovered after Chrome starts — inject placeholder
sed -i "s|__BROWSER_WS_URL__|__SELF_DISCOVER__|g" "$BRIDGE"

# Launch Chrome with CDP and bridge as startup page
echo "🌐 Launching browser..."
"$CHROME" \
    --remote-debugging-port=9222 \
    '--remote-allow-origins=*' \
    --user-data-dir="$HOME/OpenClawBrowser" \
    --no-first-run \
    "file://$BRIDGE" 2>/dev/null &
BROWSER_PID=$!

# Wait for CDP to be ready
echo -n "⏳ Waiting for CDP"
for i in $(seq 1 30); do
    sleep 1
    VJSON=$(curl -s http://127.0.0.1:9222/json/version 2>/dev/null)
    if [ -n "$VJSON" ]; then
        BROWSER_WS=$(echo "$VJSON" | grep -o '"webSocketDebuggerUrl": *"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$BROWSER_WS" ]; then break; fi
    fi
    echo -n "."
done
echo ""

if [ -z "$BROWSER_WS" ]; then
    echo "❌ Chrome CDP not responding on port 9222"
    kill $BROWSER_PID 2>/dev/null
    rm -f "$BRIDGE"
    exit 1
fi
echo "✅ CDP ready"

# Now inject the real browser WS URL and reload the bridge
sed -i "s|__SELF_DISCOVER__|$BROWSER_WS|g" "$BRIDGE"
# Reload the bridge tab so it picks up the WS URL
TAB_ID=$(curl -s "http://127.0.0.1:9222/json" | grep -o '"id": *"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$TAB_ID" ]; then
    curl -s "http://127.0.0.1:9222/json/navigate/$TAB_ID?file://$BRIDGE" > /dev/null 2>&1
fi

echo "✅ Browser ready! Keep this terminal open."
echo "   Close browser window to stop."
wait $BROWSER_PID 2>/dev/null
rm -f "$BRIDGE"
echo "👋 Goodbye!"
`, baseURL, wsURL, botUsername)
}

func generateMacScript(baseURL, wsURL, botUsername string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher

SERVER="%s"
WS_URL="%s"
BOT="%s"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
if [ ! -f "$CHROME" ]; then
    CHROME=$(command -v chromium || command -v google-chrome || true)
fi
if [ -z "$CHROME" ] || [ ! -f "$CHROME" ]; then
    echo "❌ Chrome not found."
    exit 1
fi

TOKEN_FILE="$HOME/.openclaw-relay-token"
RELAY_TOKEN=""
if [ -f "$TOKEN_FILE" ]; then
    RELAY_TOKEN=$(cat "$TOKEN_FILE")
    echo "🔑 Found saved session. Reconnecting..."
fi

if [ -z "$RELAY_TOKEN" ]; then
    echo "🦞 Getting pairing code..."
    PAIR_RESP=$(curl -sf "$SERVER/relay/pair/new" 2>/dev/null || true)
    if [ -z "$PAIR_RESP" ]; then echo "❌ Could not reach server"; exit 1; fi
    CODE=$(echo "$PAIR_RESP" | grep -o '"code":"[^"]*"' | head -1 | cut -d'"' -f4)
    PTOKEN=$(echo "$PAIR_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "   Your pairing code:  $CODE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "   Send to Ram: pair $CODE"
    echo "   Or tap: https://t.me/${BOT}?start=pair_${CODE}"
    echo ""
    echo -n "⏳ Waiting"

    for i in $(seq 1 150); do
        STATUS=$(curl -sf "$SERVER/relay/pair/status?token=$PTOKEN" 2>/dev/null || true)
        if echo "$STATUS" | grep -q '"activated":true'; then
            RELAY_TOKEN=$(echo "$STATUS" | grep -o '"relayToken":"[^"]*"' | head -1 | cut -d'"' -f4)
            break
        fi
        echo -n "."
        sleep 2
    done
    echo ""
    if [ -z "$RELAY_TOKEN" ]; then echo "❌ Timed out."; exit 1; fi
    echo "$RELAY_TOKEN" > "$TOKEN_FILE"
    echo "✅ Paired!"
fi

# Prepare bridge HTML before launching Chrome
BRIDGE=$(mktemp /tmp/openclaw-bridge-XXXXX.html)
curl -s "$SERVER/relay/bridge" > "$BRIDGE"
sed -i '' "s|__RELAY_URL__|$WS_URL|g" "$BRIDGE"
sed -i '' "s|__RELAY_TOKEN__|$RELAY_TOKEN|g" "$BRIDGE"
sed -i '' "s|__BROWSER_WS_URL__|__SELF_DISCOVER__|g" "$BRIDGE"

# Launch Chrome with CDP and bridge as startup page
echo "🌐 Launching browser..."
"$CHROME" \
    --remote-debugging-port=9222 \
    '--remote-allow-origins=*' \
    --user-data-dir="$HOME/OpenClawBrowser" \
    --no-first-run \
    "file://$BRIDGE" 2>/dev/null &
BROWSER_PID=$!

echo -n "⏳ Waiting for CDP"
for i in $(seq 1 30); do
    sleep 1
    VJSON=$(curl -s http://127.0.0.1:9222/json/version 2>/dev/null)
    if [ -n "$VJSON" ]; then
        BROWSER_WS=$(echo "$VJSON" | grep -o '"webSocketDebuggerUrl": *"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$BROWSER_WS" ]; then break; fi
    fi
    echo -n "."
done
echo ""
if [ -z "$BROWSER_WS" ]; then echo "❌ CDP not ready"; kill $BROWSER_PID 2>/dev/null; rm -f "$BRIDGE"; exit 1; fi
echo "✅ CDP ready"

# Inject real browser WS URL and reload bridge
sed -i '' "s|__SELF_DISCOVER__|$BROWSER_WS|g" "$BRIDGE"
TAB_ID=$(curl -s "http://127.0.0.1:9222/json" | grep -o '"id": *"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$TAB_ID" ]; then
    curl -s "http://127.0.0.1:9222/json/navigate/$TAB_ID?file://$BRIDGE" > /dev/null 2>&1
fi

echo "✅ Ready! Keep this terminal open."
wait $BROWSER_PID 2>/dev/null
rm -f "$BRIDGE"
echo "👋 Goodbye!"
`, baseURL, wsURL, botUsername)
}

func generateWindowsScript(baseURL, wsURL, botUsername string) string {
	escapedHTML := strings.ReplaceAll(bridgeHTML, "'", "''")
	_ = escapedHTML
	return fmt.Sprintf(`@echo off
setlocal EnableDelayedExpansion
REM OpenClaw Browser Launcher

set "SERVER=%s"
set "WS_URL=%s"
set "BOT=%s"
set "TOKEN_FILE=%%USERPROFILE%%\.openclaw-relay-token"
set "RELAY_TOKEN="

if exist "%%TOKEN_FILE%%" (
    set /p RELAY_TOKEN=<"%%TOKEN_FILE%%"
    echo Found saved session. Reconnecting...
)

if "%%RELAY_TOKEN%%"=="" (
    echo Getting pairing code...
    for /f "usebackq delims=" %%%%i in (`+"`"+`powershell -Command "$r = Invoke-RestMethod '%%SERVER%%/relay/pair/new'; Write-Host $r.code '|' $r.token"`+"`"+`) do (
        for /f "tokens=1,2 delims=|" %%%%a in ("%%%%i") do (
            set "CODE=%%%%a"
            set "PTOKEN=%%%%b"
        )
    )
    set "CODE=!CODE: =!"
    set "PTOKEN=!PTOKEN: =!"
    echo.
    echo Your pairing code: !CODE!
    echo Send to Ram: pair !CODE!
    echo.
    echo Waiting for pairing...
    :POLL
    powershell -Command "Start-Sleep 2"
    for /f "usebackq delims=" %%%%i in (`+"`"+`powershell -Command "$r = Invoke-RestMethod '%%SERVER%%/relay/pair/status?token=%%PTOKEN%%'; if ($r.activated) { $r.relayToken } else { 'waiting' }"`+"`"+`) do set "RESULT=%%%%i"
    if "%%RESULT%%"=="waiting" goto POLL
    set "RELAY_TOKEN=%%RESULT%%"
    echo %%RELAY_TOKEN%%>"%%TOKEN_FILE%%"
    echo Paired!
)

echo Preparing bridge page...
set "BRIDGE=%%TEMP%%\openclaw-bridge-%%RANDOM%%.html"
powershell -Command "$h = @'
%s
'@; $h = $h.Replace('__RELAY_URL__','%%WS_URL%%').Replace('__RELAY_TOKEN__','%%RELAY_TOKEN%%').Replace('__BROWSER_WS_URL__','__SELF_DISCOVER__'); [IO.File]::WriteAllText('%%BRIDGE%%',$h,[Text.Encoding]::UTF8)"

echo Launching browser...
set "CHROME="
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files\Google\Chrome\Application\chrome.exe"
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
if "%%CHROME%%"=="" ( echo Chrome not found! & pause & exit /b 1 )

start "" "%%CHROME%%" --remote-debugging-port=9222 --remote-allow-origins=* --user-data-dir="%%USERPROFILE%%\OpenClawBrowser" --no-first-run "file:///%%BRIDGE%%"

echo Waiting for CDP...
:CDPWAIT
powershell -Command "Start-Sleep 1"
for /f "usebackq delims=" %%%%i in (`+"`"+`powershell -Command "try { (Invoke-RestMethod 'http://localhost:9222/json/version').webSocketDebuggerUrl } catch { 'waiting' }"`+"`"+`) do set "BROWSER_WS=%%%%i"
if "%%BROWSER_WS%%"=="waiting" goto CDPWAIT

REM Inject real WS URL and reload bridge
powershell -Command "$h = [IO.File]::ReadAllText('%%BRIDGE%%'); $h = $h.Replace('__SELF_DISCOVER__','%%BROWSER_WS%%'); [IO.File]::WriteAllText('%%BRIDGE%%',$h,[Text.Encoding]::UTF8)"
powershell -Command "try { $tabs = Invoke-RestMethod 'http://localhost:9222/json'; $id = $tabs[0].id; Invoke-RestMethod ('http://localhost:9222/json/navigate/' + $id + '?file:///%%BRIDGE%%') } catch {}" >nul 2>&1

echo Ready! Keep this window open.
pause
`, baseURL, wsURL, botUsername, strings.ReplaceAll(bridgeHTML, "'", "''"))
}
