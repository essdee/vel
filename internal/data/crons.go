package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func GetCronJobs(workspace string) json.RawMessage {
	cronPaths := []string{
		filepath.Join(workspace, "..", "cron", "jobs.json"),
		filepath.Join(workspace, "..", "agents", "main", "cron-jobs.json"),
	}

	for _, p := range cronPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var raw interface{}
		if json.Unmarshal(data, &raw) != nil {
			continue
		}

		var jobs []interface{}
		switch v := raw.(type) {
		case []interface{}:
			jobs = v
		case map[string]interface{}:
			if j, ok := v["jobs"].([]interface{}); ok {
				jobs = j
			}
		}

		var result []map[string]interface{}
		for _, j := range jobs {
			job, ok := j.(map[string]interface{})
			if !ok {
				continue
			}
			entry := map[string]interface{}{
				"id":      getStr(job, "id"),
				"name":    getStrDefault(job, "name", "Unnamed"),
				"enabled": getBoolDefault(job, "enabled", true),
			}
			if s, ok := job["schedule"].(string); ok {
				entry["schedule"] = s
			} else {
				entry["schedule"] = nil
			}
			if s, ok := job["sessionTarget"].(string); ok {
				entry["sessionTarget"] = s
			} else {
				entry["sessionTarget"] = nil
			}

			if payload, ok := job["payload"].(map[string]interface{}); ok {
				entry["model"] = payload["model"]
				entry["payloadKind"] = payload["kind"]
			} else {
				entry["model"] = nil
				entry["payloadKind"] = nil
			}

			if state, ok := job["state"].(map[string]interface{}); ok {
				entry["lastStatus"] = state["lastStatus"]
				if ms, ok := state["lastRunAtMs"].(float64); ok {
					entry["lastRunAt"] = time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339)
				} else {
					entry["lastRunAt"] = nil
				}
				entry["lastDurationMs"] = state["lastDurationMs"]
				if ms, ok := state["nextRunAtMs"].(float64); ok {
					entry["nextRunAt"] = time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339)
				} else {
					entry["nextRunAt"] = nil
				}
				if ce, ok := state["consecutiveErrors"].(float64); ok {
					entry["consecutiveErrors"] = int(ce)
				} else {
					entry["consecutiveErrors"] = 0
				}
				entry["lastError"] = state["lastError"]
			} else {
				entry["lastStatus"] = nil
				entry["lastRunAt"] = nil
				entry["lastDurationMs"] = nil
				entry["nextRunAt"] = nil
				entry["consecutiveErrors"] = 0
				entry["lastError"] = nil
			}

			result = append(result, entry)
		}

		out, _ := json.Marshal(result)
		return out
	}

	return json.RawMessage("[]")
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStrDefault(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func getBoolDefault(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}
