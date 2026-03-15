package data

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RepoStatus holds the update status for a single git repository.
type RepoStatus struct {
	Name          string `json:"name"`
	CurrentSHA    string `json:"currentSHA"`
	LatestSHA     string `json:"latestSHA,omitempty"`
	CommitsBehind int    `json:"commitsBehind"`
	UpToDate      bool   `json:"upToDate"`
	Branch        string `json:"branch,omitempty"`
	Error         string `json:"error,omitempty"`
}

// UpdatesStatus is the full update report for the framework + all apps.
type UpdatesStatus struct {
	Framework *RepoStatus   `json:"framework"`
	Apps      []*RepoStatus `json:"apps"`
	CheckedAt string        `json:"checkedAt"`
}

var (
	updatesCache   json.RawMessage
	updatesCacheAt time.Time
	updatesMu      sync.Mutex
)

func checkRepo(dir, name string) *RepoStatus {
	status := &RepoStatus{Name: name}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		// Check if dir is inside a parent git repo (e.g. workspace/.git covers vel-prod/)
		gitDirCmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
		if _, gitErr := gitDirCmd.Output(); gitErr != nil {
			status.Error = "not a git repo"
			return status
		}
	}

	// Detect branch
	branchCmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	if out, err := branchCmd.Output(); err == nil {
		status.Branch = strings.TrimSpace(string(out))
	} else {
		status.Branch = "main"
	}

	// Fetch from origin (best-effort — network may be unavailable)
	fetchCmd := exec.Command("git", "-C", dir, "fetch", "origin")
	fetchCmd.Run()

	// Current SHA
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		status.CurrentSHA = strings.TrimSpace(string(out))
	}

	// Remote SHA
	remoteRef := fmt.Sprintf("origin/%s", status.Branch)
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", remoteRef).Output(); err == nil {
		status.LatestSHA = strings.TrimSpace(string(out))
	}

	// Commits behind
	behindRef := fmt.Sprintf("HEAD..%s", remoteRef)
	if out, err := exec.Command("git", "-C", dir, "rev-list", behindRef, "--count").Output(); err == nil {
		n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		status.CommitsBehind = n
	}

	status.UpToDate = status.CommitsBehind == 0
	return status
}

// CheckUpdates runs a live git check for the framework and all apps.
func CheckUpdates(prodDir string) *UpdatesStatus {
	result := &UpdatesStatus{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Framework repo may be in vel/ subdirectory (Decision 016 layout)
	frameworkDir := prodDir
	if _, err := os.Stat(filepath.Join(prodDir, "vel", ".git")); err == nil {
		frameworkDir = filepath.Join(prodDir, "vel")
	}
	result.Framework = checkRepo(frameworkDir, "vel")

	// Check apps from both apps/ subdirectory and VEL_APPS
	appsDirs := []string{filepath.Join(prodDir, "apps")}
	if externalDir := os.Getenv("VEL_APPS"); externalDir != "" {
		appsDirs = append(appsDirs, externalDir)
	}

	seen := make(map[string]bool)
	for _, appsDir := range appsDirs {
		entries, err := os.ReadDir(appsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			appDir := filepath.Join(appsDir, entry.Name())
			if _, err := os.Stat(filepath.Join(appDir, ".git")); err != nil {
				continue // not a git repo, skip
			}
			seen[entry.Name()] = true
			result.Apps = append(result.Apps, checkRepo(appDir, entry.Name()))
		}
	}

	return result
}

// GetUpdatesStatus runs a live check, caches for 10 minutes, returns raw JSON.
func GetUpdatesStatus(prodDir string) json.RawMessage {
	updatesMu.Lock()
	defer updatesMu.Unlock()

	status := CheckUpdates(prodDir)
	raw, _ := json.Marshal(status)
	updatesCache = raw
	updatesCacheAt = time.Now()
	return updatesCache
}

// GetUpdatesCached returns cached data without blocking.
// Returns nil if cache is cold or expired — caller may trigger an async refresh.
func GetUpdatesCached(prodDir string) json.RawMessage {
	updatesMu.Lock()
	defer updatesMu.Unlock()

	if time.Since(updatesCacheAt) < 10*time.Minute && updatesCache != nil {
		return updatesCache
	}

	// Trigger async refresh
	go func() {
		GetUpdatesStatus(prodDir)
	}()
	return nil
}

// InvalidateUpdatesCache forces a fresh check on the next call.
func InvalidateUpdatesCache() {
	updatesMu.Lock()
	defer updatesMu.Unlock()
	updatesCacheAt = time.Time{}
}
