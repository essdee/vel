package relay

import (
	"fmt"
	"net/http"
	"strings"
)

func deriveConnectURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	return fmt.Sprintf("%s://%s/relay/connect", scheme, host)
}

func generateLinuxScript(connectURL string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher

CHROME=$(command -v google-chrome || command -v chromium-browser || command -v chromium)
if [ -z "$CHROME" ]; then
    echo "❌ Chrome not found! Please install Google Chrome or Chromium."
    exit 1
fi

echo "🦞 Launching OpenClaw Browser..."
"$CHROME" --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "%s" &
BROWSER_PID=$!

echo "✅ Browser launched! Follow the pairing instructions in the browser."
wait $BROWSER_PID
echo "👋 Goodbye!"
`, connectURL)
}

func generateMacScript(connectURL string) string {
	return fmt.Sprintf(`#!/bin/bash
# OpenClaw Browser Launcher

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
if [ ! -f "$CHROME" ]; then
    CHROME=$(command -v chromium || command -v google-chrome || echo "")
fi
if [ -z "$CHROME" ]; then
    echo "❌ Chrome not found! Please install Google Chrome."
    exit 1
fi

echo "🦞 Launching OpenClaw Browser..."
"$CHROME" --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "%s" &
BROWSER_PID=$!

echo "✅ Browser launched! Follow the pairing instructions in the browser."
wait $BROWSER_PID
echo "👋 Goodbye!"
`, connectURL)
}

func generateWindowsScript(connectURL string) string {
	return fmt.Sprintf(`@echo off
REM OpenClaw Browser Launcher

set "CHROME="
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files\Google\Chrome\Application\chrome.exe"
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" set "CHROME=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
if "%%CHROME%%"=="" (
    echo Chrome not found! Please install Google Chrome.
    pause
    exit /b 1
)

echo Launching OpenClaw Browser...
start "" "%%CHROME%%" --remote-debugging-port=9222 --user-data-dir="%%USERPROFILE%%\OpenClawBrowser" --no-first-run "%s"
echo Browser launched! Follow the pairing instructions in the browser.
pause
`, connectURL)
}

// HandleDownload generates and serves platform-specific launcher scripts.
func (rl *Relay) HandleDownload(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "linux"
	}

	connectURL := deriveConnectURL(r)

	var script, filename, contentType string
	switch strings.ToLower(platform) {
	case "linux":
		script = generateLinuxScript(connectURL)
		filename = "OpenClaw-Browser.sh"
		contentType = "application/x-sh"
	case "mac":
		script = generateMacScript(connectURL)
		filename = "OpenClaw-Browser.command"
		contentType = "application/x-sh"
	case "windows":
		script = generateWindowsScript(connectURL)
		filename = "OpenClaw-Browser.bat"
		contentType = "application/x-bat"
	default:
		http.Error(w, "Invalid platform. Use: linux, mac, windows", 400)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write([]byte(script))
}
