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

const SCOPES = [
  'https://graph.microsoft.com/Tasks.ReadWrite',
  'https://graph.microsoft.com/Group.Read.All',
  'https://graph.microsoft.com/User.Read',
];

class AuthManager {
  private scopes: string[];
  private clientId: string;
  private tenantId: string;
  private accessToken: string | null = null;
  private refreshToken: string | null = null;
  private tokenExpiry: number | null = null;

  constructor(clientId: string, tenantId: string, scopes: string[] = SCOPES) {
    this.scopes = scopes;
    this.clientId = clientId;
    this.tenantId = tenantId;
  }

  /** All scopes including OIDC scopes — used consistently in authorize, token exchange, and refresh */
  private allScopes(): string {
    return [...this.scopes, 'offline_access', 'openid', 'profile'].join(' ');
  }

  static create(): AuthManager {
    const clientId = process.env.PLANNER_MCP_CLIENT_ID;
    const tenantId = process.env.PLANNER_MCP_TENANT_ID;
    if (!clientId || !tenantId) {
      throw new Error(
        'Missing required environment variables: PLANNER_MCP_CLIENT_ID and PLANNER_MCP_TENANT_ID. ' +
        'Set these to your Azure AD app registration values.'
      );
    }
    return new AuthManager(clientId, tenantId);
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
        try {
          const parsed = JSON.parse(cacheData);
          this.accessToken = parsed.accessToken || null;
          this.refreshToken = parsed.refreshToken || null;
          this.tokenExpiry = parsed.tokenExpiry || null;
        } catch {
          logger.warn('Token cache is corrupt, starting fresh');
        }
      }
    } catch (error) {
      logger.error(`Error loading token cache: ${(error as Error).message}`);
    }
  }

  async saveTokenCache(): Promise<void> {
    try {
      const cacheData = JSON.stringify({
        accessToken: this.accessToken,
        refreshToken: this.refreshToken,
        tokenExpiry: this.tokenExpiry,
      });
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
    // Return cached token if still valid (with 5 min buffer)
    if (this.accessToken && this.tokenExpiry && this.tokenExpiry > Date.now() + 5 * 60 * 1000) {
      return this.accessToken;
    }

    // Try silent refresh with refresh token
    if (this.refreshToken) {
      try {
        await this.refreshAccessToken();
        return this.accessToken!;
      } catch {
        logger.info('Token refresh failed, triggering interactive login...');
      }
    }

    // No token or refresh failed — auto-login via browser
    const token = await this.acquireTokenInteractively();
    if (!token) {
      throw new Error('Login required but the user did not complete sign-in.');
    }
    return token;
  }

  private async refreshAccessToken(): Promise<void> {
    const tokenUrl = `https://login.microsoftonline.com/${this.tenantId}/oauth2/v2.0/token`;
    const body = new URLSearchParams({
      client_id: this.clientId,
      scope: this.allScopes(),
      refresh_token: this.refreshToken!,
      grant_type: 'refresh_token',
    });

    const response = await fetch(tokenUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString(),
    });

    if (!response.ok) {
      this.refreshToken = null;
      throw new Error('Refresh token expired or revoked');
    }

    const tokens = await response.json() as {
      access_token: string;
      expires_in: number;
      refresh_token?: string;
    };

    this.accessToken = tokens.access_token;
    this.refreshToken = tokens.refresh_token || this.refreshToken;
    this.tokenExpiry = Date.now() + tokens.expires_in * 1000;
    await this.saveTokenCache();
  }

  async acquireTokenInteractively(): Promise<string | null> {
    const http = await import('http');
    const crypto = await import('crypto');

    // Generate PKCE codes
    const verifier = crypto.randomBytes(32).toString('base64url');
    const challenge = crypto.createHash('sha256').update(verifier).digest('base64url');

    // Start a one-shot loopback server to receive the auth code redirect
    const { port, code: authCodePromise } = await this.startLoopbackServer(http);
    const redirectUri = `http://127.0.0.1:${port}`;

    // Build the authorize URL entirely by hand — no MSAL involvement
    const authParams = new URLSearchParams({
      client_id: this.clientId,
      response_type: 'code',
      redirect_uri: redirectUri,
      response_mode: 'query',
      scope: this.allScopes(),
      code_challenge: challenge,
      code_challenge_method: 'S256',
    });
    const authUrl = `https://login.microsoftonline.com/${this.tenantId}/oauth2/v2.0/authorize?${authParams.toString()}`;

    // Open browser (detached, no terminal flash)
    await this.openBrowser(authUrl);

    // Wait for user to complete sign-in and the redirect
    const code = await authCodePromise;

    // Exchange auth code for tokens — POST directly to the Azure AD token
    // endpoint so we control every parameter (bypasses MSAL scope bug).
    const tokenUrl = `https://login.microsoftonline.com/${this.tenantId}/oauth2/v2.0/token`;
    const body = new URLSearchParams({
      client_id: this.clientId,
      scope: this.allScopes(),
      code,
      redirect_uri: redirectUri,
      grant_type: 'authorization_code',
      code_verifier: verifier,
    });

    const tokenResponse = await fetch(tokenUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString(),
    });

    if (!tokenResponse.ok) {
      const err = await tokenResponse.text();
      throw new Error(`Token exchange failed: ${err}`);
    }

    const tokens = await tokenResponse.json() as {
      access_token: string;
      expires_in: number;
      refresh_token?: string;
    };

    this.accessToken = tokens.access_token;
    this.refreshToken = tokens.refresh_token || null;
    this.tokenExpiry = Date.now() + tokens.expires_in * 1000;
    await this.saveTokenCache();
    return this.accessToken;
  }

  private async startLoopbackServer(http: typeof import('http')): Promise<{ port: number; code: Promise<string> }> {
    return new Promise((resolveSetup) => {
      let resolveCode: (code: string) => void;
      let rejectCode: (err: Error) => void;
      const codePromise = new Promise<string>((res, rej) => { resolveCode = res; rejectCode = rej; });

      const server = http.createServer((req, res) => {
        const url = new URL(req.url || '/', `http://127.0.0.1`);
        const code = url.searchParams.get('code');
        const error = url.searchParams.get('error');

        if (code) {
          res.writeHead(200, { 'Content-Type': 'text/html' });
          res.end('<h1>Login successful</h1><p>You can close this window and return to Claude.</p>');
          resolveCode(code);
        } else {
          res.writeHead(400, { 'Content-Type': 'text/html' });
          res.end('<h1>Login failed</h1><p>Something went wrong. Please try again.</p>');
          rejectCode(new Error(error || 'No authorization code received'));
        }
        server.close();
      });

      server.listen(0, '127.0.0.1', () => {
        const addr = server.address();
        const port = typeof addr === 'object' && addr ? addr.port : 0;
        resolveSetup({ port, code: codePromise });
      });

      // Timeout after 5 minutes
      setTimeout(() => {
        server.close();
        rejectCode(new Error('Login timed out — no response received within 5 minutes.'));
      }, 5 * 60 * 1000);
    });
  }

  private async openBrowser(url: string): Promise<void> {
    const { spawn } = await import('child_process');
    const platform = process.platform;
    let cmd: string;
    let args: string[];
    if (platform === 'darwin') {
      cmd = 'open';
      args = [url];
    } else if (platform === 'win32') {
      cmd = 'cmd';
      args = ['/c', 'start', '', url];
    } else {
      cmd = 'xdg-open';
      args = [url];
    }
    const child = spawn(cmd, args, { detached: true, stdio: 'ignore' });
    child.unref();
    child.on('error', (err) => {
      logger.info(`Could not auto-open browser: ${err.message}`);
    });
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
    this.accessToken = null;
    this.refreshToken = null;
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
