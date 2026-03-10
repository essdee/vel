package server

import (
	"net/http"
	"os"
	"strings"
)

// authBootstrapScript is injected before </head> in every HTML page.
// It handles Telegram Mini App auth transparently:
// 1. Checks if running inside Telegram WebApp
// 2. If no auth cookie exists, POSTs initData to /api/auth
// 3. On success, reloads the page (now with cookie set)
// This means app pages never need to handle Telegram auth themselves.
const authBootstrapScript = `<script>
(function(){
  var tg = window.Telegram && window.Telegram.WebApp;
  if (!tg || !tg.initData) return;
  if (document.cookie.indexOf('tg_user=') !== -1) return;
  if (sessionStorage.getItem('_vel_auth_tried')) return;
  sessionStorage.setItem('_vel_auth_tried', '1');
  fetch('/api/auth', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({initData: tg.initData})
  }).then(function(r){ return r.json(); }).then(function(d){
    if (d && d.ok) {
      sessionStorage.removeItem('_vel_auth_tried');
      location.reload();
    }
  }).catch(function(){});
})();
</script>`

// serveHTMLWithAuth reads an HTML file, injects the auth bootstrap script,
// and serves it. This ensures every page works with Telegram Mini App auth
// without requiring per-app auth handling.
func serveHTMLWithAuth(w http.ResponseWriter, r *http.Request, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		serve404(w, r)
		return
	}

	content := string(data)

	// Only inject into HTML files
	if !strings.Contains(content, "<html") && !strings.Contains(content, "<!DOCTYPE") && !strings.Contains(content, "<!doctype") {
		http.ServeFile(w, r, filePath)
		return
	}

	// Inject before </head> if present, otherwise before </body>
	injected := false
	for _, tag := range []string{"</head>", "</HEAD>", "</body>", "</BODY>"} {
		if idx := strings.Index(content, tag); idx >= 0 {
			content = content[:idx] + authBootstrapScript + "\n" + content[idx:]
			injected = true
			break
		}
	}
	if !injected {
		content += authBootstrapScript
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(200)
	w.Write([]byte(content))
}
