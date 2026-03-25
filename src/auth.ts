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
