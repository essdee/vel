package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
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
	PanelOrder   []string          `json:"panelOrder"`
	LandingPage  string            `json:"landingPage"`

	Dir            string             `json:"-"`
	ParsedSources  []ParsedDataSource `json:"-"`
}

type Route struct {
	Type string `json:"type"`
	Dir  string `json:"dir"`
}

type FileDataSource struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Interval string `json:"interval"` // "2s", "10s", "1m"
}

type ParsedDataSource struct {
	Name     string
	Type     string
	Path     string
	Interval time.Duration
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

		// Parse data_sources
		if len(app.DataSources) > 0 {
			var rawSources map[string]FileDataSource
			if err := json.Unmarshal(app.DataSources, &rawSources); err != nil {
				appErrors = append(appErrors, AppError{
					AppDir:  entry.Name(),
					Message: fmt.Sprintf("App %q — data_sources is not a valid object: %s", entry.Name(), err),
				})
			} else {
				for dsName, ds := range rawSources {
					if ds.Type != "file" {
						appErrors = append(appErrors, AppError{
							AppDir:  entry.Name(),
							Message: fmt.Sprintf("App %q — data source %q has unsupported type %q", entry.Name(), dsName, ds.Type),
							Hint:    "Only \"file\" type is supported. Use Go code via vel build for other data collection.",
						})
						continue
					}
					if ds.Path == "" {
						appErrors = append(appErrors, AppError{
							AppDir:  entry.Name(),
							Message: fmt.Sprintf("App %q — data source %q missing required field \"path\"", entry.Name(), dsName),
						})
						continue
					}
					if ds.Interval == "" {
						appErrors = append(appErrors, AppError{
							AppDir:  entry.Name(),
							Message: fmt.Sprintf("App %q — data source %q missing required field \"interval\"", entry.Name(), dsName),
						})
						continue
					}
					dur, err := time.ParseDuration(ds.Interval)
					if err != nil {
						appErrors = append(appErrors, AppError{
							AppDir:  entry.Name(),
							Message: fmt.Sprintf("App %q — data source %q has invalid interval %q: %s", entry.Name(), dsName, ds.Interval, err),
							Hint:    "Use Go duration format: \"2s\", \"10s\", \"1m\"",
						})
						continue
					}
					if dur < time.Second {
						appErrors = append(appErrors, AppError{
							AppDir:  entry.Name(),
							Message: fmt.Sprintf("App %q — data source %q interval must be at least 1s, got %s", entry.Name(), dsName, ds.Interval),
						})
						continue
					}
					app.ParsedSources = append(app.ParsedSources, ParsedDataSource{
						Name:     dsName,
						Type:     ds.Type,
						Path:     ds.Path,
						Interval: dur,
					})
				}
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
