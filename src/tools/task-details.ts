import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { randomUUID } from 'crypto';
import GraphClient from '../graph-client.js';

export function registerTaskDetailTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'update-task-details',
    'Update task details (description, checklist, references, previewType). ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      description: z.string().optional().describe('Task description (plain text)'),
      previewType: z.string().optional().describe('"automatic", "noPreview", "checklist", "description", or "reference"'),
      checklist: z.record(z.object({
        title: z.string(),
        isChecked: z.boolean().optional(),
      })).optional().describe('Checklist items keyed by GUID'),
      references: z.record(z.object({
        alias: z.string().optional(),
        type: z.string().optional(),
        previewPriority: z.string().optional(),
      })).optional().describe('References keyed by URL (with special chars encoded)'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-task-details', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}/details`);
        const body: Record<string, unknown> = {};
        if (params.description !== undefined) body.description = params.description;
        if (params.previewType) body.previewType = params.previewType;
        if (params.checklist) body.checklist = params.checklist;
        if (params.references) body.references = params.references;
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'add-checklist-item',
    'Add a checklist item to a Planner task. Fetches current details, generates a GUID key, and adds the item.',
    {
      'task-id': z.string().describe('Task ID'),
      title: z.string().describe('Checklist item text'),
      isChecked: z.boolean().optional().describe('Initial checked state (default: false)'),
    },
    { title: 'add-checklist-item', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const details = await graphClient.get(`/planner/tasks/${taskId}/details`);
        const etag = details['@odata.etag'];
        const guid = randomUUID();
        const checklist = {
          [guid]: {
            '@odata.type': 'microsoft.graph.plannerChecklistItem',
            title: params.title,
            isChecked: params.isChecked ?? false,
          },
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, { checklist }, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ ...result, addedItemId: guid }, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'toggle-checklist-item',
    'Toggle a checklist item\'s checked state on a Planner task.',
    {
      'task-id': z.string().describe('Task ID'),
      itemId: z.string().describe('Checklist item GUID key'),
    },
    { title: 'toggle-checklist-item', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const details = await graphClient.get(`/planner/tasks/${taskId}/details`);
        const etag = details['@odata.etag'];
        const existingItem = details.checklist?.[params.itemId];
        if (!existingItem) {
          return {
            content: [{ type: 'text', text: JSON.stringify({ error: `Checklist item ${params.itemId} not found` }) }],
            isError: true,
          };
        }
        const checklist = {
          [params.itemId]: {
            '@odata.type': 'microsoft.graph.plannerChecklistItem',
            ...existingItem,
            isChecked: !existingItem.isChecked,
          },
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}/details`, { checklist }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
