import { z } from 'zod';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import AuthManager from './auth.js';
import logger from './logger.js';

export function registerAuthTools(server: McpServer, authManager: AuthManager): void {
  server.tool(
    'planner-login',
    'Authenticate with Microsoft. Opens the browser automatically — the user signs in and the tool waits until authentication completes.',
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

        // acquireTokenByDeviceCode auto-opens the browser.
        // This call blocks until the user completes login in the browser.
        let deviceInfo: { userCode: string; verificationUri: string } | undefined;
        const token = await authManager.acquireTokenByDeviceCode((info) => {
          deviceInfo = { userCode: info.userCode, verificationUri: info.verificationUri };
          logger.info(`Device code: ${info.userCode} — browser opened to ${info.verificationUri}`);
        });

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
            status: 'Login failed',
            message: 'No token received. The user may not have completed the browser sign-in.',
            ...(deviceInfo ? { userCode: deviceInfo.userCode, verificationUri: deviceInfo.verificationUri } : {}),
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
