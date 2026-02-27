package relay

import (
	"archive/zip"
	"net/http"

	"vel/internal/auth"
)

const linuxScript = `#!/bin/bash
DASHBOARD="https://w-ram.ai.essd.ee"
CHROME=$(command -v google-chrome || command -v chromium-browser || command -v chromium)
if [ -z "$CHROME" ]; then echo "Chrome not found"; exit 1; fi
"$CHROME" --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "$DASHBOARD"
`

const macScript = `#!/bin/bash
DASHBOARD="https://w-ram.ai.essd.ee"
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 --user-data-dir="$HOME/OpenClawBrowser" --no-first-run "$DASHBOARD"
`

const windowsScript = `@echo off
set DASHBOARD=https://w-ram.ai.essd.ee
start "" "C:\Program Files\Google\Chrome\Application\chrome.exe" ^
  --remote-debugging-port=9222 --user-data-dir="%USERPROFILE%\OpenClawBrowser" --no-first-run "%DASHBOARD%"
`

const readme = `# OpenClaw Browser Launcher

These scripts launch Chrome with remote debugging enabled so OpenClaw can control your browser.

## Usage

### Linux
` + "```" + `
chmod +x launch-chrome.sh
./launch-chrome.sh
` + "```" + `

### macOS
Double-click launch-chrome.command (or run it in Terminal).

### Windows
Double-click launch-chrome.bat

## What it does
- Launches Chrome with --remote-debugging-port=9222
- Uses a separate profile (OpenClawBrowser) so it won't affect your main Chrome
- Opens the OpenClaw dashboard automatically
- The dashboard's Browser Relay panel will auto-connect

## Security
- Remote debugging is only accessible on localhost (127.0.0.1)
- The relay connection is encrypted (WSS)
- Only your authenticated OpenClaw agent can send commands
`

// HandleDownload generates and serves the launcher ZIP.
func (rl *Relay) HandleDownload(w http.ResponseWriter, r *http.Request) {
	user := auth.Check(r)
	if user == nil {
		http.Error(w, "Unauthorized", 401)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=openclaw-launcher.zip")

	zw := zip.NewWriter(w)
	defer zw.Close()

	files := map[string]string{
		"launch-chrome.sh":      linuxScript,
		"launch-chrome.command": macScript,
		"launch-chrome.bat":     windowsScript,
		"README.md":             readme,
	}

	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			return
		}
		fw.Write([]byte(content))
	}
}
