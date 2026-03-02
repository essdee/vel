package build

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options holds build command options.
type Options struct {
	RootDir string
	Mode    string // "strict" or "bypass"
	Output  string // output binary name
	Keep    bool   // keep _build/ directory
}

// Tier1Packages are always allowed (no declaration needed).
var Tier1Packages = map[string]bool{
	"fmt": true, "strings": true, "strconv": true, "math": true, "math/big": true,
	"time": true, "sort": true, "errors": true, "unicode": true, "unicode/utf8": true,
	"bytes": true, "context": true, "sync": true, "maps": true, "slices": true, "cmp": true,
	"log": true, "hash": true, "path": true, "regexp": true,
	"encoding/json": true, "encoding/xml": true, "encoding/csv": true,
	"encoding/base64": true, "encoding/hex": true,
	"crypto/sha256": true, "crypto/sha512": true, "crypto/md5": true,
	"crypto/hmac": true, "crypto/rand": true,
	"vel/pkg/vel": true,
}

// BlacklistedPackages are never allowed.
var BlacklistedPackages = map[string]bool{
	"os/exec":       true,
	"syscall":       true,
	"unsafe":        true,
	"plugin":        true,
	"reflect":       true,
	"runtime/debug": true,
	"os":            true,
}

// BlacklistedFunctions in otherwise-available packages.
var BlacklistedFunctions = map[string][]string{
	"runtime":  {"GOMAXPROCS"},
	"log":      {"Fatal", "Fatalf"},
	"net":      {"Listen"},
	"net/http": {"ListenAndServe", "ListenAndServeTLS"},
}

// Tier2Categories map capability names to the packages they unlock.
var Tier2Categories = map[string][]string{
	"read":  {"os"},
	"write": {"os"},
	"net":   {"net/http", "net/url"},
	"env":   {"os"},
	"db":    {"database/sql"},
}

// AppManifest represents the relevant fields from app.json.
type AppManifest struct {
	Name         string                     `json:"name"`
	Version      string                     `json:"version"`
	Capabilities map[string]json.RawMessage `json:"capabilities"`
}

// AppBuildInfo holds info about an app with Go code.
type AppBuildInfo struct {
	Name      string
	Dir       string
	GoFiles   []string
	Manifest  *AppManifest
	HasServer bool
}

// Violation represents an import policy violation.
type Violation struct {
	App    string
	File   string
	Line   int
	Import string
	Reason string
}

// Run executes the build process.
func Run(opts Options) error {
	fmt.Println("\n⚡ vel build")
	fmt.Printf("  Mode: %s\n", opts.Mode)

	// Load site config
	siteCfg, err := LoadSiteConfig(opts.RootDir)
	if err != nil {
		return fmt.Errorf("loading vel.yaml: %w", err)
	}

	// Step 1: Discover apps with Go code
	apps, err := discoverApps(opts.RootDir)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		fmt.Println("  No apps with Go code found. Nothing to build.")
		return nil
	}

	fmt.Printf("  Found %d app(s) with Go code\n", len(apps))
	for _, app := range apps {
		fmt.Printf("    → %s (%d .go files)\n", app.Name, len(app.GoFiles))
	}

	// Step 2: Scan imports, collect violations and tier 2 imports
	fmt.Println("\n  Scanning imports...")
	allViolations, tier2Imports, err := scanAppImports(apps, opts, siteCfg)
	if err != nil {
		return err
	}

	// Handle violations
	if len(allViolations) > 0 {
		if opts.Mode == "strict" {
			fmt.Printf("\n  ✗ %d violation(s) found:\n\n", len(allViolations))
			for _, v := range allViolations {
				fmt.Printf("    %s:%d    %s\n", v.File, v.Line, v.Import)
				fmt.Printf("      → %s (app: %s)\n\n", v.Reason, v.App)
			}
			return fmt.Errorf("build failed: %d import violation(s) in strict mode", len(allViolations))
		}
		fmt.Printf("\n  ⚠ %d violation(s) (bypass mode — logged only):\n", len(allViolations))
		for _, v := range allViolations {
			fmt.Printf("    %s:%d  %s — %s\n", v.File, v.Line, v.Import, v.Reason)
		}
	} else {
		fmt.Println("  ✓ All imports pass capability checks")
	}

	// Step 3: Set up _build/ directory
	buildDir := filepath.Join(opts.RootDir, "_build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("cleaning _build/: %w", err)
	}
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("creating _build/: %w", err)
	}

	// Step 4: Generate cap/* wrappers for tier 2 imports
	rewriteMap := make(map[string]string)
	if len(tier2Imports) > 0 {
		fmt.Println("\n  Generating capability wrappers...")
		rewriteMap, err = GenerateWrappers(buildDir, tier2Imports)
		if err != nil {
			return fmt.Errorf("generating wrappers: %w", err)
		}
		for orig, wrapper := range rewriteMap {
			fmt.Printf("    → %s → %s\n", orig, wrapper)
		}
	}

	// Step 5: Copy app Go files to _build/apps/ with AST rewriting
	for _, app := range apps {
		destDir := filepath.Join(buildDir, "apps", app.Name)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("creating build dir for %s: %w", app.Name, err)
		}
		for _, goFile := range app.GoFiles {
			relPath, _ := filepath.Rel(app.Dir, goFile)
			destFile := filepath.Join(destDir, relPath)
			if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
				return err
			}

			if len(rewriteMap) > 0 {
				if err := RewriteImports(goFile, destFile, rewriteMap); err != nil {
					return fmt.Errorf("rewriting imports in %s: %w", goFile, err)
				}
			} else {
				data, err := os.ReadFile(goFile)
				if err != nil {
					return err
				}
				if err := os.WriteFile(destFile, data, 0644); err != nil {
					return err
				}
			}
		}
	}

	// Step 6: Copy main.go and generate appimports.go
	fmt.Println("\n  Generating app imports...")
	// Copy the real main.go
	mainSrc, err := os.ReadFile(filepath.Join(opts.RootDir, "main.go"))
	if err != nil {
		return fmt.Errorf("reading main.go: %w", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "main.go"), mainSrc, 0644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}
	// Generate appimports.go with blank imports for app server packages
	appImports := generateAppImports(apps, opts)
	if err := os.WriteFile(filepath.Join(buildDir, "appimports.go"), []byte(appImports), 0644); err != nil {
		return fmt.Errorf("writing appimports.go: %w", err)
	}

	// Step 7: Copy go.mod and go.sum, update go.mod for wrapper module
	if err := setupBuildModule(opts.RootDir, buildDir); err != nil {
		return err
	}

	// Step 8: Run go build
	outputName := opts.Output
	if outputName == "" {
		outputName = "vel"
	}
	outputPath := filepath.Join(opts.RootDir, outputName)

	fmt.Printf("  Building binary → %s\n", outputName)
	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = buildDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	// Step 9: Clean _build/ unless --keep
	if !opts.Keep {
		os.RemoveAll(buildDir)
		fmt.Println("  Cleaned _build/")
	} else {
		fmt.Println("  Kept _build/ (--keep)")
	}

	fmt.Printf("\n  ✓ Build complete: ./%s\n\n", outputName)
	return nil
}

func discoverApps(rootDir string) ([]AppBuildInfo, error) {
	appsDir := filepath.Join(rootDir, "apps")
	appEntries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading apps/: %w", err)
	}

	var apps []AppBuildInfo
	for _, entry := range appEntries {
		// Follow symlinks when checking if entry is a directory
		appDir := filepath.Join(appsDir, entry.Name())
		info, err := os.Stat(appDir)
		if err != nil || !info.IsDir() {
			continue
		}

		goFiles, _ := filepath.Glob(filepath.Join(appDir, "*.go"))
		// Also look in server/ subdirectory
		serverGoFiles, _ := filepath.Glob(filepath.Join(appDir, "server", "*.go"))
		goFiles = append(goFiles, serverGoFiles...)
		// And any other subdirectories
		subGoFiles, _ := filepath.Glob(filepath.Join(appDir, "**", "*.go"))
		goFiles = append(goFiles, subGoFiles...)
		// Deduplicate
		seen := make(map[string]bool)
		var uniqueFiles []string
		for _, f := range goFiles {
			if !seen[f] {
				seen[f] = true
				uniqueFiles = append(uniqueFiles, f)
			}
		}
		goFiles = uniqueFiles
		if len(goFiles) == 0 {
			continue
		}

		manifestData, err := os.ReadFile(filepath.Join(appDir, "app.json"))
		if err != nil {
			return nil, fmt.Errorf("app %q: cannot read app.json: %w", entry.Name(), err)
		}
		var manifest AppManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return nil, fmt.Errorf("app %q: invalid app.json: %w", entry.Name(), err)
		}

		apps = append(apps, AppBuildInfo{
			Name:       manifest.Name,
			Dir:        appDir,
			GoFiles:    goFiles,
			Manifest:   &manifest,
			HasServer:  len(serverGoFiles) > 0,
		})
	}
	return apps, nil
}

func scanAppImports(apps []AppBuildInfo, opts Options, siteCfg *SiteConfig) ([]Violation, map[string]bool, error) {
	var allViolations []Violation
	tier2Imports := make(map[string]bool)

	// Build site block set
	siteBlocked := make(map[string]bool)
	for _, b := range siteCfg.Capabilities.Block {
		siteBlocked[b] = true
	}

	for _, app := range apps {
		// Merge site-level allows for this app
		siteAllows := make(map[string]bool)
		if appCfg, ok := siteCfg.Capabilities.Apps[app.Name]; ok {
			for _, a := range appCfg.Allow {
				siteAllows[a] = true
			}
		}

		fset := token.NewFileSet()
		for _, goFile := range app.GoFiles {
			f, err := parser.ParseFile(fset, goFile, nil, parser.ImportsOnly)
			if err != nil {
				return nil, nil, fmt.Errorf("parsing %s: %w", goFile, err)
			}

			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				pos := fset.Position(imp.Pos())
				relFile, _ := filepath.Rel(opts.RootDir, pos.Filename)

				if Tier1Packages[importPath] {
					continue
				}

				if BlacklistedPackages[importPath] {
					allViolations = append(allViolations, Violation{
						App: app.Name, File: relFile, Line: pos.Line,
						Import: importPath, Reason: "blacklisted package",
					})
					continue
				}

				// Check site-level blocks (function-level like "net/http.ListenAndServe")
				// These are checked at function-call level, not import level

				// Standard library (no dot in first segment)
				if !strings.Contains(strings.Split(importPath, "/")[0], ".") {
					// It's a tier 2 stdlib import
					tier2Imports[importPath] = true

					if !hasCapabilityForImport(app.Manifest, importPath) &&
						!siteAllows[importPath] &&
						opts.Mode == "strict" {
						allViolations = append(allViolations, Violation{
							App: app.Name, File: relFile, Line: pos.Line,
							Import: importPath, Reason: "undeclared capability",
						})
					}
					continue
				}

				// Third-party
				if !hasThirdPartyCapability(app.Manifest, importPath) && opts.Mode == "strict" {
					allViolations = append(allViolations, Violation{
						App: app.Name, File: relFile, Line: pos.Line,
						Import: importPath, Reason: "undeclared third-party dependency",
					})
				}
			}
		}
	}

	return allViolations, tier2Imports, nil
}

func generateAppImports(apps []AppBuildInfo, opts Options) string {
	var b strings.Builder
	b.WriteString("// Code generated by vel build. DO NOT EDIT.\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")

	for _, app := range apps {
		// Only import root package if there are root-level Go files
		hasRootGo := false
		for _, f := range app.GoFiles {
			rel, _ := filepath.Rel(app.Dir, f)
			if !strings.Contains(rel, string(filepath.Separator)) {
				hasRootGo = true
				break
			}
		}
		if hasRootGo {
			b.WriteString(fmt.Sprintf("\t_ \"vel/apps/%s\"\n", app.Name))
		}
		if app.HasServer {
			b.WriteString(fmt.Sprintf("\t_ \"vel/apps/%s/server\"\n", app.Name))
		}
	}

	b.WriteString(")\n")

	return b.String()
}

func setupBuildModule(rootDir, buildDir string) error {
	for _, f := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(rootDir, f)
		dst := filepath.Join(buildDir, f)
		data, err := os.ReadFile(src)
		if err != nil {
			if f == "go.sum" {
				continue
			}
			return fmt.Errorf("reading %s: %w", f, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return err
		}
	}

	// Symlink internal/ and pkg/ so app code can import vel packages
	for _, dir := range []string{"internal", "pkg"} {
		src := filepath.Join(rootDir, dir)
		dst := filepath.Join(buildDir, dir)
		if _, err := os.Stat(src); err == nil {
			if err := os.Symlink(src, dst); err != nil {
				return fmt.Errorf("symlinking %s: %w", dir, err)
			}
		}
	}

	return nil
}

func hasCapabilityForImport(manifest *AppManifest, importPath string) bool {
	if manifest.Capabilities == nil {
		return false
	}
	for cap, pkgs := range Tier2Categories {
		for _, pkg := range pkgs {
			if importPath == pkg || strings.HasPrefix(importPath, pkg+"/") {
				if _, ok := manifest.Capabilities[cap]; ok {
					return true
				}
			}
		}
	}
	return false
}

func hasThirdPartyCapability(manifest *AppManifest, importPath string) bool {
	if manifest.Capabilities == nil {
		return false
	}
	for cap := range manifest.Capabilities {
		if strings.Contains(cap, ".") {
			if importPath == cap || strings.HasPrefix(importPath, cap+"/") || strings.HasPrefix(importPath, cap) {
				return true
			}
		}
	}
	return false
}

// ScanImports extracts all imports from Go files in a directory.
func ScanImports(dir string) ([]string, error) {
	fset := token.NewFileSet()
	var imports []string
	seen := make(map[string]bool)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if !seen[p] {
				seen[p] = true
				imports = append(imports, p)
			}
		}
		return nil
	})
	return imports, err
}

// ScanDependencyImports scans third-party dependencies in the module cache
// for blacklisted imports. Returns violations for any blacklisted usage not
// covered by allow_blocked in the app manifest.
func ScanDependencyImports(app AppBuildInfo, goModCache string) []Violation {
	if app.Manifest.Capabilities == nil {
		return nil
	}

	var violations []Violation

	for capKey := range app.Manifest.Capabilities {
		if !strings.Contains(capKey, ".") {
			continue // not a third-party dep
		}

		// Find the dep in module cache
		depDir := findDepInCache(goModCache, capKey)
		if depDir == "" {
			continue
		}

		depCaps := ParseDepCapabilities(app.Manifest, capKey)
		allowBlocked := make(map[string]bool)
		if depCaps != nil {
			for _, ab := range depCaps.AllowBlocked {
				allowBlocked[ab] = true
			}
		}

		fset := token.NewFileSet()
		filepath.Walk(depDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return nil
			}
			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				if BlacklistedPackages[importPath] {
					// Check all blacklisted functions for this package
					// For fully blacklisted packages, check "pkg.*"
					key := importPath + ".*"
					if !allowBlocked[key] {
						// Also check if specific functions are allowed
						// For a fully blacklisted package, we need blanket allow or specific function allows
						pos := fset.Position(imp.Pos())
						relFile, _ := filepath.Rel(depDir, pos.Filename)
						violations = append(violations, Violation{
							App:    app.Name,
							File:   capKey + "/" + relFile,
							Line:   pos.Line,
							Import: importPath,
							Reason: fmt.Sprintf("blacklisted import in dependency %s", capKey),
						})
					}
				}
			}
			return nil
		})
	}

	return violations
}

func findDepInCache(goModCache, depPath string) string {
	// Try to find the dependency directory in the module cache
	// Module cache paths use ! for uppercase letters
	parts := strings.Split(depPath, "/")
	// Walk the cache looking for a match
	cacheDir := filepath.Join(goModCache, filepath.Join(parts...))

	// Check if directory exists (might have version suffix)
	matches, _ := filepath.Glob(cacheDir + "@*")
	if len(matches) > 0 {
		return matches[len(matches)-1] // latest version
	}

	// Try parent path for versioned modules (e.g., github.com/foo/bar/v3)
	if len(parts) > 0 {
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.HasPrefix(parts[i], "v") && len(parts[i]) >= 2 {
				parentPath := filepath.Join(goModCache, filepath.Join(parts[:i]...))
				matches, _ = filepath.Glob(parentPath + "@*")
				if len(matches) > 0 {
					return matches[len(matches)-1]
				}
			}
		}
	}

	return ""
}

// init suppresses unused import warnings
func init() {
	_ = ast.IsExported
}
