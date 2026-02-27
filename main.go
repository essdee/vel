package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vel/internal/auth"
	"vel/internal/hooks"
	"vel/internal/panels"
	"vel/internal/server"
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
		AllowedUsers []int64 `json:"allowedUsers"`
	} `json:"auth"`
	AllowedUsers []int64 `json:"allowedUsers"` // legacy field

	// Panels
	Panels struct {
		Order    []string `json:"order"`
		Disabled []string `json:"disabled"`
	} `json:"panels"`

	// Custom routes
	Routes map[string]string `json:"routes"`

	// Server
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
	Port int `json:"port"` // legacy field
}

func main() {
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
	if botToken == "" {
		log.Fatal("[Fatal] BOT_TOKEN environment variable is required")
	}

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
	allowedUsers := config.Auth.AllowedUsers
	if len(allowedUsers) == 0 {
		allowedUsers = config.AllowedUsers
	}

	// Init auth
	auth.Init(botToken, allowedUsers, strings.TrimSpace(string(cookieSecret)))

	// Port: env PORT > config.server.port > config.port > 3700
	port := 0
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
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

	// Discover panels
	fmt.Println("\n[Panels] Discovering panels...")
	registry, report := panels.DiscoverPanels(rootDir)

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
	publicConfig := map[string]interface{}{}
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
	panelOrder := config.Panels.Order
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
		Routes:       config.Routes,
		Hooks:        hookEngine,
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
