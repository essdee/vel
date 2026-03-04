package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vel/internal/apps"
	"vel/internal/auth"
	"vel/internal/build"
	"vel/internal/datasource"
	"vel/internal/hooks"
	"vel/internal/panels"
	"vel/internal/server"
	"vel/internal/verify"
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
		AllowedTelegramUsers []int64 `json:"allowedTelegramUsers"`
		Mode         string  `json:"mode"`
		Token        string  `json:"token"`
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
  version   Print version
  help      Show this help

Run 'vel <command> --help' for command-specific flags.`)
}

func runVerify(_ []string) {
	rootDir, _ := os.Getwd()

	fmt.Print("\n⚡ Vel Health Check\n\n")

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

	// Print grouped output
	var panelChecks, dataChecks, appChecks, coreChecks []verify.CheckResult
	for _, c := range result.Checks {
		switch {
		case strings.HasPrefix(c.Name, "panel:"):
			panelChecks = append(panelChecks, c)
		case strings.HasPrefix(c.Name, "data:"):
			dataChecks = append(dataChecks, c)
		case c.Name == "config" || c.Name == "auth" || c.Name == "openclaw-cli":
			coreChecks = append(coreChecks, c)
		default:
			appChecks = append(appChecks, c)
		}
	}

	printCheck := func(c verify.CheckResult) {
		label := c.Name
		// Strip prefixes for display
		label = strings.TrimPrefix(label, "panel:")
		label = strings.TrimPrefix(label, "data:")

		var detail string
		if c.Detail != "" {
			detail = " — " + c.Detail
		}

		if c.Status == "ok" {
			fmt.Printf("  ✓ %s%s\n", label, detail)
		} else {
			fmt.Printf("  ✗ %s%s\n", label, detail)
		}
	}

	// Core checks
	for _, c := range coreChecks {
		printCheck(c)
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

	// App checks
	if len(appChecks) > 0 {
		fmt.Println("\n  App checks:")
		for _, c := range appChecks {
			printCheck(c)
		}
	}

	fmt.Printf("\n  %d passed, %d failed\n\n", result.Passed, result.Failed)

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

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	portFlag := fs.Int("port", 0, "Override server port")
	fs.Parse(args)

	rootDir, _ := os.Getwd()

	// TEST_MODE warning
	if os.Getenv("TEST_MODE") == "true" {
		fmt.Println("\n⚠️  TEST_MODE is enabled — auth bypassed")
		fmt.Println("⚠️  Do NOT use in production.")
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
			if err := dsManager.AddFileSource(a.Name, ds.Name, ds.Path, ds.Interval); err != nil {
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
