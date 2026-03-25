# Microsoft Planner MCP Connector — Design Spec

## Overview

A standalone MCP server that enables Claude Desktop/Cowork to interact with Microsoft Planner via the Microsoft Graph API. Supports full CRUD on plans, buckets, tasks, and task details. Uses the legacy Planner API (`/planner/...`), delegated auth with device code flow, and both stdio and HTTP transports.

Auth patterns forked from the existing `ms-365-mcp-server` project.

## Architecture: Hybrid Approach

Simple read/discover operations are defined in `endpoints.json` and auto-registered as MCP tools at startup. Complex mutations (create, update, delete) are hand-written with explicit Zod schemas and handlers to accommodate Planner API quirks: mandatory ETags on PATCH/DELETE, nested assignment objects, GUID-keyed checklist items.

## Project Structure

```
plannner-connector/
├── src/
│   ├── index.ts              # Entry point (CLI parsing, startup)
│   ├── server.ts             # MCP server setup (stdio + HTTP)
│   ├── auth.ts               # MSAL auth manager (device code flow, token cache)
│   ├── graph-client.ts       # Graph API HTTP client with token injection
│   ├── endpoints.json        # Simple CRUD endpoint definitions
│   ├── endpoint-tools.ts     # Auto-registers tools from endpoints.json
│   ├── tools/
│   │   ├── plans.ts          # Plan create/update/delete
│   │   ├── buckets.ts        # Bucket create/update/delete
│   │   ├── tasks.ts          # Task create/update/delete/assign/unassign
│   │   └── task-details.ts   # Task details, checklist, references
│   ├── auth-tools.ts         # Login/logout/status MCP tools
│   ├── logger.ts             # Winston logger (stderr)
│   └── cli.ts                # CLI argument parsing (commander)
├── package.json
├── tsconfig.json
├── tsup.config.ts
├── vitest.config.js
├── .env.example
└── .gitignore
```

## Authentication

- **Library:** `@azure/msal-node` PublicClientApplication
- **Flow:** Device code (delegated, user signs in)
- **Token cache:** File-based (`.token-cache.json`), optional keytar for OS keychain
- **Scopes:** `Tasks.ReadWrite`, `Group.Read.All`, `User.Read`
- **Config env vars:**
  - `PLANNER_MCP_CLIENT_ID` — Azure AD app registration (required)
  - `PLANNER_MCP_TENANT_ID` — defaults to `common`
  - `PLANNER_MCP_TOKEN_CACHE_PATH` — optional file path override

### Auth MCP Tools

| Tool | Description |
|------|-------------|
| `planner-login` | Triggers device code flow |
| `planner-logout` | Clears token cache |
| `planner-auth-status` | Shows current auth state and user info |

## Endpoint-Driven Tools (endpoints.json)

### endpoints.json Schema

Each entry in `endpoints.json` has the following fields:

```typescript
interface EndpointConfig {
  pathPattern: string;    // Graph API path with {param} placeholders
  method: string;         // HTTP method: "get"
  toolName: string;       // MCP tool name
  scopes: string[];       // Required OAuth scopes
  llmTip?: string;        // Optional guidance for the LLM
  headers?: Record<string, string>; // Optional extra headers (e.g., ConsistencyLevel)
}
```

Fields from `ms-365-mcp-server` that are **not carried over**: `workScopes`, `returnDownloadUrl`, `supportsTimezone`, `supportsExpandExtendedProperties`, `skipEncoding`, `contentType`. These are not relevant to Planner endpoints.

### Auto-Registration Behavior (`endpoint-tools.ts`)

1. Path parameters (e.g., `{plan-id}`) are extracted by regex matching `\{([^}]+)\}` and become required string inputs in the Zod schema.
2. Standard OData query parameters are added as optional string inputs: `$filter`, `$select`, `$top`, `$orderby`, `$expand`, `$count`, `$search`.
3. If `headers` is defined on the endpoint, those headers are merged into the request (e.g., `ConsistencyLevel: eventual` for `list-groups`).
4. Responses return the raw JSON from Graph API. Paginated responses include `@odata.nextLink` in the output — the LLM can pass it as a `nextLink` optional parameter to fetch subsequent pages.

### Tool Definitions

Simple read/discover operations auto-registered from JSON definitions:

| Tool Name | Method | Path | Scopes |
|-----------|--------|------|--------|
| `list-my-plans` | GET | `/me/planner/plans` | Tasks.ReadWrite |
| `get-plan` | GET | `/planner/plans/{plan-id}` | Tasks.ReadWrite |
| `list-buckets` | GET | `/planner/plans/{plan-id}/buckets` | Tasks.ReadWrite |
| `get-bucket` | GET | `/planner/buckets/{bucket-id}` | Tasks.ReadWrite |
| `list-plan-tasks` | GET | `/planner/plans/{plan-id}/tasks` | Tasks.ReadWrite |
| `list-bucket-tasks` | GET | `/planner/buckets/{bucket-id}/tasks` | Tasks.ReadWrite |
| `get-task` | GET | `/planner/tasks/{task-id}` | Tasks.ReadWrite |
| `get-task-details` | GET | `/planner/tasks/{task-id}/details` | Tasks.ReadWrite |
| `list-groups` | GET | `/groups` | Group.Read.All |
| `list-group-plans` | GET | `/groups/{group-id}/planner/plans` | Tasks.ReadWrite, Group.Read.All |

The `list-groups` tool includes an LLM tip to use `$filter=groupTypes/any(g:g eq 'Unified')` for M365 groups and `$search` with `ConsistencyLevel:eventual`.

## Hand-Written Tools

### Plans (`tools/plans.ts`)

| Tool | Method | Path | Notes |
|------|--------|------|-------|
| `create-plan` | POST | `/planner/plans` | Body: `{owner: groupId, title}`. Owner must be a Group ID. |
| `update-plan` | PATCH | `/planner/plans/{id}` | ETag required. Supports title, category descriptions. |
| `delete-plan` | DELETE | `/planner/plans/{id}` | ETag required. |

### Buckets (`tools/buckets.ts`)

| Tool | Method | Path | Notes |
|------|--------|------|-------|
| `create-bucket` | POST | `/planner/buckets` | Body: `{name, planId, orderHint}` |
| `update-bucket` | PATCH | `/planner/buckets/{id}` | ETag required. Supports name, orderHint. |
| `delete-bucket` | DELETE | `/planner/buckets/{id}` | ETag required. |

### Tasks (`tools/tasks.ts`)

| Tool | Method | Path | Notes |
|------|--------|------|-------|
| `create-task` | POST | `/planner/tasks` | Body: `{planId, bucketId, title}` + optional assignments, priority, dates, percentComplete, orderHint. Assignments format: `{"userId": {"@odata.type": "#microsoft.graph.plannerAssignment", "orderHint": " !"}}` |
| `update-task` | PATCH | `/planner/tasks/{id}` | ETag required. All mutable fields. LLM tip: "Get task first for ETag." |
| `delete-task` | DELETE | `/planner/tasks/{id}` | ETag required. |
| `assign-task` | — | — | Convenience: fetches task, merges assignment, PATCHes with ETag. |
| `unassign-task` | — | — | Convenience: fetches task, sets assignment to null, PATCHes. |
| `move-task` | — | — | Convenience: fetches task, updates bucketId, PATCHes with ETag. |

### Task Details (`tools/task-details.ts`)

| Tool | Method | Path | Notes |
|------|--------|------|-------|
| `update-task-details` | PATCH | `/planner/tasks/{id}/details` | ETag required. Supports description, previewType, checklist, references. Checklist uses GUID keys: `{"guid": {"title": "...", "isChecked": false}}` |
| `add-checklist-item` | — | — | Convenience: fetches details, generates GUID via `crypto.randomUUID()`, adds item, PATCHes. |
| `toggle-checklist-item` | — | — | Convenience: fetches details, flips isChecked, PATCHes. |

## Graph Client

`graph-client.ts` wraps HTTP calls to `https://graph.microsoft.com/v1.0`:

### Methods

- `get(path, queryParams?)` — GET with auth header
- `post(path, body)` — POST with auth + Content-Type
- `patch(path, body, etag)` — PATCH with `If-Match` header
- `delete(path, etag)` — DELETE with `If-Match` header
- `getEtag(path)` — GET resource, extract `@odata.etag`

### ETag Strategy

Every mutation tool accepts an optional `etag` parameter. If omitted, the tool auto-fetches the resource to get the current ETag. This adds one API call but makes tools ergonomic for LLM use — Claude doesn't need to chain get-then-update.

**ETag format:** Planner returns weak ETags (e.g., `W/"JzEtVGFzay..."`) in the `@odata.etag` field. The `If-Match` header must pass through the exact value as-is — no stripping or reformatting.

**Race condition (TOCTOU):** The auto-fetch pattern has an inherent race — another client could modify the resource between the GET and PATCH/DELETE. This is an accepted trade-off for ergonomics. If the server receives a 412 Precondition Failed, it surfaces the error to the LLM with a clear message ("Resource was modified by another user, please retry"). No automatic retry — the LLM should re-fetch and re-attempt.

### Error Handling

Graph API errors (`{error: {code, message}}`) are mapped to clear MCP error responses preserving the Graph error code and message.

**Throttling:** If Graph API returns 429 Too Many Requests, the client reads the `Retry-After` header and waits before retrying once. If the retry also fails, the error is surfaced to the LLM.

### HTTP

Uses Node built-in `fetch` (Node 18+). No external HTTP library.

## Transport & Server

### Stdio (default)

Standard MCP stdio transport. Claude Desktop config:

```json
{
  "mcpServers": {
    "planner": {
      "command": "node",
      "args": ["path/to/plannner-connector/dist/index.js"]
    }
  }
}
```

### HTTP

Express-based with Streamable HTTP transport from MCP SDK. Activated with `--http [host:]port`.

**HTTP auth model (follow-up):** In the initial implementation, HTTP mode is unauthenticated (local dev only). A follow-up task will add the OAuth proxy pattern from `ms-365-mcp-server` — `/authorize`, `/token`, and `/.well-known/oauth-authorization-server` endpoints with bearer token validation — for production HTTP deployments.

### CLI Flags

| Flag | Description |
|------|-------------|
| `--login` | Run device code flow and exit |
| `--logout` | Clear tokens and exit |
| `--verify-login` | Test Graph connection and exit |
| `--http [host:]port` | Start HTTP server instead of stdio |
| `-v, --verbose` | Enable debug logging |

### Logging

Winston to stderr. Console logging enabled only in HTTP or verbose mode.

## Dependencies

### Runtime

- `@modelcontextprotocol/sdk` — MCP server framework
- `@azure/msal-node` — Microsoft auth
- `zod` — input validation
- `commander` — CLI parsing
- `dotenv` — env var loading
- `express` — HTTP transport
- `winston` — logging

### Optional

- `keytar` — OS keychain (graceful fallback to file)

### Dev

- `typescript`, `tsup`, `tsx`
- `vitest`, `eslint`, `prettier`

### Build

`tsup` to `dist/`. Target Node 18+.

## Test Strategy

- **Unit tests:** ETag auto-fetch logic, Graph client error handling (412, 429), endpoint-tools auto-registration with fixture JSON, Zod schema generation from path patterns.
- **Integration tests:** Mocked Graph API responses using vitest mocks on `fetch`. Test each hand-written tool handler with realistic Graph payloads (including ETags, pagination, error responses).
- **No live API tests in CI.** Live testing is manual via `--login` + MCP Inspector.

## Azure AD App Registration Requirements

The user needs an Azure AD app registration with:

1. **Type:** Public client (no client secret needed for stdio/device code flow; client secret needed for HTTP OAuth proxy mode)
2. **Redirect URIs:** Not required for device code flow. For HTTP mode, add `http://localhost:3000/callback` (or your configured host:port).
3. **API permissions (delegated):**
   - `Tasks.ReadWrite` — read/write Planner tasks and plans
   - `Group.Read.All` — discover M365 groups that own plans
   - `User.Read` — basic user profile for auth verification
4. **Supported account types:** Based on tenant config (single or multi-tenant)
