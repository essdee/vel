package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testToken = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

func TestInit(t *testing.T) {
	Init(testToken, []int64{100, 200}, "test-secret")
	if !IsAllowed(100) {
		t.Error("expected user 100 to be allowed")
	}
	if IsAllowed(999) {
		t.Error("expected user 999 to not be allowed")
	}
}

func TestValidateInitData(t *testing.T) {
	Init(testToken, []int64{123}, "test-secret")

	// Create valid initData
	authDate := fmt.Sprintf("%d", time.Now().Unix())
	user := `{"id":123,"first_name":"Test","username":"test"}`
	dataCheckString := fmt.Sprintf("auth_date=%s\nquery_id=AAHdF6IQAAAAAN0XohDhrOrc\nuser=%s", authDate, user)

	secretKey := hmacSHA256([]byte("WebAppData"), []byte(testToken))
	hash := hex.EncodeToString(hmacSHA256(secretKey, []byte(dataCheckString)))

	initData := fmt.Sprintf("auth_date=%s&query_id=AAHdF6IQAAAAAN0XohDhrOrc&user=%s&hash=%s", authDate, user, hash)

	result := ValidateInitData(initData)
	if result == nil {
		t.Fatal("expected valid user, got nil")
	}
	if result.ID != 123 {
		t.Errorf("expected user ID 123, got %d", result.ID)
	}
}

func TestValidateInitDataInvalid(t *testing.T) {
	Init(testToken, []int64{123}, "test-secret")
	result := ValidateInitData("invalid_data&hash=abc123")
	if result != nil {
		t.Error("expected nil for invalid initData")
	}
}

func TestValidateTelegramLogin(t *testing.T) {
	Init(testToken, []int64{123}, "test-secret")

	authDate := fmt.Sprintf("%d", time.Now().Unix())
	params := map[string]string{
		"id":         "123",
		"first_name": "Test",
		"auth_date":  authDate,
	}

	// Build data-check-string
	dataCheckString := fmt.Sprintf("auth_date=%s\nfirst_name=Test\nid=123", authDate)
	h := sha256.Sum256([]byte(testToken))
	hash := hex.EncodeToString(hmacSHA256(h[:], []byte(dataCheckString)))
	params["hash"] = hash

	if !ValidateTelegramLogin(params) {
		t.Error("expected valid telegram login")
	}

	// Tamper
	params["first_name"] = "Hacker"
	if ValidateTelegramLogin(params) {
		t.Error("expected invalid after tampering")
	}
}

func TestCookieSignVerify(t *testing.T) {
	Init(testToken, []int64{123}, "my-secret-key")

	value := `{"id":123,"first_name":"Test"}`
	signed := SignCookie(value)

	extracted, ok := VerifyCookie(signed)
	if !ok {
		t.Fatal("expected cookie verification to succeed")
	}
	if extracted != value {
		t.Errorf("expected %q, got %q", value, extracted)
	}

	// Tamper
	_, ok = VerifyCookie(signed + "x")
	if ok {
		t.Error("expected tampered cookie to fail")
	}
}

func TestGetUserFromCookie(t *testing.T) {
	Init(testToken, []int64{123}, "my-secret-key")

	value := `{"id":123,"first_name":"Test"}`
	signed := SignCookie(value)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "tg_user", Value: signed})

	user := GetUserFromCookie(req)
	if user == nil {
		t.Fatal("expected user from cookie")
	}
	if user.ID != 123 {
		t.Errorf("expected ID 123, got %d", user.ID)
	}
}
