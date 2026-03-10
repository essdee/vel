package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vel/internal/apps"
	"vel/internal/auth"
	"vel/internal/build"
	"vel/internal/datasource"
	"vel/internal/hooks"
	"vel/internal/panels"
	"vel/internal/server"
	"vel/internal/verify"
	vel "vel/pkg/vel"
)

const Version = "0.1.0"

type Card struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type AppConfig struct {
	// Landing page
	Name         string   `json:"name"`
	Emoji        string   `json:"emoji"`
	Subtitle     string   `json:"subtitle"`
	Role         string   `json:"role"`
	Quote        string   `json:"quote"`
	Traits       []string `json:"traits"`
	Cards        []Card   `json:"cards"`
	Accent       string   `json:"accent"`
	AccentName   string   `json:"accentName"`
	Company      string   `json:"company"`
	BotUsername  string   `json:"botUsername"`
	AuthURL      string   `json:"authUrl"`
	TelegramLink string   `json:"telegramLink"`

	// Auth
	Auth struct {
		AllowedTelegramUsers []int64             `json:"allowedTelegramUsers"`
		Mode                 string              `json:"mode"`
		Token                string              `json:"token"`
		Tokens               []auth.ScopedToken  `json:"tokens,omitempty"`
	} `json:"auth"`
	AllowedUsers []int64 `json:"allowedUsers"` // legacy (use auth.allowedTelegramUsers)

	// Panels
	Panels struct {
		Order    []string `json:"order"`
		Disabled []string `json:"disabled"`
	} `json:"panels"`

	// Server
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
	Port int `json:"port"` // legacy field

	// Environment
	Staging bool `json:"staging"` // true = staging instance, disables production-only features
}

func main() {
	if len(os.Args) < 2 {
		// Default: start server (backward compatible)
		runStart(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "start":
		runStart(os.Args[2:])
	case "build":
		runBuild(os.Args[2:])
	case "caps":
		runCaps(os.Args[2:])
	case "verify":
		runVerify(os.Args[2:])
	case "test":
		runTest(os.Args[2:])
	case "version":
		fmt.Printf("vel %s\n", Version)
	case "help", "--help", "-h":
		printHelp()
	default:
		// If it looks like a flag, treat as start
		if strings.HasPrefix(os.Args[1], "-") {
			runStart(os.Args[1:])
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`vel — AI-native framework for real-time web apps

Usage:
  vel [command] [flags]

Commands:
  start     Start the server (default if no command given)
  build     Scan apps, check capabilities, compile binary
  caps      List or export app capabilities
  verify    Run health checks on the current installation
  test      Run tests with fixture data
  version   Print version
  help      Show this help

Run 'vel <command> --help' for command-specific flags.`)
}

func runVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	jsonMode := fs.Bool("json", false, "Output JSON and write verify-log.json")
	verbose := fs.Bool("verbose", false, "Show all checks including passed ones")
	fs.Parse(args)

	rootDir, _ := os.Getwd()

	if !*jsonMode {
		fmt.Print("\n⚡ Vel Health Check\n\n")
	}

	// Discover apps
	discoveredApps, _ := apps.Discover(rootDir)

	// Build panel app list
	var panelApps []panels.AppInfo
	for _, a := range discoveredApps {
		panelApps = append(panelApps, panels.AppInfo{Name: a.Name, Panels: a.Panels, Dir: a.Dir})
	}

	// Discover panels
	registry, _ := panels.DiscoverPanels(rootDir, panelApps)

	cfg := verify.VerifyConfig{
		RootDir:  rootDir,
		Apps:     discoveredApps,
		Registry: registry,
	}

	result := verify.RunVerify(cfg)

	// ── JSON mode ──
	if *jsonMode {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))

		// Write to logs/verify.jsonl (create logs/ dir if needed).
		logsDir := filepath.Join(rootDir, "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "verify.jsonl")
		os.WriteFile(logPath, out, 0644)

		if result.Failed > 0 {
			os.Exit(1)
		}
		return
	}

	// ── Human-readable output ──
	// Group checks by category
	var coreChecks, panelChecks, dataChecks, frameworkChecks, authProbeChecks, endpointChecks, appVerifyChecks, otherChecks []verify.CheckResult

	for _, c := range result.Checks {
		switch {
		case strings.HasPrefix(c.Name, "panel:"):
			panelChecks = append(panelChecks, c)
		case strings.HasPrefix(c.Name, "data:"):
			dataChecks = append(dataChecks, c)
		case c.Name == "server.reachable":
			frameworkChecks = append(frameworkChecks, c)
		case c.Name == "auth.probe":
			authProbeChecks = append(authProbeChecks, c)
		case strings.HasPrefix(c.Name, "endpoint:"):
			endpointChecks = append(endpointChecks, c)
		case strings.HasPrefix(c.Name, "app:"):
			appVerifyChecks = append(appVerifyChecks, c)
		case c.Name == "config" || c.Name == "auth" || c.Name == "openclaw-cli" ||
			strings.HasPrefix(c.Name, "auth."):
			coreChecks = append(coreChecks, c)
		default:
			otherChecks = append(otherChecks, c)
		}
	}

	printCheck := func(c verify.CheckResult) {
		// In normal mode, skip passed checks unless --verbose
		if !*verbose && c.Status == "ok" {
			return
		}

		label := c.Name
		// Strip prefixes for display
		label = strings.TrimPrefix(label, "panel:")
		label = strings.TrimPrefix(label, "data:")
		label = strings.TrimPrefix(label, "endpoint:")
		label = strings.TrimPrefix(label, "app:")

		var detail string
		if c.Detail != "" {
			detail = " — " + c.Detail
		}

		switch c.Status {
		case "ok":
			fmt.Printf("  ✓ %s%s\n", label, detail)
		case "skipped":
			fmt.Printf("  ○ %s%s\n", label, detail)
		default:
			fmt.Printf("  ✗ %s%s\n", label, detail)
			if c.Hint != "" {
				fmt.Printf("    💡 %s\n", c.Hint)
			}
		}
	}

	// Core checks
	fmt.Println("  Core:")
	for _, c := range coreChecks {
		printCheck(c)
	}

	// Framework (server up)
	if len(frameworkChecks) > 0 {
		fmt.Println("\n  Framework:")
		for _, c := range frameworkChecks {
			printCheck(c)
		}
	}

	// Auth probe
	if len(authProbeChecks) > 0 {
		fmt.Println("\n  Auth probe:")
		for _, c := range authProbeChecks {
			printCheck(c)
		}
	}

	// Panels
	if len(panelChecks) > 0 {
		fmt.Println("\n  Panels:")
		for _, c := range panelChecks {
			printCheck(c)
		}
	}

	// Data sources
	if len(dataChecks) > 0 {
		fmt.Println("\n  Data sources:")
		for _, c := range dataChecks {
			printCheck(c)
		}
	}

	// Endpoint checks
	if len(endpointChecks) > 0 {
		fmt.Println("\n  Endpoints:")
		for _, c := range endpointChecks {
			printCheck(c)
		}
	}

	// App verify.json checks
	if len(appVerifyChecks) > 0 {
		fmt.Println("\n  App checks (verify.json):")
		for _, c := range appVerifyChecks {
			printCheck(c)
		}
	}

	// Other (app-registered health checks)
	if len(otherChecks) > 0 {
		fmt.Println("\n  App health checks:")
		for _, c := range otherChecks {
			printCheck(c)
		}
	}

	fmt.Printf("\n  %d passed, %d failed, %d skipped\n\n", result.Passed, result.Failed, result.Skipped)

	if result.Failed > 0 {
		os.Exit(1)
	}
}

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	mode := fs.String("mode", "strict", "Build mode: strict (default) or bypass")
	output := fs.String("output", "vel", "Output binary name")
	keep := fs.Bool("keep", false, "Keep _build/ directory for debugging")
	fs.Parse(args)

	rootDir, _ := os.Getwd()

	opts := build.Options{
		RootDir: rootDir,
		Mode:    *mode,
		Output:  *output,
		Keep:    *keep,
	}

	if err := build.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ %s\n\n", err)
		os.Exit(1)
	}
}

func runCaps(args []string) {
	rootDir, _ := os.Getwd()

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: vel caps <list|export> [app]\n")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		var err error
		if len(args) > 1 {
			err = build.CapsListApp(rootDir, args[1])
		} else {
			err = build.CapsListAll(rootDir)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	case "export":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: vel caps export <app>\n")
			os.Exit(1)
		}
		if err := build.CapsExport(rootDir, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown caps command: %s\nUsage: vel caps <list|export> [app]\n", args[0])
		os.Exit(1)
	}
}

// testCheckResult holds the outcome of a single test check.
type testCheckResult struct {
	label  string
	passed bool
}

// testFixtureResult holds results for one app+fixture combination.
type testFixtureResult struct {
	app     string
	fixture string
	checks  []testCheckResult
}

func (r *testFixtureResult) passed() int {
	n := 0
	for _, c := range r.checks {
		if c.passed {
			n++
		}
	}
	return n
}

func (r *testFixtureResult) failed() int {
	n := 0
	for _, c := range r.checks {
		if !c.passed {
			n++
		}
	}
	return n
}

func runTest(args []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	fs.Parse(args)

	rootDir, _ := os.Getwd()

	fmt.Printf("\n⚡ Vel Test Runner\n\n")

	// Discover apps
	discoveredApps, _ := apps.Discover(rootDir)
	if len(discoveredApps) == 0 {
		fmt.Println("  No apps found.")
		os.Exit(0)
	}

	// Standard fixture names to check
	allFixtures := []string{"default", "empty", "stress", "demo"}

	var results []testFixtureResult
	allPassed := true

	for _, a := range discoveredApps {
		testdataDir := filepath.Join(a.Dir, "testdata")
		if _, err := os.Stat(testdataDir); err != nil {
			fmt.Printf("  ⚠ %s — no testdata/ directory, skipping\n", a.Name)
			continue
		}

		// Find which fixture directories exist
		var availableFixtures []string
		for _, f := range allFixtures {
			if _, err := os.Stat(filepath.Join(testdataDir, f)); err == nil {
				availableFixtures = append(availableFixtures, f)
			}
		}

		if len(availableFixtures) == 0 {
			fmt.Printf("  ⚠ %s — testdata/ exists but no fixture directories found\n", a.Name)
			continue
		}

		for _, fixture := range availableFixtures {
			result := testFixtureResult{app: a.Name, fixture: fixture}

			// Start a fresh test server for this fixture
			port, stop, err := startTestServer(rootDir, discoveredApps, fixture)
			if err != nil {
				result.checks = append(result.checks, testCheckResult{
					label:  fmt.Sprintf("start server: %v", err),
					passed: false,
				})
				results = append(results, result)
				allPassed = false
				continue
			}

			base := fmt.Sprintf("http://localhost:%d", port)

			// Poll /api/health until server is ready (up to 4 seconds)
			ready := false
			for i := 0; i < 20; i++ {
				resp, err := http.Get(base + "/api/health")
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					ready = true
					break
				}
				if resp != nil {
					resp.Body.Close()
				}
				time.Sleep(200 * time.Millisecond)
			}

			if !ready {
				result.checks = append(result.checks, testCheckResult{
					label:  "server not ready (health check timed out after 4s)",
					passed: false,
				})
				stop()
				results = append(results, result)
				allPassed = false
				continue
			}

			// Check 1: /api/health → 200
			resp, err := http.Get(base + "/api/health")
			if err != nil {
				result.checks = append(result.checks, testCheckResult{label: fmt.Sprintf("GET /api/health: %v", err), passed: false})
			} else {
				ok := resp.StatusCode == 200
				resp.Body.Close()
				result.checks = append(result.checks, testCheckResult{
					label:  fmt.Sprintf("GET /api/health → %d", resp.StatusCode),
					passed: ok,
				})
			}

			// Check 2: /dashboard → 200 with HTML
			resp, err = http.Get(base + "/dashboard")
			if err != nil {
				result.checks = append(result.checks, testCheckResult{label: fmt.Sprintf("GET /dashboard: %v", err), passed: false})
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				isHTML := strings.Contains(string(body), "<html") || strings.Contains(string(body), "<!DOCTYPE")
				ok := resp.StatusCode == 200 && isHTML
				label := fmt.Sprintf("GET /dashboard → %d", resp.StatusCode)
				if resp.StatusCode == 200 && !isHTML {
					label += " (not HTML)"
				}
				result.checks = append(result.checks, testCheckResult{label: label, passed: ok})
			}

			// Check 3: first route of each app → not 500
			for _, appToCheck := range discoveredApps {
				for routePath := range appToCheck.Routes {
					resp, err := http.Get(base + routePath)
					if err != nil {
						result.checks = append(result.checks, testCheckResult{
							label:  fmt.Sprintf("GET %s (%s): %v", routePath, appToCheck.Name, err),
							passed: false,
						})
					} else {
						resp.Body.Close()
						ok := resp.StatusCode < 500
						result.checks = append(result.checks, testCheckResult{
							label:  fmt.Sprintf("GET %s (%s) → %d", routePath, appToCheck.Name, resp.StatusCode),
							passed: ok,
						})
					}
					break // only first route per app
				}
			}

			stop()
			results = append(results, result)
		}
	}

	// Print results
	fmt.Printf("\n  Results:\n\n")
	for _, r := range results {
		fmt.Printf("  [%s / fixture: %s]\n", r.app, r.fixture)
		for _, c := range r.checks {
			if c.passed {
				fmt.Printf("    ✓ %s\n", c.label)
			} else {
				fmt.Printf("    ✗ %s\n", c.label)
			}
		}
		fmt.Printf("    → %d passed, %d failed\n\n", r.passed(), r.failed())
		if r.failed() > 0 {
			allPassed = false
		}
	}

	if len(results) == 0 {
		fmt.Println("  No testable apps found (add testdata/ directories to apps)")
		os.Exit(0)
	}

	if allPassed {
		fmt.Println("  ✓ All tests passed")
	} else {
		fmt.Println("  ✗ Some tests failed")
		os.Exit(1)
	}
}

// startTestServer starts a minimal vel server on a random port with the given fixture.
// Returns the port, a stop function, and any error.
func startTestServer(rootDir string, discoveredApps []*apps.App, fixture string) (int, func(), error) {
	vel.SetTestMode(true, fixture)

	// Use "none" auth for testing
	auth.Init("", nil, "vel-test-cookie-secret-not-for-production")
	auth.InitMode("none", "")
	auth.InitScopedTokens(nil)

	hookEngine := hooks.New()

	// Create datasource manager and register sources
	dsManager := datasource.NewManager()
	for _, a := range discoveredApps {
		for _, ds := range a.ParsedSources {
			// Ignore errors — fixture files may not exist for every source
			_ = dsManager.AddFileSource(a.Name, a.Dir, ds.Name, ds.Path, ds.Interval)
		}
	}

	// Build panel app list and discover panels
	var panelApps []panels.AppInfo
	for _, a := range discoveredApps {
		panelApps = append(panelApps, panels.AppInfo{Name: a.Name, Panels: a.Panels, Dir: a.Dir})
	}
	registry, _ := panels.DiscoverPanels(rootDir, panelApps)

	cfg := &server.Config{
		RootDir:      rootDir,
		Workspace:    filepath.Dir(rootDir),
		ConfigPath:   filepath.Join(rootDir, "config.json"),
		Port:         0,
		Registry:     registry,
		Order:        []string{},
		Disabled:     []string{},
		Version:      Version,
		PublicConfig: map[string]interface{}{"authMode": "none"},
		Apps:         discoveredApps,
		Hooks:        hookEngine,
		DSManager:    dsManager,
	}

	handler := server.NewServer(cfg)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		vel.SetTestMode(false, "")
		return 0, nil, fmt.Errorf("could not bind port: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port

	dsManager.Start()
	go http.Serve(ln, handler) //nolint:errcheck

	stop := func() {
		ln.Close()
		dsManager.Stop()
		vel.SetTestMode(false, "")
	}

	return port, stop, nil
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	portFlag := fs.Int("port", 0, "Override server port")
	testModeFlag := fs.Bool("test-mode", false, "Run with test fixture data")
	fixtureFlag := fs.String("fixture", "default", "Fixture set to use (default, empty, stress, demo)")
	demoFlag := fs.Bool("demo", false, "Shortcut for --test-mode --fixture=demo")
	fs.Parse(args)

	rootDir, _ := os.Getwd()

	// Resolve test mode / demo shortcut
	testMode := *testModeFlag
	fixtureName := *fixtureFlag
	if *demoFlag {
		testMode = true
		fixtureName = "demo"
	}

	// TEST_MODE env var warning (legacy)
	if os.Getenv("TEST_MODE") == "true" {
		fmt.Println("\n⚠️  TEST_MODE is enabled — auth bypassed")
		fmt.Println("⚠️  Do NOT use in production.")
	}

	// Apply test mode state
	if testMode {
		vel.SetTestMode(true, fixtureName)
		fmt.Printf("\n⚡ Vel — TEST MODE\n")
		fmt.Printf("  Fixture: %s\n", fixtureName)
		fmt.Printf("  Data sources redirected to testdata/%s/\n", fixtureName)
		fmt.Printf("\n  ⚠️  Not for production use\n\n")
	}

	// Load config
	configPath := filepath.Join(rootDir, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("[Config] Failed to load config.json: %s\nCopy config.example.json to config.json and configure it", err)
	}
	var config AppConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("[Config] Invalid config.json: %s", err)
	}
	fmt.Println("[Config] Loaded config.json")

	// Set staging flag in SDK so apps can check vel.IsStaging()
	vel.SetStaging(config.Staging)
	if config.Staging {
		fmt.Println("[Config] Running in STAGING mode")
	}

	// BOT_TOKEN
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		// Try .env file
		envData, err := os.ReadFile(filepath.Join(rootDir, ".env"))
		if err == nil {
			for _, line := range strings.Split(string(envData), "\n") {
				if strings.HasPrefix(line, "BOT_TOKEN=") {
					botToken = strings.TrimPrefix(line, "BOT_TOKEN=")
					botToken = strings.TrimSpace(botToken)
				}
			}
		}
	}

	// Determine auth mode
	authMode := config.Auth.Mode
	if authMode == "" {
		// Auto-detect mode
		if botToken != "" {
			authMode = "telegram"
		} else if config.Auth.Token != "" {
			authMode = "token"
		} else {
			authMode = "none"
		}
	}

	if authMode == "telegram" && botToken == "" {
		log.Fatal("[Fatal] BOT_TOKEN is required for telegram auth mode")
	}

	fmt.Printf("[Auth] Mode: %s\n", authMode)

	// Cookie secret
	cookieSecretFile := filepath.Join(rootDir, ".cookie-secret")
	cookieSecret, err := os.ReadFile(cookieSecretFile)
	if err != nil {
		secret := make([]byte, 32)
		rand.Read(secret)
		secretStr := hex.EncodeToString(secret)
		os.WriteFile(cookieSecretFile, []byte(secretStr), 0600)
		cookieSecret = []byte(secretStr)
		fmt.Println("[Auth] Generated new cookie secret")
	}

	// Merge allowed users
	allowedUsers := config.Auth.AllowedTelegramUsers
	if len(allowedUsers) == 0 {
		allowedUsers = config.AllowedUsers
	}

	// Init auth
	auth.Init(botToken, allowedUsers, strings.TrimSpace(string(cookieSecret)))
	auth.InitMode(authMode, config.Auth.Token)
	auth.InitScopedTokens(config.Auth.Tokens)

	// Port: flag > env PORT > config.server.port > config.port > 3700
	port := *portFlag
	if port == 0 {
		if envPort := os.Getenv("PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				port = p
			}
		}
	}
	if port == 0 {
		port = config.Server.Port
	}
	if port == 0 {
		port = config.Port
	}
	if port == 0 {
		port = 3700
	}

	// Workspace
	workspace := os.Getenv("WORKSPACE")
	if workspace == "" {
		workspace = filepath.Dir(rootDir)
	}

	// Init hooks
	hookEngine := hooks.New()
	hookEngine.Emit("core.server.init")

	// Discover apps
	discoveredApps, appErrors := apps.Discover(rootDir)
	fmt.Printf("\n┌─ App Report ──────────────────────────\n")
	fmt.Printf("│ Loaded: %d\n", len(discoveredApps))
	for _, a := range discoveredApps {
		label := a.Name + " v" + a.Version
		if a.Title != "" {
			label += fmt.Sprintf(" — %q", a.Title)
		}
		fmt.Printf("│   ✓ %s\n", label)
	}
	if len(appErrors) > 0 {
		fmt.Printf("│ Errors: %d\n", len(appErrors))
		for _, e := range appErrors {
			fmt.Printf("│   %s\n", e)
		}
	}
	fmt.Printf("└────────────────────────────────────────\n")

	// Create datasource manager and register file sources
	dsManager := datasource.NewManager()
	dsCount := 0
	for _, a := range discoveredApps {
		for _, ds := range a.ParsedSources {
			if err := dsManager.AddFileSource(a.Name, a.Dir, ds.Name, ds.Path, ds.Interval); err != nil {
				fmt.Printf("│   ✗ Data source %s:%s — %s\n", a.Name, ds.Name, err)
			} else {
				dsCount++
			}
		}
	}
	if dsCount > 0 {
		fmt.Printf("\n┌─ Data Sources ────────────────────────\n")
		fmt.Printf("│ Registered: %d file source(s)\n", dsCount)
		for key, state := range dsManager.GetAllData() {
			status := "ready"
			if !state.OK && state.Data == nil {
				status = "waiting for file"
			}
			fmt.Printf("│   ✓ %s (%s, every %s) — %s\n", key, state.Path, state.Interval, status)
		}
		fmt.Printf("└────────────────────────────────────────\n")
		dsManager.Start()
	}

	// Build panel app list
	var panelApps []panels.AppInfo
	for _, a := range discoveredApps {
		panelApps = append(panelApps, panels.AppInfo{Name: a.Name, Panels: a.Panels, Dir: a.Dir})
	}

	// Discover panels
	fmt.Println("\n[Panels] Discovering panels...")
	registry, report := panels.DiscoverPanels(rootDir, panelApps)

	fmt.Printf("\n┌─ Panel Report ────────────────────────\n")
	fmt.Printf("│ Loaded: %d\n", len(report.Loaded))
	for _, p := range report.Loaded {
		fmt.Printf("│   ✓ %s (%s) v%s\n", p.ID, p.Source, p.Version)
	}
	if len(report.Skipped) > 0 {
		fmt.Printf("│ Legacy (no contract): %d\n", len(report.Skipped))
		for _, p := range report.Skipped {
			fmt.Printf("│   ⚠ %s (%s) — %s\n", p.ID, p.Source, p.Reason)
		}
	}
	if len(report.Failed) > 0 {
		fmt.Printf("│ Failed: %d\n", len(report.Failed))
		for _, p := range report.Failed {
			fmt.Printf("│   ✗ %s (%s) — %s\n", p.ID, p.Source, strings.Join(p.Errors, ", "))
		}
	}
	fmt.Printf("└────────────────────────────────────────\n\n")
	hookEngine.Emit("core.panels.discovered", registry, report)

	// Load version
	version := Version
	versionFile := filepath.Join(rootDir, ".version")
	if vData, err := os.ReadFile(versionFile); err == nil {
		var vInfo map[string]interface{}
		if json.Unmarshal(vData, &vInfo) == nil {
			if v, ok := vInfo["version"].(string); ok {
				version = v
			}
		}
	}

	// Build public config (safe to expose — no tokens, no secrets)
	publicConfig := map[string]interface{}{
		"authMode": authMode,
	}
	if config.Name != "" {
		publicConfig["name"] = config.Name
	}
	if config.Emoji != "" {
		publicConfig["emoji"] = config.Emoji
	}
	if config.Subtitle != "" {
		publicConfig["subtitle"] = config.Subtitle
	}
	if config.Role != "" {
		publicConfig["role"] = config.Role
	}
	if config.Quote != "" {
		publicConfig["quote"] = config.Quote
	}
	if len(config.Traits) > 0 {
		publicConfig["traits"] = config.Traits
	}
	if len(config.Cards) > 0 {
		publicConfig["cards"] = config.Cards
	}
	if config.Accent != "" {
		publicConfig["accent"] = config.Accent
	}
	if config.AccentName != "" {
		publicConfig["accentName"] = config.AccentName
	}
	if config.Company != "" {
		publicConfig["company"] = config.Company
	}
	if config.BotUsername != "" {
		publicConfig["botUsername"] = config.BotUsername
		publicConfig["authUrl"] = config.AuthURL
		publicConfig["telegramLink"] = config.TelegramLink
	}

	// Ensure non-nil slices (avoid null in JSON)
	// Priority: config.json panels.order > app.json panelOrder > empty
	panelOrder := config.Panels.Order
	if len(panelOrder) == 0 {
		// Fallback: collect panelOrder from discovered apps
		for _, a := range discoveredApps {
			if len(a.PanelOrder) > 0 {
				panelOrder = append(panelOrder, a.PanelOrder...)
			}
		}
	}
	if panelOrder == nil {
		panelOrder = []string{}
	}
	panelDisabled := config.Panels.Disabled
	if panelDisabled == nil {
		panelDisabled = []string{}
	}

	cfg := &server.Config{
		RootDir:      rootDir,
		Workspace:    workspace,
		ConfigPath:   configPath,
		Port:         port,
		Registry:     registry,
		Order:        panelOrder,
		Disabled:     panelDisabled,
		Version:      version,
		PublicConfig: publicConfig,
		Apps:         discoveredApps,
		Hooks:        hookEngine,
		DSManager:    dsManager,
	}

	handler := server.NewServer(cfg)
	hookEngine.Emit("core.server.ready")

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("[Server] Vel v%s running on http://0.0.0.0%s\n\n", version, addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Server failed: %v\n", err)
		os.Exit(1)
	}
}
