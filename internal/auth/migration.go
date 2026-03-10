package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// oldAuthConfig matches the legacy config.json auth structure.
type oldAuthConfig struct {
	AllowedTelegramUsers []int64 `json:"allowedTelegramUsers"`
	Mode                 string  `json:"mode"`
	Token                string  `json:"token"`
}

// topLevelConfig is a minimal parse of config.json to detect migration need.
type topLevelConfig struct {
	Auth *oldAuthConfig `json:"auth"`
}

// MigrateIfNeeded checks whether users.json already exists.
// If it does NOT exist, but config.json has the old allowedTelegramUsers field,
// it auto-generates users.json from that list and prints a deprecation warning.
//
// configPath: path to config.json (e.g. "/opt/vel-staging/config.json")
// usersPath:  desired path for users.json (e.g. "/opt/vel-staging/users.json")
func MigrateIfNeeded(configPath, usersPath string) error {
	// If users.json already exists, no migration needed.
	if _, err := os.Stat(usersPath); err == nil {
		return nil
	}

	// Read config.json.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist either — nothing to migrate.
		return nil
	}

	var cfg topLevelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("migration: parse config: %w", err)
	}

	// Only migrate if old allowedTelegramUsers list is present.
	if cfg.Auth == nil || len(cfg.Auth.AllowedTelegramUsers) == 0 {
		return nil
	}

	fmt.Println("⚠  Migrating allowedTelegramUsers → users.json (deprecated config fields will be ignored in a future release)")

	var uf UsersFile
	uf.APIKeys = []APIKey{}
	for _, telegramID := range cfg.Auth.AllowedTelegramUsers {
		idStr := strconv.FormatInt(telegramID, 10)
		uf.Users = append(uf.Users, UserRecord{
			ID:   "user-" + idStr,
			Name: "User " + idStr,
			Role: "admin",
			Identities: []UserIdentity{
				{Provider: "telegram", ProviderID: idStr},
			},
		})
	}

	if err := SaveUsers(usersPath, &uf); err != nil {
		return fmt.Errorf("migration: write users.json: %w", err)
	}

	fmt.Printf("✓  Created %s with %d user(s)\n", usersPath, len(uf.Users))
	return nil
}
