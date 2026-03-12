package vel

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// velRoot is set by the server at startup via SetRoot().
var velRoot string
var velRootOnce sync.Once

// SetRoot stores the Vel root directory for framework functions.
// Called once by the server at startup.
func SetRoot(root string) {
	velRootOnce.Do(func() { velRoot = root })
}

// RefreshUsage runs sdk/openclaw/claude-usage-poll.sh synchronously.
// The canonical location is sdk/openclaw/ inside the Vel root.
// Apps can call this since they don't have os/exec access in the sandboxed build.
func RefreshUsage() error {
	home, _ := os.UserHomeDir()

	// Primary: sdk/openclaw/ inside Vel root
	candidates := []string{}
	if velRoot != "" {
		candidates = append(candidates, filepath.Join(velRoot, "sdk", "openclaw", "claude-usage-poll.sh"))
	}
	// Fallback: workspace skill location
	candidates = append(candidates, filepath.Join(home, ".openclaw/workspace/skills/claude-usage-monitor/scripts/claude-usage-poll.sh"))

	var scriptPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			scriptPath = p
			break
		}
	}

	if scriptPath == "" {
		log.Printf("[vel] RefreshUsage: no poll script found (checked sdk/openclaw/)")
		return nil
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "HOME="+home)
	if err := cmd.Run(); err != nil {
		log.Printf("[vel] RefreshUsage: script failed: %v", err)
		return err
	}
	log.Printf("[vel] RefreshUsage: poll complete (%s)", scriptPath)
	return nil
}
