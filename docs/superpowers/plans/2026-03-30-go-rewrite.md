# Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite plannner-connector MCP server in Go, producing cross-platform binaries in a single .mcpb binary bundle.

**Architecture:** Go binary using `mcp-go` for stdio MCP transport, direct HTTP calls to Azure AD and Microsoft Graph API (no MSAL). Three binaries (linux/amd64, darwin/arm64, windows/amd64) in one .mcpb zip.

**Tech Stack:** Go 1.25, `github.com/mark3labs/mcp-go`, stdlib for everything else.

**Spec:** `docs/superpowers/specs/2026-03-30-go-rewrite-design.md`

---

## File Structure

```
go/
├── cmd/plannner-connector/
│   └── main.go                 # Entry point, CLI flags, MCP server setup
├── internal/
│   ├── auth/
│   │   ├── auth.go             # AuthManager: token cache, GetToken, refresh, interactive login
│   │   ├── auth_test.go        # Unit tests for token cache and scope building
│   │   ├── browser.go          # OpenBrowser: platform-specific browser launch
│   │   └── loopback.go         # StartLoopbackServer: temporary HTTP server for OAuth redirect
│   ├── graph/
│   │   ├── client.go           # Graph API HTTP client: Get, Post, Patch, Delete, GetEtag
│   │   └── client_test.go      # Unit tests for URL building, retry, error handling
│   ├── tools/
│   │   ├── register.go         # RegisterAll: wires all tools to MCP server
│   │   ├── auth.go             # planner-login, planner-logout, planner-auth-status
│   │   ├── endpoints.go        # Dynamic endpoint tools from embedded endpoints.json
│   │   ├── plans.go            # create-plan, update-plan, delete-plan
│   │   ├── buckets.go          # create-bucket, update-bucket, delete-bucket
│   │   ├── tasks.go            # create-task, update-task, delete-task, assign-task, unassign-task, move-task
│   │   └── taskdetails.go      # update-task-details, add-checklist-item, toggle-checklist-item
│   └── logger/
│       └── logger.go           # slog setup: file + optional stderr
├── endpoints.json              # Endpoint configs (go:embed)
├── go.mod
└── go.sum
```

---

### Task 1: Go Module Init + Logger

**Files:**
- Create: `go/go.mod`
- Create: `go/internal/logger/logger.go`
- Create: `go/cmd/plannner-connector/main.go` (skeleton)

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/mlu/Documents/project/plannner-connector
mkdir -p go/cmd/plannner-connector go/internal/logger
cd go
go mod init github.com/TestaVivaDK/plannner-connector
go get github.com/mark3labs/mcp-go@latest
```

- [ ] **Step 2: Create logger**

Create `go/internal/logger/logger.go`:
```go
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var Log *slog.Logger

func Init(verbose bool) {
	logDir := "logs"
	_ = os.MkdirAll(logDir, 0o700)
	logPath := filepath.Join(logDir, "planner-mcp.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// Fall back to discard if we can't open the log file
		Log = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}

	var w io.Writer = f
	if verbose {
		w = io.MultiWriter(f, os.Stderr)
	}
	Log = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
```

- [ ] **Step 3: Create main.go skeleton**

Create `go/cmd/plannner-connector/main.go`:
```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging to stderr")
	doLogin := flag.Bool("login", false, "Login and exit")
	doLogout := flag.Bool("logout", false, "Logout and exit")
	verifyLogin := flag.Bool("verify-login", false, "Verify login and exit")
	flag.Parse()

	logger.Init(*verbose)

	clientID := os.Getenv("PLANNER_MCP_CLIENT_ID")
	tenantID := os.Getenv("PLANNER_MCP_TENANT_ID")
	if clientID == "" || tenantID == "" {
		fmt.Fprintln(os.Stderr, "Missing PLANNER_MCP_CLIENT_ID or PLANNER_MCP_TENANT_ID")
		os.Exit(1)
	}

	// Placeholders for auth and server — filled in later tasks
	_, _, _ = *doLogin, *doLogout, *verifyLogin
	fmt.Fprintln(os.Stderr, "Server not yet implemented")
}
```

- [ ] **Step 4: Verify it builds**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./cmd/plannner-connector
```

Expected: builds with no errors.

- [ ] **Step 5: Commit**

```bash
git add go/
git commit -m "feat(go): init module, logger, main skeleton"
```

---

### Task 2: Auth Manager — Token Cache + Scopes

**Files:**
- Create: `go/internal/auth/auth.go`
- Create: `go/internal/auth/auth_test.go`

- [ ] **Step 1: Write tests for token cache round-trip and scope building**

Create `go/internal/auth/auth_test.go`:
```go
package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllScopes(t *testing.T) {
	am := NewAuthManager("test-client", "test-tenant")
	got := am.AllScopes()
	want := "https://graph.microsoft.com/Tasks.ReadWrite https://graph.microsoft.com/Group.Read.All https://graph.microsoft.com/User.Read offline_access openid profile"
	if got != want {
		t.Errorf("AllScopes() = %q, want %q", got, want)
	}
}

func TestTokenCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	t.Setenv("PLANNER_MCP_TOKEN_CACHE_PATH", cachePath)

	am := NewAuthManager("cid", "tid")
	am.accessToken = "tok123"
	am.refreshToken = "ref456"
	am.tokenExpiry = 9999999999999

	if err := am.SaveTokenCache(); err != nil {
		t.Fatalf("SaveTokenCache: %v", err)
	}

	am2 := NewAuthManager("cid", "tid")
	if err := am2.LoadTokenCache(); err != nil {
		t.Fatalf("LoadTokenCache: %v", err)
	}
	if am2.accessToken != "tok123" {
		t.Errorf("accessToken = %q, want tok123", am2.accessToken)
	}
	if am2.refreshToken != "ref456" {
		t.Errorf("refreshToken = %q, want ref456", am2.refreshToken)
	}

	// Verify file permissions
	info, _ := os.Stat(cachePath)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("cache file perm = %o, want 600", info.Mode().Perm())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go test ./internal/auth/...
```

Expected: FAIL — `NewAuthManager` not defined.

- [ ] **Step 3: Implement AuthManager struct, constructor, AllScopes, token cache**

Create `go/internal/auth/auth.go`:
```go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

var scopes = []string{
	"https://graph.microsoft.com/Tasks.ReadWrite",
	"https://graph.microsoft.com/Group.Read.All",
	"https://graph.microsoft.com/User.Read",
}

type tokenCache struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenExpiry  int64  `json:"tokenExpiry"`
}

type AuthManager struct {
	clientID string
	tenantID string
	scopes   []string

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	tokenExpiry  int64 // unix ms
}

func NewAuthManager(clientID, tenantID string) *AuthManager {
	return &AuthManager{
		clientID: clientID,
		tenantID: tenantID,
		scopes:   scopes,
	}
}

func (am *AuthManager) AllScopes() string {
	return strings.Join(am.scopes, " ") + " offline_access openid profile"
}

func (am *AuthManager) tokenCachePath() string {
	if p := os.Getenv("PLANNER_MCP_TOKEN_CACHE_PATH"); p != "" {
		return p
	}
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), ".token-cache.json")
}

func (am *AuthManager) LoadTokenCache() error {
	data, err := os.ReadFile(am.tokenCachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var tc tokenCache
	if err := json.Unmarshal(data, &tc); err != nil {
		logger.Log.Warn("corrupt token cache, starting fresh")
		return nil
	}
	am.accessToken = tc.AccessToken
	am.refreshToken = tc.RefreshToken
	am.tokenExpiry = tc.TokenExpiry
	return nil
}

func (am *AuthManager) SaveTokenCache() error {
	tc := tokenCache{
		AccessToken:  am.accessToken,
		RefreshToken: am.refreshToken,
		TokenExpiry:  am.tokenExpiry,
	}
	data, err := json.Marshal(tc)
	if err != nil {
		return err
	}
	p := am.tokenCachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (am *AuthManager) GetToken() (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Valid cached token (5 min buffer)
	if am.accessToken != "" && am.tokenExpiry > time.Now().UnixMilli()+5*60*1000 {
		return am.accessToken, nil
	}

	// Try refresh
	if am.refreshToken != "" {
		if err := am.refreshAccessToken(); err == nil {
			return am.accessToken, nil
		}
		logger.Log.Info("token refresh failed, triggering interactive login")
	}

	// Interactive login
	if err := am.AcquireTokenInteractively(); err != nil {
		return "", fmt.Errorf("login required: %w", err)
	}
	return am.accessToken, nil
}

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
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		am.refreshToken = ""
		return fmt.Errorf("refresh failed: %d", resp.StatusCode)
	}
	var result struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	am.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		am.refreshToken = result.RefreshToken
	}
	am.tokenExpiry = time.Now().UnixMilli() + result.ExpiresIn*1000
	return am.SaveTokenCache()
}

func (am *AuthManager) AcquireTokenInteractively() error {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return err
	}

	port, codeCh, errCh, cleanup := StartLoopbackServer()
	defer cleanup()

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d", port)

	params := url.Values{
		"client_id":             {am.clientID},
		"response_type":        {"code"},
		"redirect_uri":         {redirectURI},
		"response_mode":        {"query"},
		"scope":                {am.AllScopes()},
		"code_challenge":       {challenge},
		"code_challenge_method": {"S256"},
	}
	authURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s", am.tenantID, params.Encode())

	OpenBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("login timed out after 5 minutes")
	}

	// Exchange code for tokens
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
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := os.ReadFile("/dev/stdin") // won't work, read body instead
		var buf [4096]byte
		n, _ := resp.Body.Read(buf[:])
		return fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(buf[:n]))
	}
	var result struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	am.accessToken = result.AccessToken
	am.refreshToken = result.RefreshToken
	am.tokenExpiry = time.Now().UnixMilli() + result.ExpiresIn*1000
	return am.SaveTokenCache()
}

func (am *AuthManager) TestLogin() (map[string]any, error) {
	token, err := am.GetToken()
	if err != nil {
		return map[string]any{"success": false, "message": err.Error()}, nil
	}
	req, _ := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{"success": false, "message": err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return map[string]any{"success": false, "message": fmt.Sprintf("Graph API error: %d", resp.StatusCode)}, nil
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	return map[string]any{
		"success": true,
		"message": "Logged in",
		"user": map[string]any{
			"displayName":       data["displayName"],
			"userPrincipalName": data["userPrincipalName"],
		},
	}, nil
}

func (am *AuthManager) Logout() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.accessToken = ""
	am.refreshToken = ""
	am.tokenExpiry = 0
	p := am.tokenCachePath()
	if _, err := os.Stat(p); err == nil {
		os.Remove(p)
	}
	return nil
}

// generatePKCE creates PKCE verifier and challenge
func generatePKCE() (verifier, challenge string, err error) {
	import_crypto_rand := make([]byte, 32)
	if _, err := cryptoRandRead(import_crypto_rand); err != nil {
		return "", "", err
	}
	verifier = base64URLEncode(import_crypto_rand)
	hash := sha256Sum([]byte(verifier))
	challenge = base64URLEncode(hash[:])
	return verifier, challenge, nil
}
```

Wait — I'm making this too long inline. Let me write the actual plan file with clean task boundaries.

- [ ] **Step 4: Run tests**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go test ./internal/auth/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/auth/
git commit -m "feat(go): auth manager with token cache and scopes"
```

---

### Task 3: Auth — PKCE, Loopback Server, Browser Launch

**Files:**
- Create: `go/internal/auth/browser.go`
- Create: `go/internal/auth/loopback.go`

- [ ] **Step 1: Create browser.go**

Create `go/internal/auth/browser.go`:
```go
package auth

import (
	"os/exec"
	"runtime"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		logger.Log.Info("could not open browser", "error", err)
		return
	}
	// Detach — don't wait for browser to close
	go cmd.Wait()
}
```

- [ ] **Step 2: Create loopback.go**

Create `go/internal/auth/loopback.go`:
```go
package auth

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// StartLoopbackServer starts a temporary HTTP server on 127.0.0.1 with a random port.
// Returns the port, a channel that receives the auth code, an error channel, and a cleanup func.
func StartLoopbackServer() (port int, codeCh <-chan string, errCh <-chan error, cleanup func()) {
	code := make(chan string, 1)
	errC := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if c := q.Get("code"); c != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<h1>Login successful</h1><p>You can close this window and return to Claude.</p>")
			code <- c
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			errMsg := q.Get("error")
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			fmt.Fprint(w, "<h1>Login failed</h1><p>Something went wrong. Please try again.</p>")
			errC <- fmt.Errorf("%s: %s", errMsg, q.Get("error_description"))
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		errC <- err
		return 0, code, errC, func() {}
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, code, errC, func() { srv.Close() }
}
```

- [ ] **Step 3: Fix auth.go to use proper crypto imports and the helpers**

Replace the `generatePKCE` function and add imports in `go/internal/auth/auth.go`. The top of auth.go needs these imports:
```go
import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)
```

And `generatePKCE`:
```go
func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}
```

Fix the token exchange error reading in `AcquireTokenInteractively` — replace the broken body read:
```go
	if resp.StatusCode != 200 {
		var buf [4096]byte
		n, _ := resp.Body.Read(buf[:])
		return fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(buf[:n]))
	}
```

- [ ] **Step 4: Verify it all compiles**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
go test ./internal/auth/...
```

Expected: builds and tests pass.

- [ ] **Step 5: Commit**

```bash
git add go/internal/auth/
git commit -m "feat(go): PKCE, loopback server, browser launch"
```

---

### Task 4: Graph Client

**Files:**
- Create: `go/internal/graph/client.go`
- Create: `go/internal/graph/client_test.go`

- [ ] **Step 1: Write tests for URL building**

Create `go/internal/graph/client_test.go`:
```go
package graph

import (
	"testing"
)

func TestBuildURL(t *testing.T) {
	c := &Client{baseURL: "https://graph.microsoft.com/v1.0"}

	got := c.buildURL("/me/planner/plans", map[string]string{"$top": "10", "$filter": "status eq 'active'"})
	if got == "" {
		t.Fatal("buildURL returned empty")
	}
	// Should contain base + path + query params
	if !contains(got, "graph.microsoft.com/v1.0/me/planner/plans") {
		t.Errorf("missing path in %s", got)
	}
	if !contains(got, "%24top=10") && !contains(got, "$top=10") {
		t.Errorf("missing $top in %s", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go test ./internal/graph/...
```

Expected: FAIL — `Client` not defined.

- [ ] **Step 3: Implement Graph client**

Create `go/internal/graph/client.go`:
```go
package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

type TokenProvider interface {
	GetToken() (string, error)
}

type Client struct {
	baseURL  string
	auth     TokenProvider
	http     *http.Client
}

func NewClient(auth TokenProvider) *Client {
	return &Client{
		baseURL: "https://graph.microsoft.com/v1.0",
		auth:    auth,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) buildURL(path string, queryParams map[string]string) string {
	u := c.baseURL + path
	if len(queryParams) == 0 {
		return u
	}
	vals := url.Values{}
	for k, v := range queryParams {
		vals.Set(k, v)
	}
	return u + "?" + vals.Encode()
}

func (c *Client) Get(path string, queryParams map[string]string, extraHeaders map[string]string) (json.RawMessage, error) {
	// Support full URLs for pagination (nextLink)
	reqURL := path
	if !strings.HasPrefix(path, "http") {
		reqURL = c.buildURL(path, queryParams)
	}
	return c.doRequest("GET", reqURL, nil, "", extraHeaders)
}

func (c *Client) Post(path string, body any) (json.RawMessage, error) {
	return c.doRequest("POST", c.baseURL+path, body, "", nil)
}

func (c *Client) Patch(path string, body any, etag string) (json.RawMessage, error) {
	return c.doRequest("PATCH", c.baseURL+path, body, etag, nil)
}

func (c *Client) Delete(path string, etag string) (json.RawMessage, error) {
	return c.doRequest("DELETE", c.baseURL+path, nil, etag, nil)
}

func (c *Client) GetEtag(path string) (string, error) {
	data, err := c.Get(path, nil, nil)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}
	etag, ok := obj["@odata.etag"].(string)
	if !ok {
		return "", fmt.Errorf("no @odata.etag in response")
	}
	return etag, nil
}

func (c *Client) doRequest(method, reqURL string, body any, etag string, extraHeaders map[string]string) (json.RawMessage, error) {
	return c.doRequestWithRetry(method, reqURL, body, etag, extraHeaders, true)
}

func (c *Client) doRequestWithRetry(method, reqURL string, body any, etag string, extraHeaders map[string]string, canRetry bool) (json.RawMessage, error) {
	token, err := c.auth.GetToken()
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	logger.Log.Info(fmt.Sprintf("[GRAPH] %s %s", method, reqURL))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 429: retry once after Retry-After
	if resp.StatusCode == 429 && canRetry {
		retryAfter := 1
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if n, err := strconv.Atoi(ra); err == nil {
				retryAfter = n
			}
		}
		logger.Log.Info("throttled, retrying", "retryAfter", retryAfter)
		time.Sleep(time.Duration(retryAfter) * time.Second)
		return c.doRequestWithRetry(method, reqURL, body, etag, extraHeaders, false)
	}

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 412 {
		return nil, fmt.Errorf("resource was modified by another user (ETag mismatch). Fetch the latest version and retry")
	}

	if resp.StatusCode >= 400 {
		var graphErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &graphErr) == nil && graphErr.Error.Message != "" {
			return nil, fmt.Errorf("Graph API %d: %s", resp.StatusCode, graphErr.Error.Message)
		}
		return nil, fmt.Errorf("Graph API %d: %s", resp.StatusCode, string(respBody))
	}

	// Empty response (204 No Content)
	if len(respBody) == 0 {
		return json.RawMessage(`{"success":true}`), nil
	}
	return json.RawMessage(respBody), nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go test ./internal/graph/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/graph/
git commit -m "feat(go): graph API client with retry and ETag support"
```

---

### Task 5: Endpoint Tools (Dynamic from endpoints.json)

**Files:**
- Copy: `src/endpoints.json` → `go/endpoints.json`
- Create: `go/internal/tools/register.go`
- Create: `go/internal/tools/endpoints.go`

- [ ] **Step 1: Copy endpoints.json and embed it**

```bash
cp /home/mlu/Documents/project/plannner-connector/src/endpoints.json /home/mlu/Documents/project/plannner-connector/go/endpoints.json
```

- [ ] **Step 2: Create register.go (tool registration entry point)**

Create `go/internal/tools/register.go`:
```go
package tools

import (
	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAll(s *server.MCPServer, am *auth.AuthManager, gc *graph.Client) {
	registerAuthTools(s, am)
	registerEndpointTools(s, gc)
	registerPlanTools(s, gc)
	registerBucketTools(s, gc)
	registerTaskTools(s, gc)
	registerTaskDetailTools(s, gc)
}
```

- [ ] **Step 3: Create endpoints.go**

Create `go/internal/tools/endpoints.go`:
```go
package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed ../../endpoints.json
var endpointsJSON []byte

type endpointConfig struct {
	PathPattern string            `json:"pathPattern"`
	Method      string            `json:"method"`
	ToolName    string            `json:"toolName"`
	Scopes      []string          `json:"scopes"`
	LLMTip      string            `json:"llmTip"`
	Headers     map[string]string `json:"headers"`
}

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

func registerEndpointTools(s *server.MCPServer, gc *graph.Client) {
	var endpoints []endpointConfig
	json.Unmarshal(endpointsJSON, &endpoints)

	odataParams := []string{"filter", "select", "top", "orderby", "expand", "count", "search"}

	for _, ep := range endpoints {
		ep := ep // capture loop var
		pathParams := pathParamRe.FindAllStringSubmatch(ep.PathPattern, -1)

		desc := fmt.Sprintf("%s %s", strings.ToUpper(ep.Method), ep.PathPattern)
		if ep.LLMTip != "" {
			desc += "\n\n" + ep.LLMTip
		}

		opts := []mcp.ToolOption{mcp.WithDescription(desc), mcp.WithReadOnlyHintAnnotation(true)}
		for _, pp := range pathParams {
			opts = append(opts, mcp.WithString(pp[1], mcp.Required(), mcp.Description(pp[1])))
		}
		for _, op := range odataParams {
			opts = append(opts, mcp.WithString(op, mcp.Description("OData $"+op)))
		}
		opts = append(opts, mcp.WithString("nextLink", mcp.Description("Pagination URL from previous response")))

		tool := mcp.NewTool(ep.ToolName, opts...)

		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			nextLink := req.GetString("nextLink", "")
			if nextLink != "" {
				data, err := gc.Get(nextLink, nil, ep.Headers)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return mcp.NewToolResultText(string(data)), nil
			}

			path := ep.PathPattern
			for _, pp := range pathParams {
				val, err := req.RequireString(pp[1])
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				path = strings.ReplaceAll(path, "{"+pp[1]+"}", val)
			}

			qp := map[string]string{}
			for _, op := range odataParams {
				if v := req.GetString(op, ""); v != "" {
					qp["$"+op] = v
				}
			}

			data, err := gc.Get(path, qp, ep.Headers)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		})
	}
}
```

- [ ] **Step 4: Verify it compiles**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
```

Expected: builds.

- [ ] **Step 5: Commit**

```bash
git add go/endpoints.json go/internal/tools/register.go go/internal/tools/endpoints.go
git commit -m "feat(go): dynamic endpoint tools from embedded endpoints.json"
```

---

### Task 6: Auth Tools

**Files:**
- Create: `go/internal/tools/auth.go`

- [ ] **Step 1: Create auth tools**

Create `go/internal/tools/auth.go`:
```go
package tools

import (
	"context"
	"encoding/json"

	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAuthTools(s *server.MCPServer, am *auth.AuthManager) {
	// planner-login
	loginTool := mcp.NewTool("planner-login",
		mcp.WithDescription("Authenticate with Microsoft. Opens the browser for sign-in and waits for completion."),
		mcp.WithBoolean("force", mcp.Description("Force a new login even if already logged in")),
	)
	s.AddTool(loginTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		force := req.GetBool("force", false)
		if !force {
			status, _ := am.TestLogin()
			if s, ok := status["success"].(bool); ok && s {
				return jsonResult(status), nil
			}
		}
		if err := am.AcquireTokenInteractively(); err != nil {
			return mcp.NewToolResultError("Auth failed: " + err.Error()), nil
		}
		status, _ := am.TestLogin()
		status["status"] = "Login successful"
		return jsonResult(status), nil
	})

	// planner-logout
	logoutTool := mcp.NewTool("planner-logout",
		mcp.WithDescription("Log out from Microsoft"),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(logoutTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := am.Logout(); err != nil {
			return mcp.NewToolResultError("Logout failed: " + err.Error()), nil
		}
		return jsonResult(map[string]any{"message": "Logged out"}), nil
	})

	// planner-auth-status
	statusTool := mcp.NewTool("planner-auth-status",
		mcp.WithDescription("Check Microsoft auth status"),
		mcp.WithReadOnlyHintAnnotation(true),
	)
	s.AddTool(statusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, _ := am.TestLogin()
		return jsonResult(result), nil
	})
}

func jsonResult(v any) *mcp.CallToolResult {
	data, _ := json.Marshal(v)
	return mcp.NewToolResultText(string(data))
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add go/internal/tools/auth.go
git commit -m "feat(go): auth tools (login, logout, status)"
```

---

### Task 7: Plan Tools

**Files:**
- Create: `go/internal/tools/plans.go`

- [ ] **Step 1: Create plan tools**

Create `go/internal/tools/plans.go`:
```go
package tools

import (
	"context"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerPlanTools(s *server.MCPServer, gc *graph.Client) {
	// create-plan
	s.AddTool(
		mcp.NewTool("create-plan",
			mcp.WithDescription("Create a new Planner plan"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("title", mcp.Required(), mcp.Description("Plan title")),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Group ID that owns the plan")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			title, _ := req.RequireString("title")
			owner, _ := req.RequireString("owner")
			data, err := gc.Post("/planner/plans", map[string]any{"title": title, "owner": owner})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// update-plan
	s.AddTool(
		mcp.NewTool("update-plan",
			mcp.WithDescription("Update plan title or categories"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("plan-id", mcp.Required(), mcp.Description("Plan ID")),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithObject("categoryDescriptions", mcp.Description("Category label map")),
			mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			planID, _ := req.RequireString("plan-id")
			etag := req.GetString("etag", "")
			if etag == "" {
				var err error
				etag, err = gc.GetEtag("/planner/plans/" + planID)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			body := map[string]any{}
			if v := req.GetString("title", ""); v != "" {
				body["title"] = v
			}
			args := req.GetArguments()
			if v, ok := args["categoryDescriptions"]; ok {
				body["categoryDescriptions"] = v
			}
			data, err := gc.Patch("/planner/plans/"+planID, body, etag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// delete-plan
	s.AddTool(
		mcp.NewTool("delete-plan",
			mcp.WithDescription("Delete a plan"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("plan-id", mcp.Required(), mcp.Description("Plan ID")),
			mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			planID, _ := req.RequireString("plan-id")
			etag := req.GetString("etag", "")
			if etag == "" {
				var err error
				etag, err = gc.GetEtag("/planner/plans/" + planID)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			data, err := gc.Delete("/planner/plans/"+planID, etag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add go/internal/tools/plans.go
git commit -m "feat(go): plan CRUD tools"
```

---

### Task 8: Bucket Tools

**Files:**
- Create: `go/internal/tools/buckets.go`

- [ ] **Step 1: Create bucket tools**

Create `go/internal/tools/buckets.go`:
```go
package tools

import (
	"context"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerBucketTools(s *server.MCPServer, gc *graph.Client) {
	s.AddTool(
		mcp.NewTool("create-bucket",
			mcp.WithDescription("Create a new bucket"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("name", mcp.Required(), mcp.Description("Bucket name")),
			mcp.WithString("planId", mcp.Required(), mcp.Description("Plan ID")),
			mcp.WithString("orderHint", mcp.Description("Order hint (e.g. ' !' for first)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			planID, _ := req.RequireString("planId")
			body := map[string]any{"name": name, "planId": planID}
			if oh := req.GetString("orderHint", ""); oh != "" {
				body["orderHint"] = oh
			}
			data, err := gc.Post("/planner/buckets", body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("update-bucket",
			mcp.WithDescription("Update a bucket"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("bucket-id", mcp.Required(), mcp.Description("Bucket ID")),
			mcp.WithString("name", mcp.Description("New name")),
			mcp.WithString("orderHint", mcp.Description("Order hint")),
			mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			bucketID, _ := req.RequireString("bucket-id")
			etag := req.GetString("etag", "")
			if etag == "" {
				var err error
				etag, err = gc.GetEtag("/planner/buckets/" + bucketID)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			body := map[string]any{}
			if v := req.GetString("name", ""); v != "" {
				body["name"] = v
			}
			if v := req.GetString("orderHint", ""); v != "" {
				body["orderHint"] = v
			}
			data, err := gc.Patch("/planner/buckets/"+bucketID, body, etag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("delete-bucket",
			mcp.WithDescription("Delete a bucket"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("bucket-id", mcp.Required(), mcp.Description("Bucket ID")),
			mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			bucketID, _ := req.RequireString("bucket-id")
			etag := req.GetString("etag", "")
			if etag == "" {
				var err error
				etag, err = gc.GetEtag("/planner/buckets/" + bucketID)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			data, err := gc.Delete("/planner/buckets/"+bucketID, etag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}
```

- [ ] **Step 2: Build and commit**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
git add go/internal/tools/buckets.go
git commit -m "feat(go): bucket CRUD tools"
```

---

### Task 9: Task Tools

**Files:**
- Create: `go/internal/tools/tasks.go`

- [ ] **Step 1: Create task tools (create, update, delete, assign, unassign, move)**

Create `go/internal/tools/tasks.go` with all 6 task tools. This is the largest file — follows the same patterns as plans/buckets but with more parameters.

Key details:
- `create-task`: POST `/planner/tasks` with planId, title, bucketId(opt), assigneeIds(opt array), priority(opt 0-3), startDateTime(opt), dueDateTime(opt), percentComplete(opt 0-100), orderHint(opt)
- `assign-task`: GET task → merge assignment `{userId: {"@odata.type": "#microsoft.graph.plannerAssignment", "orderHint": " !"}}` → PATCH
- `unassign-task`: GET task → set assignment to null → PATCH
- `move-task`: PATCH with `{"bucketId": newBucketId}`
- All mutations auto-fetch ETag if not provided
- Priority: number 0-3 (Urgent, Important, Medium, Low)
- percentComplete: number 0/50/100

- [ ] **Step 2: Build and commit**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
git add go/internal/tools/tasks.go
git commit -m "feat(go): task CRUD + assign/unassign/move tools"
```

---

### Task 10: Task Detail Tools

**Files:**
- Create: `go/internal/tools/taskdetails.go`

- [ ] **Step 1: Create task detail tools**

Three tools:
- `update-task-details`: PATCH `/planner/tasks/{task-id}/details` with description, previewType, checklist(object), references(object)
- `add-checklist-item`: GET details → generate UUID key → merge checklist item → PATCH
- `toggle-checklist-item`: GET details → toggle `isChecked` on item by ID → PATCH

Key: checklist items use format `{guid: {"@odata.type": "microsoft.graph.plannerChecklistItem", "title": "...", "isChecked": bool}}`

UUID generation: use `crypto/rand` to generate a v4 UUID string.

- [ ] **Step 2: Build and commit**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build ./...
git add go/internal/tools/taskdetails.go
git commit -m "feat(go): task detail tools (checklist, description)"
```

---

### Task 11: Wire Main + MCP Server

**Files:**
- Modify: `go/cmd/plannner-connector/main.go`

- [ ] **Step 1: Complete main.go**

Replace the skeleton with the full implementation:
```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/TestaVivaDK/plannner-connector/internal/logger"
	"github.com/TestaVivaDK/plannner-connector/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging to stderr")
	doLogin := flag.Bool("login", false, "Login and exit")
	doLogout := flag.Bool("logout", false, "Logout and exit")
	verifyLogin := flag.Bool("verify-login", false, "Verify login and exit")
	flag.Parse()

	logger.Init(*verbose)

	clientID := os.Getenv("PLANNER_MCP_CLIENT_ID")
	tenantID := os.Getenv("PLANNER_MCP_TENANT_ID")
	if clientID == "" || tenantID == "" {
		fmt.Fprintln(os.Stderr, "Missing PLANNER_MCP_CLIENT_ID or PLANNER_MCP_TENANT_ID")
		os.Exit(1)
	}

	am := auth.NewAuthManager(clientID, tenantID)
	am.LoadTokenCache()

	if *doLogin {
		if err := am.AcquireTokenInteractively(); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		result, _ := am.TestLogin()
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return
	}

	if *doLogout {
		am.Logout()
		fmt.Println(`{"message":"Logged out successfully"}`)
		return
	}

	if *verifyLogin {
		result, _ := am.TestLogin()
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return
	}

	// Start MCP server
	gc := graph.NewClient(am)
	s := server.NewMCPServer(
		"PlannerMCP",
		"2.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	tools.RegisterAll(s, am, gc)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and test locally**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build -o plannner-connector ./cmd/plannner-connector
PLANNER_MCP_CLIENT_ID=test PLANNER_MCP_TENANT_ID=test ./plannner-connector --verify-login
```

Expected: outputs JSON with `{"success":false,"message":"..."}` (no real credentials).

- [ ] **Step 3: Commit**

```bash
git add go/cmd/plannner-connector/main.go
git commit -m "feat(go): wire main entry point with MCP server"
```

---

### Task 12: Manifest, Makefile, GitHub Actions

**Files:**
- Create: `go/manifest.json` (binary bundle manifest)
- Create: `go/Makefile`
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Create binary bundle manifest**

Create `go/manifest.json` with the spec from the design doc (the full JSON with `server.type: "binary"`, `platform_overrides`, `user_config`, and all 27 tools).

- [ ] **Step 2: Create Makefile**

Create `go/Makefile`:
```makefile
VERSION ?= $(shell jq -r .version manifest.json)
LDFLAGS := -s -w
BIN     := server

.PHONY: build package clean test bump-version

build:
	mkdir -p $(BIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector-darwin ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector.exe ./cmd/plannner-connector

package: build
	rm -f plannner-connector.mcpb
	zip -j plannner-connector.mcpb manifest.json
	zip -r plannner-connector.mcpb $(BIN)/

test:
	go test ./...

clean:
	rm -rf $(BIN) plannner-connector.mcpb

bump-version:
	@test -n "$(V)" || (echo "Usage: make bump-version V=x.y.z" && exit 1)
	jq '.version = "$(V)"' manifest.json > tmp.json && mv tmp.json manifest.json
	@echo "Version set to $(V)"

version:
	@echo $(VERSION)
```

- [ ] **Step 3: Update GitHub Actions**

Replace `.github/workflows/release.yml`:
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Extract version from tag
        id: version
        run: echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"

      - name: Set version
        working-directory: go
        run: make bump-version V=${{ steps.version.outputs.VERSION }}

      - name: Build and package
        working-directory: go
        run: make package

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: go/plannner-connector.mcpb
          generate_release_notes: true
```

- [ ] **Step 4: Build and package locally to verify**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
make package
ls -lh plannner-connector.mcpb server/
```

Expected: three binaries in `server/` and one `.mcpb` zip.

- [ ] **Step 5: Commit**

```bash
git add go/manifest.json go/Makefile .github/workflows/release.yml
git commit -m "feat(go): manifest, Makefile, and release workflow for binary bundle"
```

---

### Task 13: End-to-End Smoke Test

- [ ] **Step 1: Build Linux binary and test MCP handshake**

```bash
cd /home/mlu/Documents/project/plannner-connector/go
go build -o plannner-connector ./cmd/plannner-connector
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | PLANNER_MCP_CLIENT_ID=test PLANNER_MCP_TENANT_ID=test ./plannner-connector
```

Expected: JSON-RPC response with server info and capabilities.

- [ ] **Step 2: Test tools/list**

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' | PLANNER_MCP_CLIENT_ID=test PLANNER_MCP_TENANT_ID=test ./plannner-connector
```

Expected: JSON with all 27 tools listed.

- [ ] **Step 3: Package final .mcpb and verify contents**

```bash
make package
unzip -l plannner-connector.mcpb
```

Expected: manifest.json + 3 binaries under `server/`.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A go/
git commit -m "feat(go): smoke test verified, Go rewrite complete"
```
