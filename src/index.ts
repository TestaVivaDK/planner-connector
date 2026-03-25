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
