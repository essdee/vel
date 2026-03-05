package vel

import (
	"fmt"
	"os/exec"
)

// ReloadSecrets signals OpenClaw to re-resolve secret references
// by calling `openclaw secrets reload`. This is safe for apps to call
// after modifying auth-profiles or other secret stores.
func ReloadSecrets() error {
	out, err := exec.Command("openclaw", "secrets", "reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("secrets reload failed: %w — %s", err, string(out))
	}
	return nil
}
