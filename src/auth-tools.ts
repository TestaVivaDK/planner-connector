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
