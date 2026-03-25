import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { readFileSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import logger from './logger.js';
import GraphClient from './graph-client.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

interface EndpointConfig {
  pathPattern: string;
  method: string;
  toolName: string;
  scopes: string[];
  llmTip?: string;
  headers?: Record<string, string>;
}

const endpointsData = JSON.parse(
  readFileSync(path.join(__dirname, 'endpoints.json'), 'utf8')
) as EndpointConfig[];

// OData params exposed without $ prefix for LLM compatibility
// Some MCP clients don't support $ in param names, so we accept unprefixed
const ODATA_PARAMS = ['filter', 'select', 'top', 'orderby', 'expand', 'count', 'search'];

function extractPathParams(pattern: string): string[] {
  return [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
}

export function registerEndpointTools(server: McpServer, graphClient: GraphClient): number {
  let count = 0;

  for (const endpoint of endpointsData) {
    const pathParams = extractPathParams(endpoint.pathPattern);

    const schema: Record<string, z.ZodTypeAny> = {};
    for (const param of pathParams) {
      schema[param] = z.string().describe(`Path parameter: ${param}`);
    }
    // Accept OData params without $ prefix (e.g. "filter" maps to "$filter")
    for (const odataParam of ODATA_PARAMS) {
      schema[odataParam] = z.string().optional().describe(`OData query parameter $${odataParam}`);
    }
    schema['nextLink'] = z.string().optional().describe('Pagination: pass @odata.nextLink URL to fetch the next page');

    let description = `${endpoint.method.toUpperCase()} ${endpoint.pathPattern}`;
    if (endpoint.llmTip) {
      description += `\n\nTIP: ${endpoint.llmTip}`;
    }

    try {
      server.tool(
        endpoint.toolName,
        description,
        schema,
        {
          title: endpoint.toolName,
          readOnlyHint: true,
          openWorldHint: true,
        },
        async (params) => {
          try {
            if (params.nextLink) {
              const url = new URL(params.nextLink as string);
              const nextPath = url.pathname.replace('/v1.0', '') + url.search;
              const result = await graphClient.get(nextPath, undefined, endpoint.headers);
              return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
            }

            let resolvedPath = endpoint.pathPattern;
            for (const param of pathParams) {
              const value = params[param] as string;
              if (!value) {
                return {
                  content: [{ type: 'text', text: JSON.stringify({ error: `Missing required parameter: ${param}` }) }],
                  isError: true,
                };
              }
              resolvedPath = resolvedPath.replace(`{${param}}`, encodeURIComponent(value));
            }

            // Collect OData query params (accept without $ prefix, send with $)
            const queryParams: Record<string, string> = {};
            for (const odataParam of ODATA_PARAMS) {
              const value = params[odataParam] as string | undefined;
              if (value) {
                queryParams[`$${odataParam}`] = value;
              }
            }

            const result = await graphClient.get(resolvedPath, queryParams, endpoint.headers);
            return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
          } catch (error) {
            return {
              content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }],
              isError: true,
            };
          }
        }
      );
      count++;
    } catch (error) {
      logger.error(`Failed to register endpoint tool ${endpoint.toolName}: ${(error as Error).message}`);
    }
  }

  logger.info(`Registered ${count} endpoint-driven tools`);
  return count;
}

export { endpointsData };
