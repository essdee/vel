package vel

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// RefreshUsage runs the claude-usage-poll.sh script synchronously.
// This is a framework-level function that apps can call since they
// don't have direct os/exec access in the sandboxed build.
func RefreshUsage() error {
	home, _ := os.UserHomeDir()

	// Try known script locations
	candidates := []string{
		filepath.Join(home, ".openclaw/workspace/skills/claude-usage-monitor/scripts/claude-usage-poll.sh"),
	}

	// Also check VEL_ROOT if set
	if root := os.Getenv("VEL_ROOT"); root != "" {
		candidates = append([]string{
			filepath.Join(root, "sdk", "openclaw", "claude-usage-poll.sh"),
		}, candidates...)
	}

	var scriptPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			scriptPath = p
			break
		}
	}

	if scriptPath == "" {
		log.Printf("[vel] RefreshUsage: no poll script found")
		return nil
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "HOME="+home)
	if err := cmd.Run(); err != nil {
		log.Printf("[vel] RefreshUsage: script failed: %v", err)
		return err
	}
	log.Printf("[vel] RefreshUsage: poll complete")
	return nil
}
