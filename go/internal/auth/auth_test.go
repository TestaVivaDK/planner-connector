package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init(false)
	os.Exit(m.Run())
}

func TestAllScopes(t *testing.T) {
	am := NewAuthManager("test-client", "test-tenant")
	got := am.AllScopes()
	want := "https://graph.microsoft.com/Tasks.ReadWrite https://graph.microsoft.com/Group.Read.All https://graph.microsoft.com/User.Read offline_access openid profile"
	if got != want {
		t.Errorf("AllScopes()\ngot:  %s\nwant: %s", got, want)
	}
}

func TestTokenCacheRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "subdir", "tokens.json")
	t.Setenv("PLANNER_MCP_TOKEN_CACHE_PATH", cachePath)

	// Save tokens with one manager.
	am1 := NewAuthManager("cid", "tid")
	am1.accessToken = "at-123"
	am1.refreshToken = "rt-456"
	am1.tokenExpiry = 9999999999999

	if err := am1.SaveTokenCache(); err != nil {
		t.Fatalf("SaveTokenCache: %v", err)
	}

	// Verify file permissions.
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("cache file perm = %o, want 0600", perm)
	}

	// Verify parent dir permissions.
	dirInfo, err := os.Stat(filepath.Dir(cachePath))
	if err != nil {
		t.Fatalf("stat cache dir: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0o700 {
		t.Errorf("cache dir perm = %o, want 0700", dirPerm)
	}

	// Verify JSON contents.
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var cache tokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cache.AccessToken != "at-123" || cache.RefreshToken != "rt-456" || cache.TokenExpiry != 9999999999999 {
		t.Errorf("unexpected cache contents: %+v", cache)
	}

	// Load in a fresh manager.
	am2 := NewAuthManager("cid", "tid")
	if err := am2.LoadTokenCache(); err != nil {
		t.Fatalf("LoadTokenCache: %v", err)
	}
	if am2.accessToken != "at-123" {
		t.Errorf("accessToken = %q, want %q", am2.accessToken, "at-123")
	}
	if am2.refreshToken != "rt-456" {
		t.Errorf("refreshToken = %q, want %q", am2.refreshToken, "rt-456")
	}
	if am2.tokenExpiry != 9999999999999 {
		t.Errorf("tokenExpiry = %d, want %d", am2.tokenExpiry, 9999999999999)
	}
}
