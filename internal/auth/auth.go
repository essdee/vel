package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScopedToken represents a token with restricted API access.
type ScopedToken struct {
	Name   string   `json:"name"`
	Token  string   `json:"token"`
	Scopes []string `json:"scopes"` // allowed API path prefixes (e.g. "/token-swap/api/usage-share")
}

var (
	botToken     string
	allowedUsers map[int64]bool
	cookieSecret string
	authMode     string // "telegram", "token", "none"
	authToken    string
	scopedTokens []ScopedToken
	scopedMu     sync.RWMutex
)

func Init(token string, allowed []int64, secret string) {
	botToken = token
	allowedUsers = make(map[int64]bool)
	for _, id := range allowed {
		allowedUsers[id] = true
	}
	cookieSecret = secret
}

// InitMode sets the auth mode and token for token-based auth.
func InitMode(mode, token string) {
	authMode = mode
	authToken = token
}

// InitScopedTokens loads scoped tokens into memory.
func InitScopedTokens(tokens []ScopedToken) {
	scopedMu.Lock()
	defer scopedMu.Unlock()
	scopedTokens = make([]ScopedToken, len(tokens))
	copy(scopedTokens, tokens)
}

// GetScopedTokens returns a copy of all scoped tokens (with tokens masked).
func GetScopedTokens() []ScopedToken {
	scopedMu.RLock()
	defer scopedMu.RUnlock()
	result := make([]ScopedToken, len(scopedTokens))
	for i, t := range scopedTokens {
		result[i] = ScopedToken{
			Name:   t.Name,
			Token:  maskScopedToken(t.Token),
			Scopes: t.Scopes,
		}
	}
	return result
}

// GetScopedTokensFull returns unmasked tokens (for config write-back).
func GetScopedTokensFull() []ScopedToken {
	scopedMu.RLock()
	defer scopedMu.RUnlock()
	result := make([]ScopedToken, len(scopedTokens))
	copy(result, scopedTokens)
	return result
}

// AddScopedToken adds a new scoped token. Returns the generated token string.
func AddScopedToken(name string, scopes []string) (string, error) {
	scopedMu.Lock()
	defer scopedMu.Unlock()

	// Check for duplicate name
	for _, t := range scopedTokens {
		if t.Name == name {
			return "", fmt.Errorf("token with name %q already exists", name)
		}
	}

	token := generateRandomToken(32)
	scopedTokens = append(scopedTokens, ScopedToken{
		Name:   name,
		Token:  token,
		Scopes: scopes,
	})
	return token, nil
}

// RemoveScopedToken removes a scoped token by name.
func RemoveScopedToken(name string) bool {
	scopedMu.Lock()
	defer scopedMu.Unlock()

	for i, t := range scopedTokens {
		if t.Name == name {
			scopedTokens = append(scopedTokens[:i], scopedTokens[i+1:]...)
			return true
		}
	}
	return false
}

// CheckScopedToken checks if a token matches any scoped token and if the path is allowed.
// Returns the token name if valid, empty string if not.
func CheckScopedToken(token, path string) string {
	if token == "" {
		return ""
	}
	scopedMu.RLock()
	defer scopedMu.RUnlock()

	for _, t := range scopedTokens {
		if t.Token == token {
			// Check if path matches any scope
			for _, scope := range t.Scopes {
				if strings.HasPrefix(path, scope) {
					return t.Name
				}
			}
			return "" // token valid but path not in scope
		}
	}
	return ""
}

func generateRandomToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func maskScopedToken(t string) string {
	if len(t) <= 8 {
		return "••••"
	}
	return t[:4] + "••••" + t[len(t)-4:]
}

// GetAuthMode returns the current auth mode.
func GetAuthMode() string {
	return authMode
}

// ValidateToken checks if the provided token matches the configured auth token.
func ValidateToken(token string) bool {
	return authToken != "" && token == authToken
}

func IsTestMode() bool {
	return os.Getenv("TEST_MODE") == "true"
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// ValidateInitData validates Telegram Mini App initData (HMAC-SHA256)
func ValidateInitData(initData string) *User {
	params, err := neturl.ParseQuery(initData)
	if err != nil {
		return nil
	}

	hash := params.Get("hash")
	if hash == "" {
		return nil
	}

	params.Del("hash")

	// Sort and build data-check-string
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params.Get(k)))
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret_key = HMAC_SHA256("WebAppData", BOT_TOKEN)
	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	computedHash := hex.EncodeToString(hmacSHA256(secretKey, []byte(dataCheckString)))

	// Timing-safe comparison
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil
	}
	computedBytes, err := hex.DecodeString(computedHash)
	if err != nil {
		return nil
	}
	if !hmac.Equal(hashBytes, computedBytes) {
		return nil
	}

	// Check auth_date (24h expiry)
	authDate, _ := strconv.ParseInt(params.Get("auth_date"), 10, 64)
	if time.Now().Unix()-authDate > 86400 {
		return nil
	}

	// Parse user
	userStr := params.Get("user")
	if userStr == "" {
		return nil
	}
	var user User
	if err := json.Unmarshal([]byte(userStr), &user); err != nil {
		return nil
	}
	return &user
}

// ValidateTelegramLogin validates Login Widget callback
func ValidateTelegramLogin(params map[string]string) bool {
	hash, ok := params["hash"]
	if !ok {
		return false
	}

	var keys []string
	for k := range params {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret = SHA256(BOT_TOKEN)
	h := sha256.Sum256([]byte(botToken))
	computed := hex.EncodeToString(hmacSHA256(h[:], []byte(dataCheckString)))

	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	computedBytes, err := hex.DecodeString(computed)
	if err != nil {
		return false
	}
	return hmac.Equal(hashBytes, computedBytes)
}

func IsAllowed(userID int64) bool {
	if IsTestMode() {
		return true
	}
	if authMode == "token" || authMode == "none" {
		return true
	}
	return allowedUsers[userID]
}

// SignCookie creates an HMAC-signed cookie value (URL-encoded for safe cookie transport)
func SignCookie(value string) string {
	sig := hex.EncodeToString(hmacSHA256([]byte(cookieSecret), []byte(value)))
	raw := fmt.Sprintf("s:%s.%s", value, sig)
	return neturl.QueryEscape(raw)
}

// VerifyCookie verifies and extracts signed cookie value
func VerifyCookie(signed string) (string, bool) {
	// URL-decode first
	decoded, err := neturl.QueryUnescape(signed)
	if err != nil {
		decoded = signed
	}

	if !strings.HasPrefix(decoded, "s:") {
		return "", false
	}
	rest := decoded[2:]
	dotIdx := strings.LastIndex(rest, ".")
	if dotIdx < 0 {
		return "", false
	}
	value := rest[:dotIdx]
	sig := rest[dotIdx+1:]

	expected := hex.EncodeToString(hmacSHA256([]byte(cookieSecret), []byte(value)))
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return "", false
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return "", false
	}
	if !hmac.Equal(sigBytes, expectedBytes) {
		return "", false
	}
	return value, true
}

// GetUserFromCookie extracts user from signed tg_user cookie
func GetUserFromCookie(r *http.Request) *User {
	cookie, err := r.Cookie("tg_user")
	if err != nil {
		return nil
	}
	value, ok := VerifyCookie(cookie.Value)
	if !ok {
		return nil
	}
	var user User
	if err := json.Unmarshal([]byte(value), &user); err != nil {
		return nil
	}
	return &user
}

// Check returns authenticated user or nil.
// Checks (in order): test mode, auth mode "none", cookie, Bearer header,
// ?token= query param (master token), X-Telegram-Init-Data, scoped tokens.
func Check(r *http.Request) *User {
	if IsTestMode() {
		return &User{ID: 0, FirstName: "Test", Username: "test"}
	}

	// "none" mode: always return default user
	if authMode == "none" {
		return &User{ID: 1, FirstName: "Admin", Username: "admin"}
	}

	// Try cookie (works for both telegram and token modes)
	user := GetUserFromCookie(r)
	if user != nil && IsAllowed(user.ID) {
		return user
	}

	// Try Bearer token header (token mode)
	if authMode == "token" {
		if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
			if ValidateToken(strings.TrimPrefix(bearer, "Bearer ")) {
				return &User{ID: 1, FirstName: "Admin", Username: "admin"}
			}
		}
	}

	// Try ?token= query parameter — master token (full access)
	if qToken := r.URL.Query().Get("token"); qToken != "" {
		if ValidateToken(qToken) {
			return &User{ID: 1, FirstName: "Admin", Username: "admin"}
		}
		// Try scoped tokens — restricted to specific API paths
		if name := CheckScopedToken(qToken, r.URL.Path); name != "" {
			return &User{ID: 2, FirstName: name, Username: "scoped:" + name}
		}
	}

	// Try X-Telegram-Init-Data header (Telegram Mini App panels)
	if initData := r.Header.Get("X-Telegram-Init-Data"); initData != "" {
		if user := ValidateInitData(initData); user != nil && IsAllowed(user.ID) {
			return user
		}
	}

	return nil
}

// IsScopedUser returns true if the user was authenticated via a scoped token.
func IsScopedUser(u *User) bool {
	return u != nil && strings.HasPrefix(u.Username, "scoped:")
}

// GetBotToken returns the configured bot token.
func GetBotToken() string {
	return botToken
}

// CheckBotToken validates a token against the bot token.
func CheckBotToken(token string) bool {
	return token != "" && token == botToken
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
