import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerPlanTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-plan',
    'Create a new Planner plan. The owner must be a Microsoft 365 Group ID.',
    {
      title: z.string().describe('Plan title'),
      owner: z.string().describe('Group ID that owns the plan (use list-groups to find)'),
    },
    { title: 'create-plan', destructiveHint: true, openWorldHint: true },
    async ({ title, owner }) => {
      try {
        const result = await graphClient.post('/planner/plans', { owner, title });
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-plan',
    'Update a Planner plan (title, category descriptions). ETag is auto-fetched if not provided.',
    {
      'plan-id': z.string().describe('Plan ID'),
      title: z.string().optional().describe('New plan title'),
      categoryDescriptions: z.record(z.string()).optional().describe('Category label descriptions, e.g. {"category1": "Urgent", "category2": "Bug"}'),
      etag: z.string().optional().describe('ETag for optimistic concurrency (auto-fetched if omitted)'),
    },
    { title: 'update-plan', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const planId = params['plan-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/plans/${planId}`);
        const body: Record<string, unknown> = {};
        if (params.title) body.title = params.title;
        if (params.categoryDescriptions) body.categoryDescriptions = params.categoryDescriptions;
        const result = await graphClient.patch(`/planner/plans/${planId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-plan',
    'Delete a Planner plan. ETag is auto-fetched if not provided.',
    {
      'plan-id': z.string().describe('Plan ID'),
      etag: z.string().optional().describe('ETag for optimistic concurrency (auto-fetched if omitted)'),
    },
    { title: 'delete-plan', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const planId = params['plan-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/plans/${planId}`);
        await graphClient.delete(`/planner/plans/${planId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Plan deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
