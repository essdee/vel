package datasource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidJSONFileRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	os.WriteFile(path, []byte(`{"hello":"world"}`), 0644)

	m := NewManager()
	if err := m.AddFileSource("testapp", "src1", path, time.Second); err != nil {
		t.Fatal(err)
	}
	m.Start()
	defer m.Stop()

	time.Sleep(200 * time.Millisecond)

	data, ok, stale := m.GetData("testapp:src1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if stale != nil {
		t.Fatal("expected no stale info")
	}
	var parsed map[string]string
	json.Unmarshal(data, &parsed)
	if parsed["hello"] != "world" {
		t.Fatalf("unexpected data: %s", data)
	}
}

func TestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{not json`), 0644)

	m := NewManager()
	m.AddFileSource("app", "bad", path, time.Second)
	m.Start()
	defer m.Stop()

	time.Sleep(300 * time.Millisecond)

	_, ok, stale := m.GetData("app:bad")
	if ok {
		t.Fatal("expected ok=false for invalid JSON")
	}
	if stale == nil {
		t.Fatal("expected stale info")
	}
	if stale.Error != "invalid JSON" {
		t.Fatalf("unexpected error: %s", stale.Error)
	}
}

func TestFileNotFound(t *testing.T) {
	m := NewManager()
	err := m.AddFileSource("app", "missing", "/tmp/vel-test-nonexistent-file.json", time.Second)
	if err != nil {
		t.Fatal("should allow missing files")
	}
	m.Start()
	defer m.Stop()

	time.Sleep(200 * time.Millisecond)

	_, ok, stale := m.GetData("app:missing")
	if ok {
		t.Fatal("expected ok=false")
	}
	if stale == nil || stale.Error != "file not found" {
		t.Fatalf("expected file not found stale, got %+v", stale)
	}
}

func TestMinimumInterval(t *testing.T) {
	m := NewManager()
	err := m.AddFileSource("app", "fast", "/tmp/x.json", 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for interval < 1s")
	}
}

func TestTildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	expanded, err := expandTilde("~/test.json")
	if err != nil {
		t.Fatal(err)
	}
	if expanded != home+"/test.json" {
		t.Fatalf("expected %s/test.json, got %s", home, expanded)
	}

	// No tilde
	noTilde, _ := expandTilde("/absolute/path.json")
	if noTilde != "/absolute/path.json" {
		t.Fatalf("unexpected: %s", noTilde)
	}
}

func TestNamespacing(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.json")
	p2 := filepath.Join(dir, "b.json")
	os.WriteFile(p1, []byte(`{"v":1}`), 0644)
	os.WriteFile(p2, []byte(`{"v":2}`), 0644)

	m := NewManager()
	m.AddFileSource("app1", "data", p1, time.Second)
	m.AddFileSource("app2", "data", p2, time.Second)
	m.Start()
	defer m.Stop()

	time.Sleep(200 * time.Millisecond)

	d1, ok1, _ := m.GetData("app1:data")
	d2, ok2, _ := m.GetData("app2:data")
	if !ok1 || !ok2 {
		t.Fatal("both should be ok")
	}

	var v1, v2 map[string]int
	json.Unmarshal(d1, &v1)
	json.Unmarshal(d2, &v2)
	if v1["v"] != 1 || v2["v"] != 2 {
		t.Fatalf("namespacing failed: %v %v", v1, v2)
	}
}

func TestGetAllData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	os.WriteFile(path, []byte(`{"k":"v"}`), 0644)

	m := NewManager()
	m.AddFileSource("myapp", "src", path, time.Second)
	m.Start()
	defer m.Stop()

	time.Sleep(200 * time.Millisecond)

	all := m.GetAllData()
	state, exists := all["myapp:src"]
	if !exists {
		t.Fatal("expected myapp:src in GetAllData")
	}
	if !state.OK {
		t.Fatal("expected ok")
	}
	if state.Type != "file" {
		t.Fatalf("expected type=file, got %s", state.Type)
	}
}
