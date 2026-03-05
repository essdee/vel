package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWrappers(t *testing.T) {
	buildDir := t.TempDir()

	tier2 := map[string]bool{
		"net/http": true,
	}

	rewriteMap, err := GenerateWrappers(buildDir, tier2)
	if err != nil {
		t.Fatal(err)
	}

	if rewriteMap["net/http"] != "vel/cap/net/http" {
		t.Errorf("expected vel/cap/net/http, got %s", rewriteMap["net/http"])
	}

	// Check wrapper file exists
	wrapperFile := filepath.Join(buildDir, "cap", "net", "http", "http.go")
	data, err := os.ReadFile(wrapperFile)
	if err != nil {
		t.Fatalf("wrapper file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "package http") {
		t.Error("wrapper missing package declaration")
	}
	if !strings.Contains(content, `real "net/http"`) {
		t.Error("wrapper missing real import")
	}
	if !strings.Contains(content, `"vel/internal/cap"`) {
		t.Error("wrapper missing cap import")
	}
	// Blacklisted functions should not appear
	if strings.Contains(content, "ListenAndServe") {
		t.Error("wrapper should not contain blacklisted ListenAndServe")
	}
	if strings.Contains(content, "ListenAndServeTLS") {
		t.Error("wrapper should not contain blacklisted ListenAndServeTLS")
	}
}

func TestGenerateWrappersSkipsBlacklistedPackages(t *testing.T) {
	buildDir := t.TempDir()

	tier2 := map[string]bool{
		"os/exec": true, // fully blacklisted
		"net/url": true,
	}

	rewriteMap, err := GenerateWrappers(buildDir, tier2)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := rewriteMap["os/exec"]; ok {
		t.Error("should not generate wrapper for blacklisted package os/exec")
	}
	if _, ok := rewriteMap["net/url"]; !ok {
		t.Error("should generate wrapper for net/url")
	}
}
