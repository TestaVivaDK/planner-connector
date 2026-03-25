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
