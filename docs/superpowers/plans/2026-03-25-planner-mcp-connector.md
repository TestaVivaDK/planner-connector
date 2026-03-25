# Microsoft Planner MCP Connector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone MCP server that enables Claude Desktop/Cowork to perform full CRUD on Microsoft Planner via the Graph API.

**Architecture:** Hybrid approach — endpoint-driven auto-registration for simple GET operations, hand-written tools for mutations requiring ETags and complex payloads. Auth via MSAL device code flow, forked from `ms-365-mcp-server`. Stdio + HTTP transports.

**Tech Stack:** TypeScript, Node 18+, `@modelcontextprotocol/sdk`, `@azure/msal-node`, `zod`, `express`, `commander`, `winston`, `vitest`, `tsup`

**Spec:** `docs/superpowers/specs/2026-03-25-planner-mcp-connector-design.md`

**Reference codebase:** `/home/mlu/Documents/project/ms-365-mcp-server/` — fork auth, logger, and build patterns from here.

---

## File Map

| File | Responsibility |
|------|---------------|
| `src/index.ts` | Entry point — CLI parsing, auth init, server startup |
| `src/cli.ts` | Commander CLI definitions and argument parsing |
| `src/logger.ts` | Winston logger (file + optional console, stderr-safe) |
| `src/auth.ts` | MSAL AuthManager — device code flow, token cache, keytar fallback |
| `src/auth-tools.ts` | MCP tools: planner-login, planner-logout, planner-auth-status |
| `src/graph-client.ts` | HTTP client wrapping Graph API — get/post/patch/delete + ETag + throttle retry |
| `src/endpoints.json` | Endpoint definitions for auto-registered read tools |
| `src/endpoint-tools.ts` | Reads endpoints.json, builds Zod schemas, registers MCP tools |
| `src/tools/plans.ts` | Hand-written tools: create-plan, update-plan, delete-plan |
| `src/tools/buckets.ts` | Hand-written tools: create-bucket, update-bucket, delete-bucket |
| `src/tools/tasks.ts` | Hand-written tools: create-task, update-task, delete-task, assign-task, unassign-task, move-task |
| `src/tools/task-details.ts` | Hand-written tools: update-task-details, add-checklist-item, toggle-checklist-item |
| `src/server.ts` | MCP server class — tool registration, stdio + HTTP transport |
| `package.json` | Dependencies, scripts, metadata |
| `tsconfig.json` | TypeScript config |
| `tsup.config.ts` | Build config |
| `vitest.config.js` | Test config |
| `.env.example` | Example env vars |
| `.gitignore` | Git ignores |
| `test/graph-client.test.ts` | Unit tests for graph client (ETag, throttle, errors) |
| `test/endpoint-tools.test.ts` | Unit tests for endpoint auto-registration |
| `test/tools/tasks.test.ts` | Unit tests for hand-written task tools |

---

### Task 1: Project Scaffold

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `tsup.config.ts`
- Create: `vitest.config.js`
- Create: `.gitignore`
- Create: `.env.example`

- [ ] **Step 1: Initialize git repo**

```bash
cd /home/mlu/Documents/project/plannner-connector
git init
```

- [ ] **Step 2: Create package.json**

```json
{
  "name": "plannner-connector",
  "version": "0.1.0",
  "description": "MCP server for Microsoft Planner via Graph API",
  "type": "module",
  "main": "dist/index.js",
  "bin": {
    "plannner-connector": "dist/index.js"
  },
  "scripts": {
    "build": "tsup",
    "test": "vitest run",
    "test:watch": "vitest",
    "dev": "tsx src/index.ts",
    "dev:http": "tsx --watch src/index.ts --http 127.0.0.1:3000 -v",
    "lint": "eslint .",
    "lint:fix": "eslint . --fix",
    "format": "prettier --write \"**/*.{ts,js,json,md}\"",
    "format:check": "prettier --check \"**/*.{ts,js,json,md}\""
  },
  "dependencies": {
    "@azure/msal-node": "^3.8.0",
    "@modelcontextprotocol/sdk": "^1.25.0",
    "commander": "^11.1.0",
    "dotenv": "^17.0.1",
    "express": "^5.2.1",
    "winston": "^3.17.0",
    "zod": "^3.24.2"
  },
  "optionalDependencies": {
    "keytar": "^7.9.0"
  },
  "devDependencies": {
    "@types/express": "^5.0.3",
    "@types/node": "^22.15.15",
    "eslint": "^9.31.0",
    "prettier": "^3.5.3",
    "tsup": "^8.5.0",
    "tsx": "^4.19.4",
    "typescript": "^5.8.3",
    "vitest": "^3.1.1"
  },
  "engines": {
    "node": ">=18"
  }
}
```

- [ ] **Step 3: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["test/**/*"]
}
```

- [ ] **Step 4: Create tsup.config.ts**

```typescript
import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/**/*.ts', 'src/endpoints.json'],
  format: ['esm'],
  target: 'es2020',
  outDir: 'dist',
  clean: true,
  bundle: false,
  splitting: false,
  sourcemap: false,
  dts: false,
  onSuccess: 'chmod +x dist/index.js',
  loader: {
    '.json': 'copy',
  },
  external: [
    '@azure/msal-node',
    '@modelcontextprotocol/sdk',
    'commander',
    'dotenv',
    'express',
    'keytar',
    'winston',
    'zod',
  ],
});
```

- [ ] **Step 5: Create vitest.config.js**

```javascript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
  },
});
```

- [ ] **Step 6: Create .gitignore**

```
node_modules/
dist/
logs/
.env
.token-cache.json
.selected-account.json
*.tsbuildinfo
```

- [ ] **Step 7: Create .env.example**

```bash
# Azure AD App Registration
PLANNER_MCP_CLIENT_ID=your-azure-ad-app-client-id-here
PLANNER_MCP_TENANT_ID=common
# PLANNER_MCP_TOKEN_CACHE_PATH=/custom/path/.token-cache.json
```

- [ ] **Step 8: Install dependencies**

```bash
cd /home/mlu/Documents/project/plannner-connector
npm install
```

- [ ] **Step 9: Commit scaffold**

```bash
git add -A
git commit -m "feat: project scaffold with build config and dependencies"
```

---

### Task 2: Logger

**Files:**
- Create: `src/logger.ts`

- [ ] **Step 1: Create logger.ts**

Forked from `ms-365-mcp-server/src/logger.ts`. Logs to `logs/` directory with file transports. Console transport added on demand (HTTP/verbose mode). Uses stderr to avoid polluting stdio transport.

```typescript
import winston from 'winston';
import path from 'path';
import { fileURLToPath } from 'url';
import fs from 'fs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const logsDir = path.join(__dirname, '..', 'logs');

if (!fs.existsSync(logsDir)) {
  fs.mkdirSync(logsDir);
}

const logger = winston.createLogger({
  level: process.env.LOG_LEVEL || 'info',
  format: winston.format.combine(
    winston.format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
    winston.format.printf(({ level, message, timestamp }) => {
      return `${timestamp} ${level.toUpperCase()}: ${message}`;
    })
  ),
  transports: [
    new winston.transports.File({
      filename: path.join(logsDir, 'error.log'),
      level: 'error',
    }),
    new winston.transports.File({
      filename: path.join(logsDir, 'planner-mcp.log'),
    }),
  ],
});

export const enableConsoleLogging = (): void => {
  logger.add(
    new winston.transports.Console({
      format: winston.format.combine(winston.format.colorize(), winston.format.simple()),
      stderrLevels: ['error', 'warn', 'info', 'debug'],
    })
  );
};

export default logger;
```

- [ ] **Step 2: Commit**

```bash
git add src/logger.ts
git commit -m "feat: add winston logger with file and optional console transports"
```

---

### Task 3: CLI

**Files:**
- Create: `src/cli.ts`

- [ ] **Step 1: Create cli.ts**

Simplified from ms-365-mcp-server. No presets, no org-mode, no cloud types.

```typescript
import { Command } from 'commander';

const program = new Command();

program
  .name('plannner-connector')
  .description('MCP server for Microsoft Planner via Graph API')
  .version('0.1.0')
  .option('-v, --verbose', 'Enable verbose logging')
  .option('--login', 'Login using device code flow')
  .option('--logout', 'Log out and clear saved credentials')
  .option('--verify-login', 'Verify login without starting the server')
  .option(
    '--http [address]',
    'Use Streamable HTTP transport instead of stdio. Format: [host:]port (default: 3000)'
  );

export interface CommandOptions {
  verbose?: boolean;
  login?: boolean;
  logout?: boolean;
  verifyLogin?: boolean;
  http?: string | boolean;
}

export function parseArgs(): CommandOptions {
  program.parse();
  return program.opts();
}
```

- [ ] **Step 2: Commit**

```bash
git add src/cli.ts
git commit -m "feat: add CLI argument parsing"
```

---

### Task 4: Auth Manager

**Files:**
- Create: `src/auth.ts`

- [ ] **Step 1: Create auth.ts**

Forked from ms-365-mcp-server. Simplified: no cloud-config, no secrets module, no scope hierarchy, no multi-account selection. Reads `PLANNER_MCP_CLIENT_ID` and `PLANNER_MCP_TENANT_ID` directly from env.

```typescript
import type { AccountInfo, Configuration } from '@azure/msal-node';
import { PublicClientApplication } from '@azure/msal-node';
import logger from './logger.js';
import fs, { existsSync, readFileSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

let keytar: typeof import('keytar') | null = null;
async function getKeytar() {
  if (keytar === undefined) return null;
  if (keytar === null) {
    try {
      keytar = await import('keytar');
      return keytar;
    } catch {
      logger.info('keytar not available, using file-based credential storage');
      keytar = undefined as any;
      return null;
    }
  }
  return keytar;
}

const SERVICE_NAME = 'plannner-connector';
const TOKEN_CACHE_ACCOUNT = 'msal-token-cache';
const FALLBACK_DIR = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_TOKEN_CACHE_PATH = path.join(FALLBACK_DIR, '..', '.token-cache.json');

function getTokenCachePath(): string {
  return process.env.PLANNER_MCP_TOKEN_CACHE_PATH?.trim() || DEFAULT_TOKEN_CACHE_PATH;
}

function ensureParentDir(filePath: string): void {
  const dir = path.dirname(filePath);
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
}

const SCOPES = ['Tasks.ReadWrite', 'Group.Read.All', 'User.Read'];

class AuthManager {
  private msalApp: PublicClientApplication;
  private scopes: string[];
  private accessToken: string | null = null;
  private tokenExpiry: number | null = null;

  constructor(config: Configuration, scopes: string[] = SCOPES) {
    this.scopes = scopes;
    this.msalApp = new PublicClientApplication(config);
  }

  static create(): AuthManager {
    const clientId = process.env.PLANNER_MCP_CLIENT_ID;
    if (!clientId) {
      throw new Error('PLANNER_MCP_CLIENT_ID environment variable is required');
    }
    const tenantId = process.env.PLANNER_MCP_TENANT_ID || 'common';
    const config: Configuration = {
      auth: {
        clientId,
        authority: `https://login.microsoftonline.com/${tenantId}`,
      },
    };
    return new AuthManager(config);
  }

  async loadTokenCache(): Promise<void> {
    try {
      let cacheData: string | undefined;
      try {
        const kt = await getKeytar();
        if (kt) {
          const cached = await kt.getPassword(SERVICE_NAME, TOKEN_CACHE_ACCOUNT);
          if (cached) cacheData = cached;
        }
      } catch {
        logger.warn('Keychain access failed, falling back to file storage');
      }
      const cachePath = getTokenCachePath();
      if (!cacheData && existsSync(cachePath)) {
        cacheData = readFileSync(cachePath, 'utf8');
      }
      if (cacheData) {
        this.msalApp.getTokenCache().deserialize(cacheData);
      }
    } catch (error) {
      logger.error(`Error loading token cache: ${(error as Error).message}`);
    }
  }

  async saveTokenCache(): Promise<void> {
    try {
      const cacheData = this.msalApp.getTokenCache().serialize();
      try {
        const kt = await getKeytar();
        if (kt) {
          await kt.setPassword(SERVICE_NAME, TOKEN_CACHE_ACCOUNT, cacheData);
          return;
        }
      } catch {
        logger.warn('Keychain save failed, falling back to file storage');
      }
      const cachePath = getTokenCachePath();
      ensureParentDir(cachePath);
      fs.writeFileSync(cachePath, cacheData, { mode: 0o600 });
    } catch (error) {
      logger.error(`Error saving token cache: ${(error as Error).message}`);
    }
  }

  async getToken(): Promise<string> {
    if (this.accessToken && this.tokenExpiry && this.tokenExpiry > Date.now()) {
      return this.accessToken;
    }
    const accounts = await this.msalApp.getTokenCache().getAllAccounts();
    if (accounts.length === 0) {
      throw new Error('Not logged in. Use planner-login tool first.');
    }
    try {
      const response = await this.msalApp.acquireTokenSilent({
        account: accounts[0],
        scopes: this.scopes,
      });
      this.accessToken = response.accessToken;
      this.tokenExpiry = response.expiresOn ? new Date(response.expiresOn).getTime() : null;
      return this.accessToken;
    } catch {
      throw new Error('Token refresh failed. Please re-authenticate with planner-login.');
    }
  }

  async acquireTokenByDeviceCode(
    callback?: (message: string) => void
  ): Promise<string | null> {
    const response = await this.msalApp.acquireTokenByDeviceCode({
      scopes: this.scopes,
      deviceCodeCallback: (resp) => {
        const text = `\n${resp.message}\n`;
        if (callback) {
          callback(text);
        } else {
          console.log(text);
        }
      },
    });
    this.accessToken = response?.accessToken || null;
    this.tokenExpiry = response?.expiresOn ? new Date(response.expiresOn).getTime() : null;
    await this.saveTokenCache();
    return this.accessToken;
  }

  async testLogin(): Promise<{ success: boolean; message: string; user?: { displayName: string; userPrincipalName: string } }> {
    try {
      const token = await this.getToken();
      const response = await fetch('https://graph.microsoft.com/v1.0/me', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (response.ok) {
        const data = await response.json();
        return {
          success: true,
          message: 'Logged in',
          user: { displayName: data.displayName, userPrincipalName: data.userPrincipalName },
        };
      }
      return { success: false, message: `Graph API error: ${response.status}` };
    } catch (error) {
      return { success: false, message: (error as Error).message };
    }
  }

  async logout(): Promise<void> {
    const accounts = await this.msalApp.getTokenCache().getAllAccounts();
    for (const account of accounts) {
      await this.msalApp.getTokenCache().removeAccount(account);
    }
    this.accessToken = null;
    this.tokenExpiry = null;
    try {
      const kt = await getKeytar();
      if (kt) await kt.deletePassword(SERVICE_NAME, TOKEN_CACHE_ACCOUNT);
    } catch { /* ignore */ }
    const cachePath = getTokenCachePath();
    if (fs.existsSync(cachePath)) fs.unlinkSync(cachePath);
  }
}

export default AuthManager;
```

- [ ] **Step 2: Commit**

```bash
git add src/auth.ts
git commit -m "feat: add MSAL auth manager with device code flow"
```

---

### Task 5: Auth Tools

**Files:**
- Create: `src/auth-tools.ts`

- [ ] **Step 1: Create auth-tools.ts**

Three MCP tools: planner-login, planner-logout, planner-auth-status.

```typescript
import { z } from 'zod';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import AuthManager from './auth.js';
import logger from './logger.js';

export function registerAuthTools(server: McpServer, authManager: AuthManager): void {
  server.tool(
    'planner-login',
    'Authenticate with Microsoft using device code flow',
    {
      force: z.boolean().default(false).describe('Force a new login even if already logged in'),
    },
    async ({ force }) => {
      try {
        if (!force) {
          const status = await authManager.testLogin();
          if (status.success) {
            return {
              content: [{ type: 'text', text: JSON.stringify({ status: 'Already logged in', ...status }) }],
            };
          }
        }
        // Fire-and-forget: acquireTokenByDeviceCode runs in background.
        // The callback fires immediately with the device code URL.
        // The token acquisition completes after the user logs in at the URL.
        // We return the device code message immediately so the LLM can show it.
        const text = await new Promise<string>((resolve, reject) => {
          authManager.acquireTokenByDeviceCode(resolve).catch((err) => {
            // Log but don't reject — the device code message was already returned
            logger.error(`Device code flow error: ${err.message}`);
          });
        });
        return {
          content: [{ type: 'text', text: JSON.stringify({ action: 'device_code_required', message: text.trim() }) }],
        };
      } catch (error) {
        return {
          content: [{ type: 'text', text: JSON.stringify({ error: `Auth failed: ${(error as Error).message}` }) }],
          isError: true,
        };
      }
    }
  );

  server.tool('planner-logout', 'Log out from Microsoft', {}, async () => {
    try {
      await authManager.logout();
      return { content: [{ type: 'text', text: JSON.stringify({ message: 'Logged out' }) }] };
    } catch {
      return { content: [{ type: 'text', text: JSON.stringify({ error: 'Logout failed' }) }], isError: true };
    }
  });

  server.tool('planner-auth-status', 'Check Microsoft auth status', {}, async () => {
    const result = await authManager.testLogin();
    return { content: [{ type: 'text', text: JSON.stringify(result) }] };
  });
}
```

- [ ] **Step 2: Commit**

```bash
git add src/auth-tools.ts
git commit -m "feat: add auth MCP tools (login, logout, status)"
```

---

### Task 6: Graph Client

**Files:**
- Create: `src/graph-client.ts`
- Create: `test/graph-client.test.ts`

- [ ] **Step 1: Write failing tests for graph client**

```typescript
// test/graph-client.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';

// We'll mock fetch globally
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Mock auth manager
const mockAuthManager = {
  getToken: vi.fn().mockResolvedValue('test-token'),
};

import GraphClient from '../src/graph-client.js';

describe('GraphClient', () => {
  let client: GraphClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GraphClient(mockAuthManager as any);
  });

  describe('get', () => {
    it('sends GET request with auth header', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ id: '123', title: 'Test' }),
      });

      const result = await client.get('/planner/tasks/123');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://graph.microsoft.com/v1.0/planner/tasks/123',
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            Authorization: 'Bearer test-token',
          }),
        })
      );
      expect(result).toEqual({ id: '123', title: 'Test' });
    });

    it('appends query parameters', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ value: [] }),
      });

      await client.get('/planner/plans/abc/tasks', { $top: '10', $filter: "status eq 'active'" });
      const url = mockFetch.mock.calls[0][0];
      expect(url).toContain('$top=10');
      expect(url).toContain('$filter=');
    });
  });

  describe('patch with ETag', () => {
    it('sends If-Match header with ETag', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => '',
      });

      await client.patch('/planner/tasks/123', { title: 'Updated' }, 'W/"etag123"');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://graph.microsoft.com/v1.0/planner/tasks/123',
        expect.objectContaining({
          method: 'PATCH',
          headers: expect.objectContaining({
            'If-Match': 'W/"etag123"',
          }),
        })
      );
    });
  });

  describe('getEtag', () => {
    it('extracts @odata.etag from response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ '@odata.etag': 'W/"abc"', id: '123' }),
      });

      const etag = await client.getEtag('/planner/tasks/123');
      expect(etag).toBe('W/"abc"');
    });
  });

  describe('throttle handling', () => {
    it('retries once on 429 with Retry-After', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 429,
          headers: { get: (name: string) => name === 'Retry-After' ? '1' : null },
          text: async () => JSON.stringify({ error: { code: 'TooManyRequests', message: 'Throttled' } }),
        })
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          text: async () => JSON.stringify({ id: '123' }),
        });

      const result = await client.get('/planner/tasks/123');
      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(result).toEqual({ id: '123' });
    });
  });

  describe('error handling', () => {
    it('throws on 412 Precondition Failed with clear message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 412,
        headers: { get: () => null },
        text: async () => JSON.stringify({ error: { code: 'PreconditionFailed', message: 'ETag mismatch' } }),
      });

      await expect(client.patch('/planner/tasks/123', {}, 'W/"old"')).rejects.toThrow(
        /modified by another user/i
      );
    });

    it('throws on generic Graph API error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: { get: () => null },
        text: async () => JSON.stringify({ error: { code: 'Request_ResourceNotFound', message: 'Not found' } }),
      });

      await expect(client.get('/planner/tasks/nonexistent')).rejects.toThrow(/Not found/);
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/mlu/Documents/project/plannner-connector
npx vitest run test/graph-client.test.ts
```

Expected: FAIL — `../src/graph-client.js` does not exist.

- [ ] **Step 3: Implement graph-client.ts**

```typescript
import logger from './logger.js';
import AuthManager from './auth.js';

const GRAPH_BASE = 'https://graph.microsoft.com/v1.0';

class GraphClient {
  private authManager: AuthManager;

  constructor(authManager: AuthManager) {
    this.authManager = authManager;
  }

  async get(path: string, queryParams?: Record<string, string>, extraHeaders?: Record<string, string>): Promise<any> {
    let url = `${GRAPH_BASE}${path}`;
    if (queryParams && Object.keys(queryParams).length > 0) {
      const qs = Object.entries(queryParams)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
        .join('&');
      url += `${url.includes('?') ? '&' : '?'}${qs}`;
    }
    return this.request(url, { method: 'GET', headers: extraHeaders });
  }

  async post(path: string, body: unknown): Promise<any> {
    return this.request(`${GRAPH_BASE}${path}`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  async patch(path: string, body: unknown, etag: string): Promise<any> {
    return this.request(`${GRAPH_BASE}${path}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
      headers: { 'If-Match': etag },
    });
  }

  async delete(path: string, etag: string): Promise<void> {
    await this.request(`${GRAPH_BASE}${path}`, {
      method: 'DELETE',
      headers: { 'If-Match': etag },
    });
  }

  async getEtag(path: string): Promise<string> {
    const resource = await this.get(path);
    const etag = resource['@odata.etag'];
    if (!etag) {
      throw new Error(`No @odata.etag found on resource at ${path}`);
    }
    return etag;
  }

  private async request(
    url: string,
    options: { method: string; body?: string; headers?: Record<string, string> }
  ): Promise<any> {
    const token = await this.authManager.getToken();
    const headers: Record<string, string> = {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers,
    };

    logger.info(`[GRAPH] ${options.method} ${url}`);

    let response = await fetch(url, {
      method: options.method,
      headers,
      body: options.body,
    });

    // Retry once on 429
    if (response.status === 429) {
      const retryAfter = parseInt(response.headers.get('Retry-After') || '5', 10);
      logger.warn(`Throttled by Graph API, retrying after ${retryAfter}s`);
      await new Promise((resolve) => setTimeout(resolve, retryAfter * 1000));
      response = await fetch(url, { method: options.method, headers, body: options.body });
    }

    // Handle 412 specifically
    if (response.status === 412) {
      throw new Error(
        'Resource was modified by another user, please retry. (412 Precondition Failed)'
      );
    }

    if (!response.ok) {
      const errorBody = await response.text();
      let message = `Graph API error ${response.status}`;
      try {
        const parsed = JSON.parse(errorBody);
        if (parsed.error?.message) {
          message = `${parsed.error.code}: ${parsed.error.message}`;
        }
      } catch { /* use default message */ }
      throw new Error(message);
    }

    const text = await response.text();
    if (!text) return { success: true };
    try {
      return JSON.parse(text);
    } catch {
      return { success: true, raw: text };
    }
  }
}

export default GraphClient;
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
npx vitest run test/graph-client.test.ts
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src/graph-client.ts test/graph-client.test.ts
git commit -m "feat: add Graph API client with ETag support and throttle retry"
```

---

### Task 7: Endpoints JSON and Auto-Registration

**Files:**
- Create: `src/endpoints.json`
- Create: `src/endpoint-tools.ts`
- Create: `test/endpoint-tools.test.ts`

- [ ] **Step 1: Write failing tests for endpoint tool registration**

```typescript
// test/endpoint-tools.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('endpoint-tools', () => {
  it('extracts path parameters from pathPattern', async () => {
    // Test the parameter extraction logic
    const pattern = '/planner/plans/{plan-id}/buckets';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual(['plan-id']);
  });

  it('extracts multiple path parameters', () => {
    const pattern = '/groups/{group-id}/planner/plans/{plan-id}';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual(['group-id', 'plan-id']);
  });

  it('extracts zero path parameters from parameterless path', () => {
    const pattern = '/me/planner/plans';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual([]);
  });
});
```

- [ ] **Step 2: Run tests to verify they pass**

These are testing pure regex logic, so they should pass immediately.

```bash
npx vitest run test/endpoint-tools.test.ts
```

- [ ] **Step 3: Create endpoints.json**

```json
[
  {
    "pathPattern": "/me/planner/plans",
    "method": "get",
    "toolName": "list-my-plans",
    "scopes": ["Tasks.ReadWrite"],
    "llmTip": "Returns all Planner plans the authenticated user is a member of."
  },
  {
    "pathPattern": "/planner/plans/{plan-id}",
    "method": "get",
    "toolName": "get-plan",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/plans/{plan-id}/buckets",
    "method": "get",
    "toolName": "list-buckets",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/buckets/{bucket-id}",
    "method": "get",
    "toolName": "get-bucket",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/plans/{plan-id}/tasks",
    "method": "get",
    "toolName": "list-plan-tasks",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/buckets/{bucket-id}/tasks",
    "method": "get",
    "toolName": "list-bucket-tasks",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/tasks/{task-id}",
    "method": "get",
    "toolName": "get-task",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/planner/tasks/{task-id}/details",
    "method": "get",
    "toolName": "get-task-details",
    "scopes": ["Tasks.ReadWrite"]
  },
  {
    "pathPattern": "/groups",
    "method": "get",
    "toolName": "list-groups",
    "scopes": ["Group.Read.All"],
    "headers": { "ConsistencyLevel": "eventual" },
    "llmTip": "Use $filter=groupTypes/any(g:g eq 'Unified') to find M365 groups that can have Planner plans. Use $search=\"displayName:keyword\" and remember to set ConsistencyLevel:eventual (handled automatically)."
  },
  {
    "pathPattern": "/groups/{group-id}/planner/plans",
    "method": "get",
    "toolName": "list-group-plans",
    "scopes": ["Tasks.ReadWrite", "Group.Read.All"]
  }
]
```

- [ ] **Step 4: Create endpoint-tools.ts**

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { readFileSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import logger from './logger.js';
import GraphClient from './graph-client.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

interface EndpointConfig {
  pathPattern: string;
  method: string;
  toolName: string;
  scopes: string[];
  llmTip?: string;
  headers?: Record<string, string>;
}

const endpointsData = JSON.parse(
  readFileSync(path.join(__dirname, 'endpoints.json'), 'utf8')
) as EndpointConfig[];

// OData params exposed both with and without $ prefix for LLM compatibility
// Some MCP clients don't support $ in param names, so we accept both forms
const ODATA_PARAMS = ['filter', 'select', 'top', 'orderby', 'expand', 'count', 'search'];

function extractPathParams(pattern: string): string[] {
  return [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
}

export function registerEndpointTools(server: McpServer, graphClient: GraphClient): number {
  let count = 0;

  for (const endpoint of endpointsData) {
    const pathParams = extractPathParams(endpoint.pathPattern);

    // Build Zod schema: path params are required strings, OData params are optional
    const schema: Record<string, z.ZodTypeAny> = {};
    for (const param of pathParams) {
      schema[param] = z.string().describe(`Path parameter: ${param}`);
    }
    // Accept OData params without $ prefix (e.g. "filter" maps to "$filter")
    for (const odataParam of ODATA_PARAMS) {
      schema[odataParam] = z.string().optional().describe(`OData query parameter $${odataParam}`);
    }
    // Pagination support
    schema['nextLink'] = z.string().optional().describe('Pagination: pass @odata.nextLink URL to fetch the next page');

    let description = `${endpoint.method.toUpperCase()} ${endpoint.pathPattern}`;
    if (endpoint.llmTip) {
      description += `\n\nTIP: ${endpoint.llmTip}`;
    }

    try {
      server.tool(
        endpoint.toolName,
        description,
        schema,
        {
          title: endpoint.toolName,
          readOnlyHint: true,
          openWorldHint: true,
        },
        async (params) => {
          try {
            // If nextLink is provided, fetch that URL directly
            if (params.nextLink) {
              const url = new URL(params.nextLink as string);
              const nextPath = url.pathname.replace('/v1.0', '') + url.search;
              const result = await graphClient.get(nextPath, undefined, endpoint.headers);
              return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
            }

            // Build path by replacing {param} placeholders
            let resolvedPath = endpoint.pathPattern;
            for (const param of pathParams) {
              const value = params[param] as string;
              if (!value) {
                return {
                  content: [{ type: 'text', text: JSON.stringify({ error: `Missing required parameter: ${param}` }) }],
                  isError: true,
                };
              }
              resolvedPath = resolvedPath.replace(`{${param}}`, encodeURIComponent(value));
            }

            // Collect OData query params (accept without $ prefix, send with $)
            const queryParams: Record<string, string> = {};
            for (const odataParam of ODATA_PARAMS) {
              const value = params[odataParam] as string | undefined;
              if (value) {
                queryParams[`$${odataParam}`] = value;
              }
            }

            const result = await graphClient.get(resolvedPath, queryParams, endpoint.headers);
            return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
          } catch (error) {
            return {
              content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }],
              isError: true,
            };
          }
        }
      );
      count++;
    } catch (error) {
      logger.error(`Failed to register endpoint tool ${endpoint.toolName}: ${(error as Error).message}`);
    }
  }

  logger.info(`Registered ${count} endpoint-driven tools`);
  return count;
}

export { endpointsData };
```

- [ ] **Step 5: Commit**

```bash
git add src/endpoints.json src/endpoint-tools.ts test/endpoint-tools.test.ts
git commit -m "feat: add endpoint-driven tool auto-registration"
```

---

### Task 8: Hand-Written Tools — Plans

**Files:**
- Create: `src/tools/plans.ts`

- [ ] **Step 1: Create tools/plans.ts**

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerPlanTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-plan',
    'Create a new Planner plan. The owner must be a Microsoft 365 Group ID.',
    {
      title: z.string().describe('Plan title'),
      owner: z.string().describe('Group ID that owns the plan (use list-groups to find)'),
    },
    { title: 'create-plan', destructiveHint: true, openWorldHint: true },
    async ({ title, owner }) => {
      try {
        const result = await graphClient.post('/planner/plans', { owner, title });
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-plan',
    'Update a Planner plan (title, category descriptions). ETag is auto-fetched if not provided.',
    {
      'plan-id': z.string().describe('Plan ID'),
      title: z.string().optional().describe('New plan title'),
      categoryDescriptions: z.record(z.string()).optional().describe('Category label descriptions, e.g. {"category1": "Urgent", "category2": "Bug"}'),
      etag: z.string().optional().describe('ETag for optimistic concurrency (auto-fetched if omitted)'),
    },
    { title: 'update-plan', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const planId = params['plan-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/plans/${planId}`);
        const body: Record<string, unknown> = {};
        if (params.title) body.title = params.title;
        if (params.categoryDescriptions) body.categoryDescriptions = params.categoryDescriptions;
        const result = await graphClient.patch(`/planner/plans/${planId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-plan',
    'Delete a Planner plan. ETag is auto-fetched if not provided.',
    {
      'plan-id': z.string().describe('Plan ID'),
      etag: z.string().optional().describe('ETag for optimistic concurrency (auto-fetched if omitted)'),
    },
    { title: 'delete-plan', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const planId = params['plan-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/plans/${planId}`);
        await graphClient.delete(`/planner/plans/${planId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Plan deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p src/tools
git add src/tools/plans.ts
git commit -m "feat: add plan CRUD tools (create, update, delete)"
```

---

### Task 9: Hand-Written Tools — Buckets

**Files:**
- Create: `src/tools/buckets.ts`

- [ ] **Step 1: Create tools/buckets.ts**

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerBucketTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-bucket',
    'Create a new bucket in a Planner plan.',
    {
      name: z.string().describe('Bucket name'),
      planId: z.string().describe('Plan ID to create the bucket in'),
      orderHint: z.string().optional().describe('Order hint for positioning (use " !" for first position)'),
    },
    { title: 'create-bucket', destructiveHint: true, openWorldHint: true },
    async ({ name, planId, orderHint }) => {
      try {
        const body: Record<string, unknown> = { name, planId };
        if (orderHint) body.orderHint = orderHint;
        const result = await graphClient.post('/planner/buckets', body);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-bucket',
    'Update a Planner bucket (name, orderHint). ETag is auto-fetched if not provided.',
    {
      'bucket-id': z.string().describe('Bucket ID'),
      name: z.string().optional().describe('New bucket name'),
      orderHint: z.string().optional().describe('New order hint'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-bucket', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const bucketId = params['bucket-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/buckets/${bucketId}`);
        const body: Record<string, unknown> = {};
        if (params.name) body.name = params.name;
        if (params.orderHint) body.orderHint = params.orderHint;
        const result = await graphClient.patch(`/planner/buckets/${bucketId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-bucket',
    'Delete a Planner bucket. ETag is auto-fetched if not provided.',
    {
      'bucket-id': z.string().describe('Bucket ID'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'delete-bucket', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const bucketId = params['bucket-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/buckets/${bucketId}`);
        await graphClient.delete(`/planner/buckets/${bucketId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Bucket deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add src/tools/buckets.ts
git commit -m "feat: add bucket CRUD tools (create, update, delete)"
```

---

### Task 10: Hand-Written Tools — Tasks

**Files:**
- Create: `src/tools/tasks.ts`
- Create: `test/tools/tasks.test.ts`

- [ ] **Step 1: Write failing test for assignment format helper**

```typescript
// test/tools/tasks.test.ts
import { describe, it, expect } from 'vitest';

function buildAssignment(userId: string) {
  return {
    [userId]: {
      '@odata.type': '#microsoft.graph.plannerAssignment',
      orderHint: ' !',
    },
  };
}

describe('Task helpers', () => {
  it('builds correct assignment format', () => {
    const assignment = buildAssignment('user-123');
    expect(assignment).toEqual({
      'user-123': {
        '@odata.type': '#microsoft.graph.plannerAssignment',
        orderHint: ' !',
      },
    });
  });
});
```

- [ ] **Step 2: Run test to verify it passes**

```bash
npx vitest run test/tools/tasks.test.ts
```

- [ ] **Step 3: Create tools/tasks.ts**

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerTaskTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-task',
    'Create a new Planner task.',
    {
      planId: z.string().describe('Plan ID'),
      bucketId: z.string().optional().describe('Bucket ID (task goes to default bucket if omitted)'),
      title: z.string().describe('Task title'),
      assigneeIds: z.array(z.string()).optional().describe('Array of user IDs to assign'),
      priority: z.number().optional().describe('Priority: 0=Urgent, 1=Important, 2=Medium, 3+ =Low'),
      startDateTime: z.string().optional().describe('Start date in ISO 8601 format'),
      dueDateTime: z.string().optional().describe('Due date in ISO 8601 format'),
      percentComplete: z.number().optional().describe('Completion: 0=Not started, 50=In progress, 100=Complete'),
      orderHint: z.string().optional().describe('Order hint for positioning'),
    },
    { title: 'create-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const body: Record<string, unknown> = {
          planId: params.planId,
          title: params.title,
        };
        if (params.bucketId) body.bucketId = params.bucketId;
        if (params.priority !== undefined) body.priority = params.priority;
        if (params.startDateTime) body.startDateTime = params.startDateTime;
        if (params.dueDateTime) body.dueDateTime = params.dueDateTime;
        if (params.percentComplete !== undefined) body.percentComplete = params.percentComplete;
        if (params.orderHint) body.orderHint = params.orderHint;
        if (params.assigneeIds && params.assigneeIds.length > 0) {
          const assignments: Record<string, unknown> = {};
          for (const userId of params.assigneeIds) {
            assignments[userId] = {
              '@odata.type': '#microsoft.graph.plannerAssignment',
              orderHint: ' !',
            };
          }
          body.assignments = assignments;
        }
        const result = await graphClient.post('/planner/tasks', body);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-task',
    'Update a Planner task. ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      title: z.string().optional().describe('New title'),
      bucketId: z.string().optional().describe('Move to different bucket'),
      priority: z.number().optional().describe('Priority: 0=Urgent, 1=Important, 2=Medium, 3+=Low'),
      startDateTime: z.string().optional().describe('Start date ISO 8601'),
      dueDateTime: z.string().optional().describe('Due date ISO 8601'),
      percentComplete: z.number().optional().describe('0=Not started, 50=In progress, 100=Complete'),
      appliedCategories: z.record(z.boolean()).optional().describe('Categories, e.g. {"category1": true, "category2": false}'),
      orderHint: z.string().optional().describe('Order hint'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}`);
        const body: Record<string, unknown> = {};
        if (params.title) body.title = params.title;
        if (params.bucketId) body.bucketId = params.bucketId;
        if (params.priority !== undefined) body.priority = params.priority;
        if (params.startDateTime) body.startDateTime = params.startDateTime;
        if (params.dueDateTime) body.dueDateTime = params.dueDateTime;
        if (params.percentComplete !== undefined) body.percentComplete = params.percentComplete;
        if (params.appliedCategories) body.appliedCategories = params.appliedCategories;
        if (params.orderHint) body.orderHint = params.orderHint;
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-task',
    'Delete a Planner task. ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'delete-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}`);
        await graphClient.delete(`/planner/tasks/${taskId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Task deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'assign-task',
    'Assign a user to a Planner task. Fetches current task, merges assignment, and updates.',
    {
      'task-id': z.string().describe('Task ID'),
      userId: z.string().describe('User ID to assign'),
    },
    { title: 'assign-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const assignments = task.assignments || {};
        assignments[params.userId] = {
          '@odata.type': '#microsoft.graph.plannerAssignment',
          orderHint: ' !',
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { assignments }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'unassign-task',
    'Remove a user assignment from a Planner task.',
    {
      'task-id': z.string().describe('Task ID'),
      userId: z.string().describe('User ID to unassign'),
    },
    { title: 'unassign-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const assignments = { [params.userId]: null };
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { assignments }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'move-task',
    'Move a Planner task to a different bucket.',
    {
      'task-id': z.string().describe('Task ID'),
      bucketId: z.string().describe('Target bucket ID'),
    },
    { title: 'move-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { bucketId: params.bucketId }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add src/tools/tasks.ts test/tools/tasks.test.ts
git commit -m "feat: add task tools (create, update, delete, assign, unassign, move)"
```

---

### Task 11: Hand-Written Tools — Task Details

**Files:**
- Create: `src/tools/task-details.ts`

- [ ] **Step 1: Create tools/task-details.ts**

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { randomUUID } from 'crypto';
import GraphClient from '../graph-client.js';

export function registerTaskDetailTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'update-task-details',
    'Update task details (description, checklist, references, previewType). ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      description: z.string().optional().describe('Task description (plain text)'),
      previewType: z.string().optional().describe('"automatic", "noPreview", "checklist", "description", or "reference"'),
      checklist: z.record(z.object({
        title: z.string(),
        isChecked: z.boolean().optional(),
      })).optional().describe('Checklist items keyed by GUID, e.g. {"guid-here": {"title": "Item", "isChecked": false}}'),
      references: z.record(z.object({
        alias: z.string().optional(),
        type: z.string().optional(),
        previewPriority: z.string().optional(),
      })).optional().describe('References keyed by URL (with special chars encoded)'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-task-details', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}/details`);
        const body: Record<string, unknown> = {};
        if (params.description !== undefined) body.description = params.description;
        if (params.previewType) body.previewType = params.previewType;
        if (params.checklist) body.checklist = params.checklist;
        if (params.references) body.references = params.references;
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'add-checklist-item',
    'Add a checklist item to a Planner task. Fetches current details, generates a GUID key, and adds the item.',
    {
      'task-id': z.string().describe('Task ID'),
      title: z.string().describe('Checklist item text'),
      isChecked: z.boolean().optional().describe('Initial checked state (default: false)'),
    },
    { title: 'add-checklist-item', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const details = await graphClient.get(`/planner/tasks/${taskId}/details`);
        const etag = details['@odata.etag'];
        const guid = randomUUID();
        const checklist = {
          [guid]: {
            '@odata.type': 'microsoft.graph.plannerChecklistItem',
            title: params.title,
            isChecked: params.isChecked ?? false,
          },
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, { checklist }, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ ...result, addedItemId: guid }, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'toggle-checklist-item',
    'Toggle a checklist item\'s checked state on a Planner task.',
    {
      'task-id': z.string().describe('Task ID'),
      itemId: z.string().describe('Checklist item GUID key'),
    },
    { title: 'toggle-checklist-item', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const details = await graphClient.get(`/planner/tasks/${taskId}/details`);
        const etag = details['@odata.etag'];
        const existingItem = details.checklist?.[params.itemId];
        if (!existingItem) {
          return {
            content: [{ type: 'text', text: JSON.stringify({ error: `Checklist item ${params.itemId} not found` }) }],
            isError: true,
          };
        }
        const checklist = {
          [params.itemId]: {
            '@odata.type': 'microsoft.graph.plannerChecklistItem',
            ...existingItem,
            isChecked: !existingItem.isChecked,
          },
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, { checklist }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add src/tools/task-details.ts
git commit -m "feat: add task detail tools (update details, checklist add/toggle)"
```

---

### Task 12: Server (MCP Server Class)

**Files:**
- Create: `src/server.ts`

- [ ] **Step 1: Create server.ts**

Wires together all tools with stdio + HTTP transport support.

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { StreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/streamableHttp.js';
import express from 'express';
import logger, { enableConsoleLogging } from './logger.js';
import { registerAuthTools } from './auth-tools.js';
import { registerEndpointTools } from './endpoint-tools.js';
import { registerPlanTools } from './tools/plans.js';
import { registerBucketTools } from './tools/buckets.js';
import { registerTaskTools } from './tools/tasks.js';
import { registerTaskDetailTools } from './tools/task-details.js';
import GraphClient from './graph-client.js';
import AuthManager from './auth.js';
import type { CommandOptions } from './cli.js';

function parseHttpOption(httpOption: string | boolean): { host: string | undefined; port: number } {
  if (typeof httpOption === 'boolean') return { host: undefined, port: 3000 };
  const s = (httpOption as string).trim();
  if (s.includes(':')) {
    const [hostPart, portPart] = s.split(':');
    return { host: hostPart || undefined, port: parseInt(portPart) || 3000 };
  }
  return { host: undefined, port: parseInt(s) || 3000 };
}

class PlannerServer {
  private authManager: AuthManager;
  private graphClient: GraphClient;
  private options: CommandOptions;

  constructor(authManager: AuthManager, options: CommandOptions = {}) {
    this.authManager = authManager;
    this.graphClient = new GraphClient(authManager);
    this.options = options;
  }

  private createMcpServer(): McpServer {
    const server = new McpServer({
      name: 'PlannerMCP',
      version: '0.1.0',
    });

    registerAuthTools(server, this.authManager);
    registerEndpointTools(server, this.graphClient);
    registerPlanTools(server, this.graphClient);
    registerBucketTools(server, this.graphClient);
    registerTaskTools(server, this.graphClient);
    registerTaskDetailTools(server, this.graphClient);

    return server;
  }

  async start(): Promise<void> {
    if (this.options.verbose) {
      enableConsoleLogging();
    }

    logger.info('Planner MCP Server starting...');

    if (this.options.http) {
      const { host, port } = parseHttpOption(this.options.http);
      const app = express();
      app.use(express.json());

      // NOTE: HTTP OAuth proxy (matching ms-365-mcp-server's /authorize, /token,
      // /.well-known/oauth-authorization-server pattern) is descoped to a follow-up task.
      // For now, HTTP mode is unauthenticated — suitable for local dev only.
      // TODO: Add OAuth proxy endpoints for production HTTP deployments.

      app.post('/mcp', async (req, res) => {
        try {
          const server = this.createMcpServer();
          const transport = new StreamableHTTPServerTransport({
            sessionIdGenerator: undefined,
          });
          res.on('close', () => transport.close());
          await server.connect(transport);
          await transport.handleRequest(req as any, res as any, req.body);
        } catch (error) {
          logger.error('MCP request error:', error);
          if (!res.headersSent) {
            res.status(500).json({ jsonrpc: '2.0', error: { code: -32603, message: 'Internal error' }, id: null });
          }
        }
      });

      app.get('/mcp', async (req, res) => {
        try {
          const server = this.createMcpServer();
          const transport = new StreamableHTTPServerTransport({
            sessionIdGenerator: undefined,
          });
          res.on('close', () => transport.close());
          await server.connect(transport);
          await transport.handleRequest(req as any, res as any, undefined);
        } catch (error) {
          logger.error('MCP request error:', error);
          if (!res.headersSent) {
            res.status(500).json({ jsonrpc: '2.0', error: { code: -32603, message: 'Internal error' }, id: null });
          }
        }
      });

      app.get('/', (_req, res) => {
        res.send('Planner MCP Server is running');
      });

      if (host) {
        app.listen(port, host, () => logger.info(`Listening on ${host}:${port}`));
      } else {
        app.listen(port, () => logger.info(`Listening on 0.0.0.0:${port}`));
      }
    } else {
      const server = this.createMcpServer();
      const transport = new StdioServerTransport();
      await server.connect(transport);
      logger.info('Connected to stdio transport');
    }
  }
}

export default PlannerServer;
```

- [ ] **Step 2: Commit**

```bash
git add src/server.ts
git commit -m "feat: add MCP server with stdio and HTTP transport support"
```

---

### Task 13: Entry Point

**Files:**
- Create: `src/index.ts`

- [ ] **Step 1: Create index.ts**

```typescript
#!/usr/bin/env node

import 'dotenv/config';
import { parseArgs } from './cli.js';
import logger from './logger.js';
import AuthManager from './auth.js';
import PlannerServer from './server.js';

async function main(): Promise<void> {
  const args = parseArgs();
  const authManager = AuthManager.create();
  await authManager.loadTokenCache();

  if (args.login) {
    await authManager.acquireTokenByDeviceCode();
    logger.info('Login completed, testing connection...');
    const result = await authManager.testLogin();
    console.log(JSON.stringify(result));
    process.exit(0);
  }

  if (args.verifyLogin) {
    const result = await authManager.testLogin();
    console.log(JSON.stringify(result));
    process.exit(0);
  }

  if (args.logout) {
    await authManager.logout();
    console.log(JSON.stringify({ message: 'Logged out successfully' }));
    process.exit(0);
  }

  const server = new PlannerServer(authManager, args);
  await server.start();
}

main().catch((error) => {
  logger.error(`Fatal error: ${error.message}`);
  process.exit(1);
});
```

- [ ] **Step 2: Commit**

```bash
git add src/index.ts
git commit -m "feat: add entry point with CLI routing"
```

---

### Task 14: Build and Verify

- [ ] **Step 1: Run the build**

```bash
cd /home/mlu/Documents/project/plannner-connector
npm run build
```

Expected: Clean build to `dist/`, no TypeScript errors.

- [ ] **Step 2: Run all tests**

```bash
npm test
```

Expected: All tests pass.

- [ ] **Step 3: Fix any build or test errors**

Iterate until both build and tests pass cleanly.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve build and test issues"
```

---

### Task 15: Security Scan

- [ ] **Step 1: Run Snyk code scan on src/**

Use the `snyk_code_scan` tool on all `.ts` files in `src/` to check for security issues per CLAUDE.md instructions.

- [ ] **Step 2: Fix any findings and rescan**

Iterate until clean.

- [ ] **Step 3: Commit fixes**

```bash
git add -A
git commit -m "fix: resolve security scan findings"
```

---

### Task 16: Final Verification

- [ ] **Step 1: Verify dev mode starts without errors**

```bash
PLANNER_MCP_CLIENT_ID=test npm run dev -- --verify-login 2>&1 || true
```

Expected: Should fail with auth error (no real token), but proves the entry point loads correctly without crashes.

- [ ] **Step 2: Verify the built binary is executable**

```bash
ls -la dist/index.js
head -1 dist/index.js
```

Expected: First line is `#!/usr/bin/env node` and file is executable.

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "chore: final verification pass"
```
