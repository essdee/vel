package debug

import (
	"net/http"
	"runtime"
	"sort"
	"time"
)

func handleDiagnose(w http.ResponseWriter, r *http.Request) {
	buf := GetBuffer()
	info := serverInfo

	result := map[string]interface{}{}

	// Server section
	serverSection := map[string]interface{}{
		"go_version":    runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"debug_mode":    IsDebugMode(),
		"ai_debug_mode": IsAIDebugMode(),
	}
	if info != nil {
		serverSection["version"] = info.Version
		serverSection["uptime"] = formatDuration(time.Since(info.StartTime))
	}
	result["server"] = serverSection

	// If no buffer, return minimal info
	if buf == nil {
		result["recent_errors"] = map[string]interface{}{"last_5min": 0}
		writeJSON(w, result)
		return
	}

	// Recent errors
	now := time.Now()
	fiveMinAgo := now.Add(-5 * time.Minute)

	stats := buf.Stats()
	recentErrors := buf.RecentErrors(50)

	last5min := 0
	byCode := map[string]int{}
	var samples []map[string]interface{}

	for _, e := range recentErrors {
		if e.Timestamp.After(fiveMinAgo) {
			last5min++
		}
		if e.Error != "" {
			byCode[e.Error]++
		}
		// Include up to 5 samples
		if len(samples) < 5 {
			ago := formatDuration(now.Sub(e.Timestamp))
			sample := map[string]interface{}{
				"request_id": e.RequestID,
				"status":     e.Status,
				"path":       e.Path,
				"user_agent": e.UserAgent,
				"ago":        ago,
			}
			if e.Error != "" {
				sample["error"] = e.Error
			}
			samples = append(samples, sample)
		}
	}

	if samples == nil {
		samples = []map[string]interface{}{}
	}

	result["recent_errors"] = map[string]interface{}{
		"last_5min": last5min,
		"by_code":   byCode,
		"sample":    samples,
	}

	// Slow requests — find p50, p99, and slowest from all buffered entries
	allEntries := buf.Recent(buf.Count())
	if len(allEntries) > 0 {
		latencies := make([]int64, len(allEntries))
		for i, e := range allEntries {
			latencies[i] = e.LatencyMs
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		p50 := latencies[len(latencies)/2]
		p99idx := int(float64(len(latencies)) * 0.99)
		if p99idx >= len(latencies) {
			p99idx = len(latencies) - 1
		}
		p99 := latencies[p99idx]

		// Top 3 slowest
		var slowest []map[string]interface{}
		// Sort all entries by latency descending
		sort.Slice(allEntries, func(i, j int) bool {
			return allEntries[i].LatencyMs > allEntries[j].LatencyMs
		})
		for i := 0; i < 3 && i < len(allEntries); i++ {
			slowest = append(slowest, map[string]interface{}{
				"request_id": allEntries[i].RequestID,
				"path":       allEntries[i].Path,
				"latency_ms": allEntries[i].LatencyMs,
			})
		}

		result["slow_requests"] = map[string]interface{}{
			"p50_ms":  p50,
			"p99_ms":  p99,
			"slowest": slowest,
		}
	}

	// Buffer stats
	result["buffer"] = map[string]interface{}{
		"total":     stats.Total,
		"by_status": stats.ByStatus,
	}

	// Sessions info
	if info != nil && info.SessionCountFn != nil {
		active, _, _ := info.SessionCountFn()
		result["auth_summary"] = map[string]interface{}{
			"active_sessions": active,
		}
	}

	writeJSON(w, result)
}
