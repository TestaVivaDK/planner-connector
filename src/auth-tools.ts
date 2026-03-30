import { z } from 'zod';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import AuthManager from './auth.js';

export function registerAuthTools(server: McpServer, authManager: AuthManager): void {
  server.tool(
    'planner-login',
    'Authenticate with Microsoft. Opens the browser for sign-in and waits for completion.',
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

        // Opens the browser for Microsoft sign-in. MSAL handles the
        // localhost redirect automatically — no device codes needed.
        const token = await authManager.acquireTokenInteractively();

        if (token) {
          const status = await authManager.testLogin();
          return {
            content: [{ type: 'text', text: JSON.stringify({
              status: 'Login successful',
              ...status,
            }) }],
          };
        }

        return {
          content: [{ type: 'text', text: JSON.stringify({
            status: 'Login failed — the user may not have completed sign-in.',
            hint: 'Call planner-login again to retry.',
          }) }],
          isError: true,
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
