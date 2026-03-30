# Go Rewrite Design Spec

## Goal

Rewrite the plannner-connector MCP server from TypeScript/Node.js to Go. Ship cross-platform binaries (macOS arm64, Linux amd64, Windows amd64) inside a single `.mcpb` bundle using the official binary bundle format.

## Why

- Eliminates Node.js runtime dependency for end users
- Single static binary per platform — no `node_modules/`
- Faster startup, smaller memory footprint
- Solves the MSAL/scope issues by using direct HTTP calls (already done in TS, carries over)

## Architecture

```
go/
├── cmd/plannner-connector/
│   └── main.go                # Entry point, CLI flags, MCP server startup
├── internal/
│   ├── auth/
│   │   └── auth.go            # OAuth PKCE flow, token cache, refresh, browser launch
│   ├── graph/
│   │   └── client.go          # Graph API HTTP client (retry on 429, ETag support)
│   ├── tools/
│   │   ├── auth.go            # planner-login, planner-logout, planner-auth-status
│   │   ├── endpoints.go       # Dynamic endpoint tools from embedded endpoints.json
│   │   ├── plans.go           # create-plan, update-plan, delete-plan
│   │   ├── buckets.go         # create-bucket, update-bucket, delete-bucket
│   │   ├── tasks.go           # create-task, update-task, delete-task, assign-task, unassign-task, move-task
│   │   └── taskdetails.go     # update-task-details, add-checklist-item, toggle-checklist-item
│   └── logger/
│       └── logger.go          # slog-based structured logging
├── endpoints.json             # Embedded via go:embed
├── go.mod
└── go.sum
```

## Dependencies

| Purpose | Package |
|---------|---------|
| MCP server + stdio transport | `github.com/mark3labs/mcp-go` |
| Logging | stdlib `log/slog` |
| HTTP client | stdlib `net/http` |
| JSON | stdlib `encoding/json` |
| CLI flags | stdlib `flag` |
| PKCE crypto | stdlib `crypto/sha256`, `crypto/rand` |
| Loopback server | stdlib `net/http` |
| Browser launch | stdlib `os/exec` |
| Embed endpoints.json | stdlib `embed` |

Only one external dependency: `mcp-go`.

## MCPB Binary Bundle

### manifest.json

```json
{
  "manifest_version": "0.3",
  "name": "plannner-connector",
  "display_name": "Microsoft Planner",
  "version": "2.0.0",
  "description": "Connect Claude to Microsoft Planner — manage plans, buckets, tasks, assignments, and checklists via the Graph API.",
  "long_description": "Full CRUD access to Microsoft Planner through the Graph API. Create and manage plans, organize work with buckets, create and assign tasks, track progress, and manage checklists. Authenticates via interactive browser login.",
  "author": { "name": "TestaViva" },
  "license": "MIT",
  "server": {
    "type": "binary",
    "entry_point": "server/plannner-connector",
    "mcp_config": {
      "command": "server/plannner-connector",
      "args": [],
      "env": {
        "PLANNER_MCP_CLIENT_ID": "${user_config.client_id}",
        "PLANNER_MCP_TENANT_ID": "${user_config.tenant_id}"
      },
      "platform_overrides": {
        "win32": {
          "command": "server/plannner-connector.exe"
        },
        "darwin": {
          "command": "server/plannner-connector-darwin"
        }
      }
    }
  },
  "user_config": {
    "client_id": {
      "type": "string",
      "title": "Azure AD Client ID",
      "description": "Application (client) ID from your Azure AD app registration",
      "required": true
    },
    "tenant_id": {
      "type": "string",
      "title": "Azure AD Tenant ID",
      "description": "Directory (tenant) ID from your Azure AD app registration",
      "required": true
    }
  },
  "compatibility": {
    "platforms": ["darwin", "win32", "linux"]
  },
  "tools": [
    { "name": "planner-login", "description": "Authenticate with Microsoft (opens browser)" },
    { "name": "planner-logout", "description": "Log out from Microsoft" },
    { "name": "planner-auth-status", "description": "Check authentication status" },
    { "name": "list-my-plans", "description": "List all Planner plans you are a member of" },
    { "name": "get-plan", "description": "Get details of a specific plan" },
    { "name": "create-plan", "description": "Create a new Planner plan" },
    { "name": "update-plan", "description": "Update plan title or categories" },
    { "name": "delete-plan", "description": "Delete a plan" },
    { "name": "list-buckets", "description": "List buckets in a plan" },
    { "name": "create-bucket", "description": "Create a new bucket" },
    { "name": "update-bucket", "description": "Update a bucket" },
    { "name": "delete-bucket", "description": "Delete a bucket" },
    { "name": "list-plan-tasks", "description": "List tasks in a plan" },
    { "name": "list-bucket-tasks", "description": "List tasks in a bucket" },
    { "name": "get-task", "description": "Get details of a task" },
    { "name": "create-task", "description": "Create a new task" },
    { "name": "update-task", "description": "Update a task" },
    { "name": "delete-task", "description": "Delete a task" },
    { "name": "assign-task", "description": "Assign a user to a task" },
    { "name": "unassign-task", "description": "Remove a user from a task" },
    { "name": "move-task", "description": "Move a task to a different bucket" },
    { "name": "get-task-details", "description": "Get task description and checklist" },
    { "name": "update-task-details", "description": "Update task description, checklist, or references" },
    { "name": "add-checklist-item", "description": "Add a checklist item to a task" },
    { "name": "toggle-checklist-item", "description": "Toggle a checklist item's checked state" },
    { "name": "list-groups", "description": "List Microsoft 365 groups" },
    { "name": "list-group-plans", "description": "List plans in a group" }
  ],
  "keywords": ["microsoft", "planner", "tasks", "project-management", "graph-api", "office365"]
}
```

### Bundle Structure

```
plannner-connector.mcpb (zip):
├── manifest.json
└── server/
    ├── plannner-connector          # Linux amd64
    ├── plannner-connector-darwin   # macOS arm64
    └── plannner-connector.exe      # Windows amd64
```

## Components

### 1. Auth (internal/auth/auth.go)

Direct port of the current TypeScript auth.ts. No MSAL — all direct HTTP.

- `AuthManager` struct with clientId, tenantId, scopes, tokens
- `NewAuthManager(clientId, tenantId)` constructor
- `GetToken()` — returns cached token, tries refresh, falls back to interactive login
- `AcquireTokenInteractively()` — PKCE auth code flow:
  - Generate verifier (32 random bytes, base64url)
  - SHA256 challenge
  - Start loopback HTTP server on 127.0.0.1:0
  - Build authorize URL manually
  - Open browser (platform-specific: `open`, `xdg-open`, `cmd /c start`)
  - Wait for redirect with auth code
  - POST to token endpoint with explicit scope
  - 5-minute timeout
- `RefreshAccessToken()` — POST with grant_type=refresh_token
- `LoadTokenCache()` / `SaveTokenCache()` — JSON file with 0600 permissions
- `TestLogin()` — GET /me to verify token
- `Logout()` — clear tokens, delete cache file

Scopes (consistent everywhere):
```
https://graph.microsoft.com/Tasks.ReadWrite
https://graph.microsoft.com/Group.Read.All
https://graph.microsoft.com/User.Read
offline_access openid profile
```

### 2. Graph Client (internal/graph/client.go)

Direct port of graph-client.ts.

- `Client` struct with authManager reference
- `Get(path, queryParams, extraHeaders)` — GET with Bearer token
- `Post(path, body)` — POST for resource creation
- `Patch(path, body, etag)` — PATCH with If-Match header
- `Delete(path, etag)` — DELETE with If-Match header
- `GetEtag(path)` — fetch resource, extract `@odata.etag`
- Auto-retry on 429 with Retry-After header (once)
- Clear error on 412 (ETag mismatch)
- Base URL: `https://graph.microsoft.com/v1.0`

### 3. Tools (internal/tools/*.go)

All 27 tools, same names, same parameters, same behavior as TypeScript version.

Each tool file registers its tools on the mcp-go server using the server's tool registration API. Parameters use mcp-go's schema types.

### 4. Logger (internal/logger/logger.go)

- `slog` with file handler (logs/planner-mcp.log)
- Optional stderr handler when `--verbose` flag set
- Same format: timestamp + level + message

### 5. Entry Point (cmd/plannner-connector/main.go)

- Parse flags: `--verbose`, `--login`, `--logout`, `--verify-login`
- Read env vars: `PLANNER_MCP_CLIENT_ID`, `PLANNER_MCP_TENANT_ID`
- Create AuthManager, load token cache
- Handle CLI modes (login/logout/verify) with early exit
- Default: create MCP server, register all tools, run stdio transport

## Build

### Makefile

```makefile
VERSION ?= $(shell jq -r .version manifest.json)
LDFLAGS := -s -w

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o server/plannner-connector ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o server/plannner-connector-darwin ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o server/plannner-connector.exe ./cmd/plannner-connector

package: build
	rm -f plannner-connector.mcpb
	zip plannner-connector.mcpb manifest.json server/plannner-connector server/plannner-connector-darwin server/plannner-connector.exe

clean:
	rm -rf server/plannner-connector* plannner-connector.mcpb

bump-version:
	@test -n "$(V)" || (echo "Usage: make bump-version V=x.y.z" && exit 1)
	jq '.version = "$(V)"' manifest.json > tmp.json && mv tmp.json manifest.json
```

`-ldflags="-s -w"` strips debug symbols for smaller binaries.
`CGO_ENABLED=0` ensures fully static binaries.

### GitHub Actions

Same tag-based trigger (`v*`), but:
1. Setup Go instead of Node
2. `make package`
3. Create release with `.mcpb` attached

## Migration Notes

- The `go/` directory lives alongside the existing `src/` during development
- Once validated, the TypeScript source can be archived
- `endpoints.json` is shared — embedded in Go binary via `go:embed`
- Token cache format stays the same JSON structure (backward compatible)
- All tool names, parameters, and response formats are identical
