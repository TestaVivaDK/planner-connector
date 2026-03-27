import { z } from 'zod';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import AuthManager from './auth.js';
import logger from './logger.js';

export function registerAuthTools(server: McpServer, authManager: AuthManager): void {
  server.tool(
    'planner-login',
    'Authenticate with Microsoft. Opens the browser, shows the device code, and waits for the user to complete sign-in.',
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

        // Start device code flow. The browser opens automatically.
        // Send the device code to the client as a log notification so the
        // user sees it while we block waiting for sign-in to complete.
        let deviceInfo: { userCode: string; verificationUri: string } | undefined;
        const token = await authManager.acquireTokenByDeviceCode((info) => {
          deviceInfo = { userCode: info.userCode, verificationUri: info.verificationUri };
          logger.info(`Device code: ${info.userCode} — browser opened to ${info.verificationUri}`);
          server.sendLoggingMessage({
            level: 'warning',
            data: `🔑 Enter this code in the browser: ${info.userCode}\n\nA browser window should have opened to ${info.verificationUri}.\nIf not, open that URL manually and enter the code above.`,
          }).catch(() => {});
        });

        if (token) {
          const status = await authManager.testLogin();
          return {
            content: [{ type: 'text', text: JSON.stringify({
              status: 'Login successful',
              ...(deviceInfo ? { userCode: deviceInfo.userCode, verificationUri: deviceInfo.verificationUri } : {}),
              ...status,
            }) }],
          };
        }

        return {
          content: [{ type: 'text', text: JSON.stringify({
            status: 'Login failed — the user may not have completed sign-in in time.',
            ...(deviceInfo ? { userCode: deviceInfo.userCode, verificationUri: deviceInfo.verificationUri } : {}),
            hint: 'Call planner-login again to get a new device code.',
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
