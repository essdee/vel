package debug

import (
	"sync"
	"time"
)

// RequestLog holds the full debug data for a single HTTP request.
type RequestLog struct {
	RequestID    string            `json:"request_id"`
	Timestamp    time.Time         `json:"timestamp"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Query        string            `json:"query,omitempty"`
	ClientIP     string            `json:"client_ip"`
	UserAgent    string            `json:"user_agent"`
	Identity     string            `json:"identity"`
	Status       int               `json:"status"`
	LatencyMs    int64             `json:"latency_ms"`
	BytesWritten int64             `json:"bytes_written"`
	Error        string            `json:"error,omitempty"`
	MiddlewareLog []MiddlewareEntry `json:"middleware_log,omitempty"`
	HandlerLog   *HandlerEntry     `json:"handler_log,omitempty"`
}

// BufferStats provides aggregated statistics from the ring buffer.
type BufferStats struct {
	Total       int            `json:"total"`
	ByStatus    map[int]int    `json:"by_status"`
	ByErrorCode map[string]int `json:"by_error_code"`
}

// RingBuffer is a thread-safe circular buffer for RequestLog entries.
type RingBuffer struct {
	mu       sync.RWMutex
	entries  []RequestLog
	capacity int
	head     int // next write position
	count    int
}

// globalBuffer is the singleton ring buffer, initialized when AI debug is on.
var globalBuffer *RingBuffer

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		entries:  make([]RequestLog, capacity),
		capacity: capacity,
	}
}

// GetBuffer returns the global ring buffer (may be nil if AI debug is off).
func GetBuffer() *RingBuffer {
	return globalBuffer
}

// Count returns the number of entries currently in the buffer.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Add inserts a RequestLog entry into the buffer. O(1), overwrites oldest.
func (rb *RingBuffer) Add(log RequestLog) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = log
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
}

// Get retrieves a RequestLog by request ID. O(n) scan.
func (rb *RingBuffer) Get(requestID string) *RequestLog {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	for i := 0; i < rb.count; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		if rb.entries[idx].RequestID == requestID {
			entry := rb.entries[idx]
			return &entry
		}
	}
	return nil
}

// Recent returns the last N entries, newest first.
func (rb *RingBuffer) Recent(n int) []RequestLog {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.count {
		n = rb.count
	}
	result := make([]RequestLog, n)
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		result[i] = rb.entries[idx]
	}
	return result
}

// RecentErrors returns the last N entries with status >= 400, newest first.
func (rb *RingBuffer) RecentErrors(n int) []RequestLog {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []RequestLog
	for i := 0; i < rb.count && len(result) < n; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		if rb.entries[idx].Status >= 400 {
			result = append(result, rb.entries[idx])
		}
	}
	return result
}

// Stats returns aggregated buffer statistics.
func (rb *RingBuffer) Stats() BufferStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	stats := BufferStats{
		Total:       rb.count,
		ByStatus:    make(map[int]int),
		ByErrorCode: make(map[string]int),
	}

	for i := 0; i < rb.count; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		entry := rb.entries[idx]
		stats.ByStatus[entry.Status]++
		if entry.Error != "" {
			stats.ByErrorCode[entry.Error]++
		}
	}
	return stats
}
