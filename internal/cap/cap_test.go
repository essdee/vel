package cap

import (
	"errors"
	"testing"
)

func TestCheckBlacklisted(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "strict",
		Blacklist: map[string]bool{"os/exec.Command": true},
		AppCaps:   map[string]bool{},
	})
	err := Check("os/exec", "Command")
	if !errors.Is(err, ErrBlacklisted) {
		t.Errorf("expected ErrBlacklisted, got %v", err)
	}
}

func TestCheckDeclaredCapability(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "strict",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{"net/http": true},
	})
	err := Check("net/http", "Get")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckStrictDenied(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "strict",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{},
	})
	err := Check("net/http", "Get")
	if !errors.Is(err, ErrNotDeclared) {
		t.Errorf("expected ErrNotDeclared, got %v", err)
	}
}

func TestCheckBypassAllows(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "bypass",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{},
	})
	err := Check("net/http", "Get")
	if err != nil {
		t.Errorf("expected nil in bypass mode, got %v", err)
	}
}

func TestCheckBypassStillBlocksBlacklist(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "bypass",
		Blacklist: map[string]bool{"os/exec.Command": true},
		AppCaps:   map[string]bool{},
	})
	err := Check("os/exec", "Command")
	if !errors.Is(err, ErrBlacklisted) {
		t.Errorf("expected ErrBlacklisted even in bypass, got %v", err)
	}
}

func TestCheckSiteBlock(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "bypass",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{"net/http": true},
		SiteBlock: map[string]bool{"net/http.Get": true},
	})
	err := Check("net/http", "Get")
	if !errors.Is(err, ErrBlacklisted) {
		t.Errorf("expected ErrBlacklisted from site block, got %v", err)
	}
}

func TestCheckSiteAllow(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "strict",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{},
		SiteAllow: map[string]bool{"database/sql": true},
	})
	err := Check("database/sql", "Open")
	if err != nil {
		t.Errorf("expected nil with site allow, got %v", err)
	}
}

func TestUsageLog(t *testing.T) {
	ResetLog()
	Init(&Config{
		Mode:      "bypass",
		Blacklist: map[string]bool{},
		AppCaps:   map[string]bool{},
	})
	Check("net/http", "Get")
	Check("net/http", "Post")
	log := GetLog()
	if len(log) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(log))
	}
	if log[0].Pkg != "net/http" || log[0].Fn != "Get" || log[0].Action != "bypass" {
		t.Errorf("unexpected log entry: %+v", log[0])
	}
}

func TestCheckNilConfig(t *testing.T) {
	Init(nil)
	err := Check("anything", "Func")
	if err != nil {
		t.Errorf("nil config should allow all, got %v", err)
	}
}
