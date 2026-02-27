package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteImports(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "test.go")

	src := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("hello")
	_ = http.StatusOK
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	destFile := filepath.Join(srcDir, "test_rewritten.go")
	rewriteMap := map[string]string{
		"net/http": "vel/cap/net/http",
	}

	if err := RewriteImports(srcFile, destFile, rewriteMap); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, `"vel/cap/net/http"`) {
		t.Errorf("expected rewritten import, got:\n%s", content)
	}
	if strings.Contains(content, `"net/http"`) {
		t.Errorf("original import should be rewritten, got:\n%s", content)
	}
	// fmt should stay unchanged
	if !strings.Contains(content, `"fmt"`) {
		t.Errorf("tier 1 import should not be rewritten, got:\n%s", content)
	}
}

func TestRewriteImportsNoMap(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "test.go")

	src := `package main

import "fmt"

func main() { fmt.Println("hi") }
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	destFile := filepath.Join(srcDir, "out.go")
	if err := RewriteImports(srcFile, destFile, map[string]string{}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"fmt"`) {
		t.Error("unchanged file should still have fmt import")
	}
}
