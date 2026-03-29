package vel

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecResult holds the output and exit code of a command execution.
type ExecResult struct {
	Output   string // Combined stdout + stderr
	ExitCode int    // 0 = success, non-zero = failure, -1 = exec error
}

// Exec runs an external command and returns the result.
//
// dir: working directory for the command (must be non-empty and absolute).
// name: the command to run (e.g., "git", "make").
// args: arguments to the command.
//
// Returns ExecResult with output and exit code.
// Returns error only for execution failures (command not found, permission denied, etc.).
// A non-zero exit code is NOT an error — check result.ExitCode.
func Exec(dir string, name string, args ...string) (ExecResult, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := buf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ExecResult{Output: output, ExitCode: exitErr.ExitCode()}, nil
		}
		return ExecResult{Output: output, ExitCode: -1}, err
	}
	return ExecResult{Output: output, ExitCode: 0}, nil
}

// ExecWithTimeout runs an external command with a timeout.
// If the command exceeds the timeout, it is killed and an error is returned.
//
// Same parameters as Exec, with an additional timeout duration.
func ExecWithTimeout(dir string, timeout time.Duration, name string, args ...string) (ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := buf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return ExecResult{Output: output, ExitCode: -1}, fmt.Errorf("command timed out after %v", timeout)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ExecResult{Output: output, ExitCode: exitErr.ExitCode()}, nil
		}
		return ExecResult{Output: output, ExitCode: -1}, err
	}
	return ExecResult{Output: output, ExitCode: 0}, nil
}
