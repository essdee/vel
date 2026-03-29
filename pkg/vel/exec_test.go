package vel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExec_Success(t *testing.T) {
	result, err := Exec("/tmp", "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected output to contain 'hello', got: %q", result.Output)
	}
}

func TestExec_NonZeroExit(t *testing.T) {
	result, err := Exec("/tmp", "sh", "-c", "exit 42")
	if err != nil {
		t.Fatalf("non-zero exit should not return error, got: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("expected ExitCode 42, got %d", result.ExitCode)
	}
}

func TestExec_CommandNotFound(t *testing.T) {
	result, err := Exec("/tmp", "this-command-does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected error for command not found, got nil")
	}
	if result.ExitCode != -1 {
		t.Errorf("expected ExitCode -1, got %d", result.ExitCode)
	}
}

func TestExec_WorkingDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "vel-exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	filename := "testfile.txt"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := Exec(dir, "ls")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, filename) {
		t.Errorf("expected output to contain %q, got: %q", filename, result.Output)
	}
}

func TestExecWithTimeout_Completes(t *testing.T) {
	result, err := ExecWithTimeout("/tmp", 5*time.Second, "echo", "fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "fast") {
		t.Errorf("expected output to contain 'fast', got: %q", result.Output)
	}
}

func TestExecWithTimeout_TimesOut(t *testing.T) {
	result, err := ExecWithTimeout("/tmp", 100*time.Millisecond, "sleep", "10")
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to contain 'timed out', got: %v", err)
	}
	if result.ExitCode != -1 {
		t.Errorf("expected ExitCode -1 on timeout, got %d", result.ExitCode)
	}
}
