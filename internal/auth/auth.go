package auth

import (
	"crypto/hmac"
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
	"time"
)

var (
	botToken     string
	allowedUsers map[int64]bool
	cookieSecret string
)

func Init(token string, allowed []int64, secret string) {
	botToken = token
	allowedUsers = make(map[int64]bool)
	for _, id := range allowed {
		allowedUsers[id] = true
	}
	cookieSecret = secret
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

// Check returns authenticated user or nil
func Check(r *http.Request) *User {
	if IsTestMode() {
		return &User{ID: 0, FirstName: "Test", Username: "test"}
	}

	// Try cookie
	user := GetUserFromCookie(r)
	if user != nil && IsAllowed(user.ID) {
		return user
	}
	return nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
