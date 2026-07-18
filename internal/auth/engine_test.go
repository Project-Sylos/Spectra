package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

func TestIssueAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AuthJSONFileName)
	e := NewEngine(&types.AuthConfig{Enabled: true, AccessTokenTTLSeconds: 3600}, path)

	pair, err := e.IssueTokens("primary")
	if err != nil {
		t.Fatalf("IssueTokens: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected tokens")
	}
	if err := e.ValidateWorld("primary"); err != nil {
		t.Fatalf("ValidateWorld: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persist file: %v", err)
	}
}

func TestExpiryAndRefresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AuthJSONFileName)
	e := NewEngine(&types.AuthConfig{Enabled: true, AccessTokenTTLSeconds: 10}, path)

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e.SetNow(func() time.Time { return now })

	pair, err := e.IssueTokens("s1")
	if err != nil {
		t.Fatalf("IssueTokens: %v", err)
	}
	oldAccess := pair.AccessToken

	now = now.Add(11 * time.Second)
	if err := e.ValidateWorld("s1"); err == nil {
		t.Fatal("expected expired")
	}

	refreshed, err := e.Refresh("s1", pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken == oldAccess {
		t.Fatal("expected new access token")
	}
	if refreshed.RefreshToken != pair.RefreshToken {
		t.Fatal("refresh token should stay the same")
	}
	if err := e.ValidateWorld("s1"); err != nil {
		t.Fatalf("ValidateWorld after refresh: %v", err)
	}
}

func TestRefreshDebounce(t *testing.T) {
	e := NewEngine(&types.AuthConfig{Enabled: true, AccessTokenTTLSeconds: 10}, "")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e.SetNow(func() time.Time { return now })

	pair, err := e.IssueTokens("primary")
	if err != nil {
		t.Fatalf("IssueTokens: %v", err)
	}
	now = now.Add(11 * time.Second)

	first, err := e.Refresh("primary", pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	second, err := e.Refresh("primary", pair.RefreshToken)
	if err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if second.AccessToken != first.AccessToken {
		t.Fatal("debounced Refresh should reuse access token")
	}

	now = now.Add(RefreshDebounce)
	third, err := e.Refresh("primary", pair.RefreshToken)
	if err != nil {
		t.Fatalf("third Refresh: %v", err)
	}
	if third.AccessToken == first.AccessToken {
		t.Fatal("expected new access token after debounce window")
	}
}

func TestPersistAcrossEngines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AuthJSONFileName)
	cfg := &types.AuthConfig{Enabled: true, AccessTokenTTLSeconds: -1}

	e1 := NewEngine(cfg, path)
	pair, err := e1.IssueTokens("primary")
	if err != nil {
		t.Fatalf("IssueTokens: %v", err)
	}

	e2 := NewEngine(cfg, path)
	if err := e2.ValidateWorld("primary"); err != nil {
		t.Fatalf("restored ValidateWorld: %v", err)
	}
	e2.ClearAccessToken("primary")
	e2.SetAccessToken("primary", pair.AccessToken)
	if err := e2.ValidateWorld("primary"); err != nil {
		t.Fatalf("SetAccessToken ValidateWorld: %v", err)
	}
}

func TestDisabledNoOp(t *testing.T) {
	e := NewEngine(nil, "")
	if e.Enabled() {
		t.Fatal("expected disabled")
	}
	if err := e.ValidateWorld("primary"); err != nil {
		t.Fatalf("disabled should allow: %v", err)
	}
}

func TestWrongWorldToken(t *testing.T) {
	e := NewEngine(&types.AuthConfig{Enabled: true, AccessTokenTTLSeconds: -1}, "")
	p1, _ := e.IssueTokens("primary")
	_, _ = e.IssueTokens("s1")
	e.SetAccessToken("s1", p1.AccessToken)
	if err := e.ValidateWorld("s1"); err == nil {
		t.Fatal("expected invalid token for wrong world")
	}
}
