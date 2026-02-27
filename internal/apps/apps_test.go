package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "apps"), 0755)
	return dir
}

func writeAppJSON(t *testing.T, root, appName, content string) {
	t.Helper()
	appDir := filepath.Join(root, "apps", appName)
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "app.json"), []byte(content), 0644)
}

func TestValidApp(t *testing.T) {
	root := setupTestDir(t)
	panelsDir := filepath.Join(root, "apps", "myapp", "panels")
	os.MkdirAll(panelsDir, 0755)
	writeAppJSON(t, root, "myapp", `{
		"name": "myapp",
		"version": "1.0.0",
		"title": "My App",
		"panels": "panels/"
	}`)

	apps, errors := Discover(root)
	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %v", errors)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "myapp" {
		t.Errorf("expected name myapp, got %s", apps[0].Name)
	}
	if apps[0].Title != "My App" {
		t.Errorf("expected title My App, got %s", apps[0].Title)
	}
}

func TestMissingName(t *testing.T) {
	root := setupTestDir(t)
	writeAppJSON(t, root, "badapp", `{"version": "1.0.0"}`)

	apps, errors := Discover(root)
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Hint != `Add: "name": "badapp"` {
		t.Errorf("unexpected hint: %s", errors[0].Hint)
	}
}

func TestMissingVersion(t *testing.T) {
	root := setupTestDir(t)
	writeAppJSON(t, root, "noversion", `{"name": "noversion"}`)

	apps, errors := Discover(root)
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}

func TestNoAppJSON(t *testing.T) {
	root := setupTestDir(t)
	os.MkdirAll(filepath.Join(root, "apps", "plain-dir"), 0755)

	apps, errors := Discover(root)
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestRoutesParsing(t *testing.T) {
	root := setupTestDir(t)
	writeAppJSON(t, root, "webapp", `{
		"name": "webapp",
		"version": "2.0.0",
		"routes": {
			"/webapp/": {"type": "static", "dir": "."}
		}
	}`)

	apps, errors := Discover(root)
	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %v", errors)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	r, ok := apps[0].Routes["/webapp/"]
	if !ok {
		t.Fatal("expected route /webapp/")
	}
	if r.Type != "static" || r.Dir != "." {
		t.Errorf("unexpected route: %+v", r)
	}
}
