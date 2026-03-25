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
      app.disable('x-powered-by');
      app.use(express.json());

      // NOTE: HTTP OAuth proxy is descoped to a follow-up task.
      // For now, HTTP mode is unauthenticated — suitable for local dev only.

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
