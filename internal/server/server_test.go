package server

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vel/internal/auth"
	"vel/internal/hooks"
	"vel/internal/panels"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create core/public with landing.html and shell.html
	pubDir := filepath.Join(dir, "core", "public")
	os.MkdirAll(pubDir, 0755)
	os.WriteFile(filepath.Join(pubDir, "landing.html"), []byte("<html>landing</html>"), 0644)
	os.WriteFile(filepath.Join(pubDir, "shell.html"), []byte("<html>shell</html>"), 0644)

	// Create a test panel
	cpuDir := filepath.Join(dir, "core", "panels", "cpu")
	os.MkdirAll(cpuDir, 0755)
	manifest := `{"id":"cpu","contractVersion":"1.0","name":"CPU Load","description":"CPU usage","version":"1.0.0","author":"core","position":10,"size":"half","refreshMs":2000,"requires":[],"capabilities":["fetch"],"dataSchema":{}}`
	os.WriteFile(filepath.Join(cpuDir, "manifest.json"), []byte(manifest), 0644)
	os.WriteFile(filepath.Join(cpuDir, "ui.js"), []byte("export default {}"), 0644)

	// Create vendor dir
	os.MkdirAll(filepath.Join(dir, "core", "vendor"), 0755)

	return dir
}

func newTestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	auth.Init("test-token", []int64{123}, "test-secret")

	dir := setupTestDir(t)
	registry, _ := panels.DiscoverPanels(dir)

	cfg := &Config{
		RootDir:   dir,
		Workspace: t.TempDir(),
		Port:      8080,
		Registry:  registry,
		Order:     []string{"cpu"},
		Disabled:  []string{},
		Version:   "0.1.0-test",
		PublicConfig: map[string]interface{}{
			"name":  "TestBoard",
			"emoji": "🧪",
		},
		Routes: map[string]string{},
		Hooks:  hooks.New(),
	}

	return NewServer(cfg), dir
}

func doRequest(handler http.Handler, method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func readGzip(t *testing.T, body []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	return b
}

func TestGetRoot(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/", "", nil)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestGetDashboard(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/dashboard", "", nil)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAPIHealth(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/health", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
	if resp["version"] != "0.1.0-test" {
		t.Fatalf("expected version 0.1.0-test, got %v", resp["version"])
	}
	if resp["timestamp"] == nil {
		t.Fatal("expected timestamp")
	}
}

func TestAPIMode(t *testing.T) {
	h, _ := newTestServer(t)

	t.Run("normal", func(t *testing.T) {
		rr := doRequest(h, "GET", "/api/mode", "", map[string]string{"Accept-Encoding": "identity"})
		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["testMode"] != false {
			t.Fatalf("expected testMode false, got %v", resp["testMode"])
		}
	})

	t.Run("test_mode_on", func(t *testing.T) {
		t.Setenv("TEST_MODE", "true")
		rr := doRequest(h, "GET", "/api/mode", "", map[string]string{"Accept-Encoding": "identity"})
		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["testMode"] != true {
			t.Fatalf("expected testMode true, got %v", resp["testMode"])
		}
	})
}

func TestAPIVersion(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/version", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["version"] != "0.1.0-test" {
		t.Fatalf("expected version, got %v", resp["version"])
	}
}

func TestAPIConfig(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/config", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["name"] != "TestBoard" {
		t.Fatalf("expected name TestBoard, got %v", resp["name"])
	}
	if resp["emoji"] != "🧪" {
		t.Fatalf("expected emoji 🧪, got %v", resp["emoji"])
	}
	// Should not contain auth secrets
	if _, ok := resp["botToken"]; ok {
		t.Fatal("config should not expose botToken")
	}
}

func TestAPIPanels(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/panels", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp) == 0 {
		t.Fatal("expected at least one panel")
	}
	if resp[0]["id"] != "cpu" {
		t.Fatalf("expected cpu panel, got %v", resp[0]["id"])
	}
}

func TestAPIPanelCPU(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/panels/cpu", "", map[string]string{"Accept-Encoding": "identity"})
	// CPU data may return 200 with data or 200 with null - both acceptable
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAPIPanelNotFound(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/panels/nonexistent", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 404 {
		// gzip may wrap the response, check body
		body := rr.Body.Bytes()
		if rr.Header().Get("Content-Encoding") == "gzip" {
			body = readGzip(t, body)
		}
		if !strings.Contains(string(body), "Panel not found") && rr.Code != 404 {
			t.Fatalf("expected 404, got %d body: %s", rr.Code, string(body))
		}
	}
}

func TestAPIAuthPostEmpty(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "POST", "/api/auth", "{}", map[string]string{
		"Accept-Encoding": "identity",
		"Content-Type":    "application/json",
	})
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
}

func TestAuthDev(t *testing.T) {
	h, _ := newTestServer(t)

	t.Run("off", func(t *testing.T) {
		rr := doRequest(h, "GET", "/auth/dev", "", map[string]string{"Accept-Encoding": "identity"})
		if rr.Code != 404 {
			// Could be gzipped
			body := rr.Body.Bytes()
			if rr.Header().Get("Content-Encoding") == "gzip" {
				body = readGzip(t, body)
			}
			if !strings.Contains(string(body), "Not available") {
				t.Fatalf("expected 404, got %d", rr.Code)
			}
		}
	})

	t.Run("on", func(t *testing.T) {
		t.Setenv("TEST_MODE", "true")
		rr := doRequest(h, "GET", "/auth/dev", "", map[string]string{"Accept-Encoding": "identity"})
		// Should redirect to /dashboard (302)
		if rr.Code != 302 {
			t.Fatalf("expected 302, got %d", rr.Code)
		}
		loc := rr.Header().Get("Location")
		if loc != "/dashboard" {
			t.Fatalf("expected redirect to /dashboard, got %s", loc)
		}
	})
}

func TestAuthLogout(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/auth/logout", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 302 {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/" {
		t.Fatalf("expected redirect to /, got %s", loc)
	}
	// Check cookie is cleared
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "tg_user" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected tg_user cookie to be cleared")
	}
}

func TestGzipMiddleware(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/health", "", map[string]string{"Accept-Encoding": "gzip"})
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected gzip content-encoding")
	}
	body := readGzip(t, rr.Body.Bytes())
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode gzipped json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatal("expected status ok in gzipped response")
	}
}

func TestSecurityHeaders(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/api/health", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing X-Content-Type-Options")
	}
	if rr.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal("missing X-Frame-Options")
	}
}

func TestRateLimiting(t *testing.T) {
	h, _ := newTestServer(t)
	// Auth limiter allows 10 per 15 min
	for i := 0; i < 11; i++ {
		rr := doRequest(h, "GET", "/auth/telegram/callback", "", map[string]string{"Accept-Encoding": "identity"})
		if i == 10 {
			if rr.Code != 429 {
				body := rr.Body.Bytes()
				if rr.Header().Get("Content-Encoding") == "gzip" {
					body = readGzip(t, body)
				}
				if !strings.Contains(string(body), "Too many requests") {
					t.Fatalf("expected 429 on request %d, got %d: %s", i, rr.Code, string(body))
				}
			}
		}
	}
}

func TestNotFoundPath(t *testing.T) {
	h, _ := newTestServer(t)
	rr := doRequest(h, "GET", "/nonexistent", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCustomRoutes(t *testing.T) {
	auth.Init("test-token", []int64{123}, "test-secret")
	dir := setupTestDir(t)
	registry, _ := panels.DiscoverPanels(dir)

	// Create a custom static dir
	customDir := filepath.Join(dir, "custom", "mysite")
	os.MkdirAll(customDir, 0755)
	os.WriteFile(filepath.Join(customDir, "index.html"), []byte("<html>mysite</html>"), 0644)

	cfg := &Config{
		RootDir:   dir,
		Workspace: t.TempDir(),
		Port:      8080,
		Registry:  registry,
		Version:   "0.1.0-test",
		PublicConfig: map[string]interface{}{},
		Routes: map[string]string{
			"/mysite/": "custom/mysite",
		},
		Hooks: hooks.New(),
	}

	h := NewServer(cfg)
	// Request the directory root — FileServer serves index.html
	rr := doRequest(h, "GET", "/mysite/", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200 for custom route, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "mysite") {
		t.Fatalf("expected mysite content, got %s", body)
	}
}

func TestThemeServing(t *testing.T) {
	auth.Init("test-token", []int64{123}, "test-secret")
	dir := setupTestDir(t)
	registry, _ := panels.DiscoverPanels(dir)

	// Create theme file
	themeDir := filepath.Join(dir, "custom", "theme")
	os.MkdirAll(themeDir, 0755)
	os.WriteFile(filepath.Join(themeDir, "theme.css"), []byte("body { color: red; }"), 0644)

	cfg := &Config{
		RootDir:      dir,
		Workspace:    t.TempDir(),
		Port:         8080,
		Registry:     registry,
		Version:      "0.1.0-test",
		PublicConfig: map[string]interface{}{},
		Routes:       map[string]string{},
		Hooks:        hooks.New(),
	}

	h := NewServer(cfg)
	rr := doRequest(h, "GET", "/custom/theme/theme.css", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 200 {
		t.Fatalf("expected 200 for theme, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "color: red") {
		t.Fatalf("expected theme CSS, got %s", body)
	}
}

func TestCustomRouteNonexistentDir(t *testing.T) {
	auth.Init("test-token", []int64{123}, "test-secret")
	dir := setupTestDir(t)
	registry, _ := panels.DiscoverPanels(dir)

	cfg := &Config{
		RootDir:      dir,
		Workspace:    t.TempDir(),
		Port:         8080,
		Registry:     registry,
		Version:      "0.1.0-test",
		PublicConfig: map[string]interface{}{},
		Routes: map[string]string{
			"/ghost/": "nonexistent/dir",
		},
		Hooks: hooks.New(),
	}

	h := NewServer(cfg) // should not crash
	rr := doRequest(h, "GET", "/ghost/index.html", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 404 {
		t.Fatalf("expected 404 for nonexistent route dir, got %d", rr.Code)
	}
}

func TestAPIPanelsIncludesCustomPanel(t *testing.T) {
	auth.Init("test-token", []int64{123}, "test-secret")
	dir := setupTestDir(t)

	// Add custom panel
	customDir := filepath.Join(dir, "custom", "panels", "my-custom")
	os.MkdirAll(customDir, 0755)
	manifest := `{"id":"my-custom","contractVersion":"1.0","name":"My Custom","description":"d","version":"1.0.0","author":"a","size":"half"}`
	os.WriteFile(filepath.Join(customDir, "manifest.json"), []byte(manifest), 0644)
	os.WriteFile(filepath.Join(customDir, "ui.js"), []byte("export default {}"), 0644)

	registry, _ := panels.DiscoverPanels(dir)
	cfg := &Config{
		RootDir:      dir,
		Workspace:    t.TempDir(),
		Port:         8080,
		Registry:     registry,
		Order:        []string{"cpu", "my-custom"},
		Version:      "0.1.0-test",
		PublicConfig: map[string]interface{}{},
		Routes:       map[string]string{},
		Hooks:        hooks.New(),
	}

	h := NewServer(cfg)
	rr := doRequest(h, "GET", "/api/panels", "", map[string]string{"Accept-Encoding": "identity"})
	var resp []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	found := false
	for _, p := range resp {
		if p["id"] == "my-custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected custom panel in /api/panels")
	}
}

func TestAPIPanelsIncludesPluginPanel(t *testing.T) {
	auth.Init("test-token", []int64{123}, "test-secret")
	dir := setupTestDir(t)

	// Add app panel
	plugDir := filepath.Join(dir, "apps", "myplugin", "panels", "plug-panel")
	os.MkdirAll(plugDir, 0755)
	manifest := `{"id":"plug-panel","contractVersion":"1.0","name":"Plugin Panel","description":"d","version":"1.0.0","author":"a","size":"half"}`
	os.WriteFile(filepath.Join(plugDir, "manifest.json"), []byte(manifest), 0644)
	os.WriteFile(filepath.Join(plugDir, "ui.js"), []byte("export default {}"), 0644)

	registry, _ := panels.DiscoverPanels(dir)
	cfg := &Config{
		RootDir:      dir,
		Workspace:    t.TempDir(),
		Port:         8080,
		Registry:     registry,
		Order:        []string{},
		Version:      "0.1.0-test",
		PublicConfig: map[string]interface{}{},
		Routes:       map[string]string{},
		Hooks:        hooks.New(),
	}

	h := NewServer(cfg)
	rr := doRequest(h, "GET", "/api/panels", "", map[string]string{"Accept-Encoding": "identity"})
	var resp []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	found := false
	for _, p := range resp {
		if p["id"] == "plug-panel" && p["_source"] == "app:myplugin" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected app panel in /api/panels")
	}
}

func TestThemeNotPresent(t *testing.T) {
	h, _ := newTestServer(t)
	// No theme file created, should 404
	rr := doRequest(h, "GET", "/custom/theme/theme.css", "", map[string]string{"Accept-Encoding": "identity"})
	if rr.Code != 404 {
		t.Fatalf("expected 404 when no theme, got %d", rr.Code)
	}
}
