import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerBucketTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-bucket',
    'Create a new bucket in a Planner plan.',
    {
      name: z.string().describe('Bucket name'),
      planId: z.string().describe('Plan ID to create the bucket in'),
      orderHint: z.string().optional().describe('Order hint for positioning (use " !" for first position)'),
    },
    { title: 'create-bucket', destructiveHint: true, openWorldHint: true },
    async ({ name, planId, orderHint }) => {
      try {
        const body: Record<string, unknown> = { name, planId };
        if (orderHint) body.orderHint = orderHint;
        const result = await graphClient.post('/planner/buckets', body);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-bucket',
    'Update a Planner bucket (name, orderHint). ETag is auto-fetched if not provided.',
    {
      'bucket-id': z.string().describe('Bucket ID'),
      name: z.string().optional().describe('New bucket name'),
      orderHint: z.string().optional().describe('New order hint'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-bucket', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const bucketId = params['bucket-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/buckets/${bucketId}`);
        const body: Record<string, unknown> = {};
        if (params.name) body.name = params.name;
        if (params.orderHint) body.orderHint = params.orderHint;
        const result = await graphClient.patch(`/planner/buckets/${bucketId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-bucket',
    'Delete a Planner bucket. ETag is auto-fetched if not provided.',
    {
      'bucket-id': z.string().describe('Bucket ID'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'delete-bucket', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const bucketId = params['bucket-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/buckets/${bucketId}`);
        await graphClient.delete(`/planner/buckets/${bucketId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Bucket deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
