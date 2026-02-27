package data

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func GetUsageData(workspace string) json.RawMessage {
	p := filepath.Join(workspace, "claude-usage.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	// Validate it's valid JSON
	var raw json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	return raw
}
