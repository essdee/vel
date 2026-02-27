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
	"hash": true, "path": true, "regexp": true,
	// Encoding (no I/O)
	"encoding/json": true, "encoding/xml": true, "encoding/csv": true,
	"encoding/base64": true, "encoding/hex": true,
	// Crypto (no I/O)
	"crypto/sha256": true, "crypto/sha512": true, "crypto/md5": true,
	"crypto/hmac": true, "crypto/rand": true,
	// Framework
	"github.com/essdee/vel/pkg/vel": true,
}

// BlacklistedPackages are never allowed.
var BlacklistedPackages = map[string]bool{
	"os/exec":       true,
	"syscall":       true,
	"unsafe":        true,
	"plugin":        true,
	"reflect":       true,
	"runtime/debug": true,
	"os":            true, // all os.* is blacklisted — use ctx.*
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
	"read":  {"os"}, // via ctx wrappers
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
	Name     string
	Dir      string
	GoFiles  []string
	Manifest *AppManifest
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

	// Step 1: Discover apps with Go code
	appsDir := filepath.Join(opts.RootDir, "apps")
	appEntries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  No apps/ directory found. Nothing to build.")
			return nil
		}
		return fmt.Errorf("reading apps/: %w", err)
	}

	var apps []AppBuildInfo
	for _, entry := range appEntries {
		if !entry.IsDir() {
			continue
		}
		appDir := filepath.Join(appsDir, entry.Name())

		// Check for Go files
		goFiles, _ := filepath.Glob(filepath.Join(appDir, "*.go"))
		subGoFiles, _ := filepath.Glob(filepath.Join(appDir, "**", "*.go"))
		goFiles = append(goFiles, subGoFiles...)
		if len(goFiles) == 0 {
			continue
		}

		// Load app.json
		manifestData, err := os.ReadFile(filepath.Join(appDir, "app.json"))
		if err != nil {
			return fmt.Errorf("app %q: cannot read app.json: %w", entry.Name(), err)
		}
		var manifest AppManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return fmt.Errorf("app %q: invalid app.json: %w", entry.Name(), err)
		}

		apps = append(apps, AppBuildInfo{
			Name:     manifest.Name,
			Dir:      appDir,
			GoFiles:  goFiles,
			Manifest: &manifest,
		})
	}

	if len(apps) == 0 {
		fmt.Println("  No apps with Go code found. Nothing to build.")
		return nil
	}

	fmt.Printf("  Found %d app(s) with Go code\n", len(apps))
	for _, app := range apps {
		fmt.Printf("    → %s (%d .go files)\n", app.Name, len(app.GoFiles))
	}

	// Step 2: Parse Go AST and extract imports
	fmt.Println("\n  Scanning imports...")
	var allViolations []Violation

	for _, app := range apps {
		fset := token.NewFileSet()
		for _, goFile := range app.GoFiles {
			f, err := parser.ParseFile(fset, goFile, nil, parser.ImportsOnly)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", goFile, err)
			}

			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				pos := fset.Position(imp.Pos())
				relFile, _ := filepath.Rel(opts.RootDir, pos.Filename)

				// Check against tiers
				if Tier1Packages[importPath] {
					continue // always allowed
				}

				if BlacklistedPackages[importPath] {
					v := Violation{
						App:    app.Name,
						File:   relFile,
						Line:   pos.Line,
						Import: importPath,
						Reason: "blacklisted package",
					}
					allViolations = append(allViolations, v)
					continue
				}

				// Check if it's a standard library package (starts without a dot in first segment)
				if !strings.Contains(strings.Split(importPath, "/")[0], ".") {
					// Standard library, not in tier 1 or blacklist — check tier 2
					if !hasCapabilityForImport(app.Manifest, importPath) && opts.Mode == "strict" {
						v := Violation{
							App:    app.Name,
							File:   relFile,
							Line:   pos.Line,
							Import: importPath,
							Reason: "undeclared capability",
						}
						allViolations = append(allViolations, v)
					}
					continue
				}

				// Third-party package — check if declared in capabilities
				if !hasThirdPartyCapability(app.Manifest, importPath) && opts.Mode == "strict" {
					v := Violation{
						App:    app.Name,
						File:   relFile,
						Line:   pos.Line,
						Import: importPath,
						Reason: "undeclared third-party dependency",
					}
					allViolations = append(allViolations, v)
				}
			}
		}
	}

	// In bypass mode, log violations but don't fail
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

	// Step 4: Copy app Go files to _build/apps/
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
			data, err := os.ReadFile(goFile)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destFile, data, 0644); err != nil {
				return err
			}
		}
	}

	// Step 5: Generate _build/main.go
	fmt.Println("\n  Generating main.go...")
	mainGo := generateMainGo(apps, opts.RootDir)
	mainGoPath := filepath.Join(buildDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainGo), 0644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}

	// Step 6: Copy go.mod and go.sum to _build/
	for _, f := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(opts.RootDir, f)
		dst := filepath.Join(buildDir, f)
		data, err := os.ReadFile(src)
		if err != nil {
			if f == "go.sum" {
				continue // go.sum might not exist yet
			}
			return fmt.Errorf("reading %s: %w", f, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return err
		}
	}

	// Step 7: Run go build
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

	// Step 8: Clean _build/ unless --keep
	if !opts.Keep {
		os.RemoveAll(buildDir)
		fmt.Println("  Cleaned _build/")
	} else {
		fmt.Println("  Kept _build/ (--keep)")
	}

	fmt.Printf("\n  ✓ Build complete: ./%s\n\n", outputName)
	return nil
}

func generateMainGo(apps []AppBuildInfo, rootDir string) string {
	var b strings.Builder
	b.WriteString("// Code generated by vel build. DO NOT EDIT.\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")

	// Import app packages with blank identifier (for registration side effects)
	for _, app := range apps {
		// Use the module-relative import path
		b.WriteString(fmt.Sprintf("\t_ \"vel/apps/%s\"\n", app.Name))
	}

	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("\t// App packages are imported above for registration.\n")
	b.WriteString("\t// The vel framework handles startup via vel start.\n")
	b.WriteString("}\n")

	return b.String()
}

func hasCapabilityForImport(manifest *AppManifest, importPath string) bool {
	if manifest.Capabilities == nil {
		return false
	}
	// Check each tier 2 category
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
	// Check if the third-party package (or a prefix of it) is declared
	for cap := range manifest.Capabilities {
		if strings.Contains(cap, ".") { // third-party packages contain dots
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

// init suppresses unused import warnings
func init() {
	_ = ast.IsExported
}
