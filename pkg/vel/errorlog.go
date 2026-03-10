package vel

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrorEntry represents a single error log entry.
type ErrorEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Method    string    `json:"method,omitempty"`
	Status    int       `json:"status"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Hint      string    `json:"hint,omitempty"`
	Stack     string    `json:"stack,omitempty"`
}

// errorLogger manages the error log file. Thread-safe.
type errorLogger struct {
	mu      sync.Mutex
	logPath string
	entries []ErrorEntry
}

var globalErrorLogger *errorLogger

// InitErrorLog initializes the global error logger with the given logs directory.
// Auto-creates the directory. Loads existing entries and trims to the last 1000.
func InitErrorLog(logsDir string) {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return
	}
	logPath := filepath.Join(logsDir, "error.jsonl")
	l := &errorLogger{logPath: logPath}
	l.loadAndTrim(1000)
	globalErrorLogger = l
}

// LogError appends an error entry to the global log. Thread-safe. No-op if not initialized.
func LogError(entry ErrorEntry) {
	if globalErrorLogger == nil {
		return
	}
	globalErrorLogger.log(entry)
}

// GetRecentErrors returns the last N error entries. No-op if not initialized.
func GetRecentErrors(limit int) []ErrorEntry {
	if globalErrorLogger == nil {
		return nil
	}
	return globalErrorLogger.getRecent(limit)
}

func (l *errorLogger) log(entry ErrorEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.WriteString("\n")
}

func (l *errorLogger) getRecent(limit int) []ErrorEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := len(l.entries)
	if limit <= 0 || limit > n {
		limit = n
	}
	start := n - limit
	if start < 0 {
		start = 0
	}
	result := make([]ErrorEntry, n-start)
	copy(result, l.entries[start:])
	return result
}

func (l *errorLogger) loadAndTrim(maxEntries int) {
	f, err := os.Open(l.logPath)
	if err != nil {
		// File doesn't exist yet — that's fine.
		return
	}
	defer f.Close()

	var entries []ErrorEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry ErrorEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	l.entries = entries

	// Rewrite the file with trimmed entries if we pruned anything.
	if len(entries) == maxEntries {
		l.rewriteFile()
	}
}

func (l *errorLogger) rewriteFile() {
	f, err := os.Create(l.logPath)
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, entry := range l.entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		w.Write(data)
		w.WriteByte('\n')
	}
	w.Flush()
}
