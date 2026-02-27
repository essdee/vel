package build

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CapsListAll lists capabilities for all apps.
func CapsListAll(rootDir string) error {
	apps, err := discoverApps(rootDir)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		fmt.Println("No apps with Go code found.")
		return nil
	}
	for _, app := range apps {
		printAppCaps(app)
	}
	return nil
}

// CapsListApp lists capabilities for a specific app.
func CapsListApp(rootDir, appName string) error {
	apps, err := discoverApps(rootDir)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if app.Name == appName {
			printAppCaps(app)
			return nil
		}
	}
	return fmt.Errorf("app %q not found", appName)
}

func printAppCaps(app AppBuildInfo) {
	fmt.Printf("\n📦 %s\n", app.Name)
	if app.Manifest.Capabilities == nil || len(app.Manifest.Capabilities) == 0 {
		fmt.Println("  (no capabilities declared)")
		return
	}

	// Sort keys for consistent output
	var keys []string
	for k := range app.Manifest.Capabilities {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		raw := app.Manifest.Capabilities[k]
		// Try to pretty-print the value
		var val interface{}
		if err := json.Unmarshal(raw, &val); err == nil {
			switch v := val.(type) {
			case bool:
				fmt.Printf("  %-20s %v\n", k, v)
			case []interface{}:
				items := make([]string, len(v))
				for i, item := range v {
					items[i] = fmt.Sprint(item)
				}
				fmt.Printf("  %-20s [%s]\n", k, strings.Join(items, ", "))
			case map[string]interface{}:
				data, _ := json.Marshal(v)
				fmt.Printf("  %-20s %s\n", k, string(data))
			default:
				fmt.Printf("  %-20s %s\n", k, string(raw))
			}
		} else {
			fmt.Printf("  %-20s %s\n", k, string(raw))
		}
	}
}

// CapsExport generates a capabilities block from bypass-mode usage logs.
// For now, it scans the app's Go files and generates a capabilities block
// based on which tier 2 imports are used.
func CapsExport(rootDir, appName string) error {
	apps, err := discoverApps(rootDir)
	if err != nil {
		return err
	}

	var targetApp *AppBuildInfo
	for _, app := range apps {
		if app.Name == appName {
			a := app
			targetApp = &a
			break
		}
	}
	if targetApp == nil {
		return fmt.Errorf("app %q not found", appName)
	}

	// Scan all imports from the app's Go files
	tier2Used := make(map[string]bool)
	thirdPartyUsed := make(map[string]bool)

	for _, goFile := range targetApp.GoFiles {
		imports, err := scanFileImports(goFile)
		if err != nil {
			return err
		}
		for _, imp := range imports {
			if Tier1Packages[imp] || BlacklistedPackages[imp] {
				continue
			}
			if !strings.Contains(strings.Split(imp, "/")[0], ".") {
				tier2Used[imp] = true
			} else {
				thirdPartyUsed[imp] = true
			}
		}
	}

	// Also check for usage log file from bypass mode
	logFile := filepath.Join(rootDir, "_build", "cap-usage.log")
	if data, err := os.ReadFile(logFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				tier2Used[parts[0]] = true
			}
		}
	}

	// Generate capabilities block
	caps := make(map[string]interface{})

	// Map tier 2 imports back to capability categories
	for imp := range tier2Used {
		for capName, pkgs := range Tier2Categories {
			for _, pkg := range pkgs {
				if imp == pkg || strings.HasPrefix(imp, pkg+"/") {
					caps[capName] = true
				}
			}
		}
	}

	// Add third-party deps
	for imp := range thirdPartyUsed {
		// Find the root module path (simplistic: use first 3 segments for github.com)
		parts := strings.Split(imp, "/")
		modPath := imp
		if len(parts) >= 3 && parts[0] == "github.com" {
			modPath = strings.Join(parts[:3], "/")
		}
		caps[modPath] = map[string]interface{}{}
	}

	// Output as JSON
	data, err := json.MarshalIndent(map[string]interface{}{"capabilities": caps}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("\nGenerated capabilities for %s:\n\n%s\n\n", appName, string(data))
	fmt.Println("Copy the \"capabilities\" block into your app.json.")
	return nil
}

func scanFileImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, imp := range f.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports, nil
}
