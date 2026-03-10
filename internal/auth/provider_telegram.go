package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// TelegramCredentials holds raw initData extracted from the request.
type TelegramCredentials struct {
	InitData string
}

// TelegramProvider authenticates Telegram Mini App initData.
type TelegramProvider struct {
	botToken  string
	userStore *UserStore
}

// NewTelegramProvider creates a Telegram auth provider.
func NewTelegramProvider(botToken string, userStore *UserStore) *TelegramProvider {
	return &TelegramProvider{
		botToken:  botToken,
		userStore: userStore,
	}
}

func (p *TelegramProvider) Name() string { return "telegram" }

// Extract checks for initData in Authorization header (tma scheme) or
// X-Telegram-Init-Data header.
func (p *TelegramProvider) Extract(r *http.Request) (Credentials, bool) {
	// Try Authorization: tma <initData>
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "tma ") {
		initData := strings.TrimPrefix(authHeader, "tma ")
		if initData != "" {
			return TelegramCredentials{InitData: initData}, true
		}
	}

	// Fall back to X-Telegram-Init-Data header
	if initData := r.Header.Get("X-Telegram-Init-Data"); initData != "" {
		return TelegramCredentials{InitData: initData}, true
	}

	return nil, false
}

// Authenticate verifies the HMAC-SHA256 signature and looks up the user.
func (p *TelegramProvider) Authenticate(creds Credentials) (*Identity, error) {
	tc, ok := creds.(TelegramCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for telegram provider")
	}

	// Validate initData using existing HMAC logic (uses package-level botToken)
	user := ValidateInitData(tc.InitData)
	if user == nil {
		return nil, fmt.Errorf("invalid or expired initData")
	}

	// Look up user in UserStore
	providerID := strconv.FormatInt(user.ID, 10)
	record := p.userStore.FindUserByIdentity("telegram", providerID)
	if record == nil {
		return nil, fmt.Errorf("telegram user %d not found in users.json", user.ID)
	}

	return &Identity{
		UserID:   record.ID,
		Name:     record.Name,
		Provider: "telegram",
		Role:     record.Role,
		Scopes:   nil, // full access based on role
		Meta: map[string]string{
			"telegram_id":       providerID,
			"telegram_name":     user.FirstName,
			"telegram_username": user.Username,
		},
	}, nil
}
