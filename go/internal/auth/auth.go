package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

// AuthManager handles OAuth2 PKCE authentication against Azure AD and
// caches tokens on disk.
type AuthManager struct {
	clientID string
	tenantID string
	scopes   []string

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	tokenExpiry  int64 // unix milliseconds
}

// tokenCache is the on-disk JSON representation of the token cache.
type tokenCache struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenExpiry  int64  `json:"tokenExpiry"`
}

// tokenResponse is the JSON body returned by the Azure AD token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// NewAuthManager creates an AuthManager with the standard Microsoft Graph scopes.
func NewAuthManager(clientID, tenantID string) *AuthManager {
	return &AuthManager{
		clientID: clientID,
		tenantID: tenantID,
		scopes: []string{
			"https://graph.microsoft.com/Tasks.ReadWrite",
			"https://graph.microsoft.com/Group.Read.All",
			"https://graph.microsoft.com/User.Read",
		},
	}
}

// AllScopes returns all scopes (Graph + OIDC) joined by spaces.
func (am *AuthManager) AllScopes() string {
	all := make([]string, 0, len(am.scopes)+3)
	all = append(all, am.scopes...)
	all = append(all, "offline_access", "openid", "profile")
	return strings.Join(all, " ")
}

// tokenCachePath returns the path to the token cache file.
func (am *AuthManager) tokenCachePath() string {
	if p := strings.TrimSpace(os.Getenv("PLANNER_MCP_TOKEN_CACHE_PATH")); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ".token-cache.json"
	}
	return filepath.Join(filepath.Dir(exe), ".token-cache.json")
}

// LoadTokenCache reads tokens from the JSON cache file.
// Missing or corrupt files are silently ignored.
func (am *AuthManager) LoadTokenCache() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(am.tokenCachePath())
	if err != nil {
		// File missing or unreadable — not an error worth surfacing.
		return nil
	}

	var cache tokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.Log.Warn("token cache is corrupt, starting fresh")
		return nil
	}

	am.accessToken = cache.AccessToken
	am.refreshToken = cache.RefreshToken
	am.tokenExpiry = cache.TokenExpiry
	return nil
}

// SaveTokenCache writes the current tokens to the JSON cache file with
// restricted permissions.
func (am *AuthManager) SaveTokenCache() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	return am.saveTokenCacheLocked()
}

func (am *AuthManager) saveTokenCacheLocked() error {
	cache := tokenCache{
		AccessToken:  am.accessToken,
		RefreshToken: am.refreshToken,
		TokenExpiry:  am.tokenExpiry,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal token cache: %w", err)
	}

	p := am.tokenCachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write token cache: %w", err)
	}
	return nil
}

// GetToken returns a valid access token, refreshing or acquiring interactively
// as needed.
func (am *AuthManager) GetToken() (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Return cached token if valid (5 min buffer).
	now := time.Now().UnixMilli()
	if am.accessToken != "" && am.tokenExpiry > now+5*60*1000 {
		return am.accessToken, nil
	}

	// Try silent refresh.
	if am.refreshToken != "" {
		if err := am.refreshAccessToken(); err != nil {
			logger.Log.Info("token refresh failed, triggering interactive login...", "error", err)
		} else {
			return am.accessToken, nil
		}
	}

	// Interactive login.
	if err := am.acquireTokenInteractively(); err != nil {
		return "", err
	}
	return am.accessToken, nil
}

// refreshAccessToken uses the refresh token to obtain new tokens.
// Must be called with am.mu held.
func (am *AuthManager) refreshAccessToken() error {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", am.tenantID)

	form := url.Values{
		"client_id":     {am.clientID},
		"scope":         {am.AllScopes()},
		"refresh_token": {am.refreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		am.refreshToken = ""
		return fmt.Errorf("refresh token expired or revoked (HTTP %d)", resp.StatusCode)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return fmt.Errorf("decode refresh response: %w", err)
	}

	am.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		am.refreshToken = tokens.RefreshToken
	}
	am.tokenExpiry = time.Now().UnixMilli() + tokens.ExpiresIn*1000
	return am.saveTokenCacheLocked()
}

// AcquireTokenInteractively runs the full PKCE authorization code flow.
// Must be called with am.mu held.
func (am *AuthManager) acquireTokenInteractively() error {
	// 1. Generate PKCE codes.
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// 2. Start loopback server.
	port, codeCh, errCh, cleanup := StartLoopbackServer()
	defer cleanup()

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 3. Build authorize URL.
	params := url.Values{
		"client_id":             {am.clientID},
		"response_type":        {"code"},
		"redirect_uri":         {redirectURI},
		"response_mode":        {"query"},
		"scope":                {am.AllScopes()},
		"code_challenge":       {challenge},
		"code_challenge_method": {"S256"},
	}
	authURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s",
		am.tenantID, params.Encode())

	// 4. Open browser.
	OpenBrowser(authURL)

	// 5. Wait for code (5 min timeout).
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return fmt.Errorf("auth callback error: %w", err)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("login timed out — no response received within 5 minutes")
	}

	// 6. Exchange code for tokens.
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", am.tenantID)
	form := url.Values{
		"client_id":     {am.clientID},
		"scope":         {am.AllScopes()},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}

	// 7. Save tokens.
	am.accessToken = tokens.AccessToken
	am.refreshToken = tokens.RefreshToken
	am.tokenExpiry = time.Now().UnixMilli() + tokens.ExpiresIn*1000
	return am.saveTokenCacheLocked()
}

// TestLogin acquires a token, calls the Graph /me endpoint, and returns the result.
func (am *AuthManager) TestLogin() (map[string]any, error) {
	token, err := am.GetToken()
	if err != nil {
		return map[string]any{
			"success": false,
			"message": err.Error(),
		}, nil
	}

	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, fmt.Errorf("build /me request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{
			"success": false,
			"message": err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"success": false,
			"message": fmt.Sprintf("Graph API error: %d", resp.StatusCode),
		}, nil
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return map[string]any{
			"success": false,
			"message": fmt.Sprintf("decode Graph response: %s", err),
		}, nil
	}

	return map[string]any{
		"success": true,
		"message": "Logged in",
		"user": map[string]any{
			"displayName":       data["displayName"],
			"userPrincipalName": data["userPrincipalName"],
		},
	}, nil
}

// Logout clears all tokens from memory and deletes the cache file.
func (am *AuthManager) Logout() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.accessToken = ""
	am.refreshToken = ""
	am.tokenExpiry = 0

	p := am.tokenCachePath()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove token cache: %w", err)
	}
	return nil
}
