package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type App struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	License      string            `json:"license"`
	Vel          string            `json:"vel"`
	Panels       string            `json:"panels"`
	Routes       map[string]Route  `json:"routes"`
	Theme        string            `json:"theme"`
	DataSources  json.RawMessage   `json:"data_sources"`
	Tasks        json.RawMessage   `json:"tasks"`
	Capabilities json.RawMessage   `json:"capabilities"`
	Models       string            `json:"models"`
	Pages        string            `json:"pages"`

	Dir string `json:"-"`
}

type Route struct {
	Type string `json:"type"`
	Dir  string `json:"dir"`
}

type AppError struct {
	AppDir  string
	Message string
	Hint    string
}

func (e AppError) String() string {
	s := fmt.Sprintf("✗ %s", e.Message)
	if e.Hint != "" {
		s += fmt.Sprintf("\n  → %s", e.Hint)
	}
	return s
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func Discover(rootDir string) ([]*App, []AppError) {
	var apps []*App
	var errors []AppError

	appsDir := filepath.Join(rootDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		// Follow symlinks: check the resolved path
		info, err := os.Stat(filepath.Join(appsDir, entry.Name()))
		if err != nil || !info.IsDir() {
			continue
		}
		appDir := filepath.Join(appsDir, entry.Name())
		manifestPath := filepath.Join(appDir, "app.json")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// No app.json — skip silently
			continue
		}

		var app App
		if err := json.Unmarshal(data, &app); err != nil {
			errors = append(errors, AppError{
				AppDir:  entry.Name(),
				Message: fmt.Sprintf("App %q — app.json is not valid JSON: %s", entry.Name(), err),
			})
			continue
		}
		app.Dir = appDir

		var appErrors []AppError

		if app.Name == "" {
			appErrors = append(appErrors, AppError{
				AppDir:  entry.Name(),
				Message: fmt.Sprintf("App %q — app.json missing required field \"name\"", entry.Name()),
				Hint:    fmt.Sprintf("Add: \"name\": \"%s\"", entry.Name()),
			})
		} else if !nameRe.MatchString(app.Name) {
			appErrors = append(appErrors, AppError{
				AppDir:  entry.Name(),
				Message: fmt.Sprintf("App %q — name must be lowercase alphanumeric + hyphens", entry.Name()),
				Hint:    "Example: \"my-app\"",
			})
		}

		if app.Version == "" {
			appErrors = append(appErrors, AppError{
				AppDir:  entry.Name(),
				Message: fmt.Sprintf("App %q — app.json missing required field \"version\"", entry.Name()),
				Hint:    "Add: \"version\": \"1.0.0\"",
			})
		} else if !semverRe.MatchString(app.Version) {
			appErrors = append(appErrors, AppError{
				AppDir:  entry.Name(),
				Message: fmt.Sprintf("App %q — version %q doesn't look like semver", entry.Name(), app.Version),
				Hint:    "Use format: \"1.0.0\"",
			})
		}

		if app.Panels != "" {
			panelsDir := filepath.Join(appDir, app.Panels)
			if _, err := os.Stat(panelsDir); err != nil {
				appErrors = append(appErrors, AppError{
					AppDir:  entry.Name(),
					Message: fmt.Sprintf("App %q — panels directory %q not found", entry.Name(), app.Panels),
					Hint:    fmt.Sprintf("Create directory: %s", app.Panels),
				})
			}
		}

		if len(appErrors) > 0 {
			errors = append(errors, appErrors...)
			continue
		}

		apps = append(apps, &app)
	}

	return apps, errors
}
