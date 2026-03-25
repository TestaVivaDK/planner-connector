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
        .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
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
