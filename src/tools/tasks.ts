import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import GraphClient from '../graph-client.js';

export function registerTaskTools(server: McpServer, graphClient: GraphClient): void {
  server.tool(
    'create-task',
    'Create a new Planner task.',
    {
      planId: z.string().describe('Plan ID'),
      bucketId: z.string().optional().describe('Bucket ID (task goes to default bucket if omitted)'),
      title: z.string().describe('Task title'),
      assigneeIds: z.array(z.string()).optional().describe('Array of user IDs to assign'),
      priority: z.number().optional().describe('Priority: 0=Urgent, 1=Important, 2=Medium, 3+=Low'),
      startDateTime: z.string().optional().describe('Start date in ISO 8601 format'),
      dueDateTime: z.string().optional().describe('Due date in ISO 8601 format'),
      percentComplete: z.number().optional().describe('Completion: 0=Not started, 50=In progress, 100=Complete'),
      orderHint: z.string().optional().describe('Order hint for positioning'),
    },
    { title: 'create-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const body: Record<string, unknown> = {
          planId: params.planId,
          title: params.title,
        };
        if (params.bucketId) body.bucketId = params.bucketId;
        if (params.priority !== undefined) body.priority = params.priority;
        if (params.startDateTime) body.startDateTime = params.startDateTime;
        if (params.dueDateTime) body.dueDateTime = params.dueDateTime;
        if (params.percentComplete !== undefined) body.percentComplete = params.percentComplete;
        if (params.orderHint) body.orderHint = params.orderHint;
        if (params.assigneeIds && params.assigneeIds.length > 0) {
          const assignments: Record<string, unknown> = {};
          for (const userId of params.assigneeIds) {
            assignments[userId] = {
              '@odata.type': '#microsoft.graph.plannerAssignment',
              orderHint: ' !',
            };
          }
          body.assignments = assignments;
        }
        const result = await graphClient.post('/planner/tasks', body);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'update-task',
    'Update a Planner task. ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      title: z.string().optional().describe('New title'),
      bucketId: z.string().optional().describe('Move to different bucket'),
      priority: z.number().optional().describe('Priority: 0=Urgent, 1=Important, 2=Medium, 3+=Low'),
      startDateTime: z.string().optional().describe('Start date ISO 8601'),
      dueDateTime: z.string().optional().describe('Due date ISO 8601'),
      percentComplete: z.number().optional().describe('0=Not started, 50=In progress, 100=Complete'),
      appliedCategories: z.record(z.boolean()).optional().describe('Categories, e.g. {"category1": true}'),
      orderHint: z.string().optional().describe('Order hint'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'update-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}`);
        const body: Record<string, unknown> = {};
        if (params.title) body.title = params.title;
        if (params.bucketId) body.bucketId = params.bucketId;
        if (params.priority !== undefined) body.priority = params.priority;
        if (params.startDateTime) body.startDateTime = params.startDateTime;
        if (params.dueDateTime) body.dueDateTime = params.dueDateTime;
        if (params.percentComplete !== undefined) body.percentComplete = params.percentComplete;
        if (params.appliedCategories) body.appliedCategories = params.appliedCategories;
        if (params.orderHint) body.orderHint = params.orderHint;
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, body, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'delete-task',
    'Delete a Planner task. ETag is auto-fetched if not provided.',
    {
      'task-id': z.string().describe('Task ID'),
      etag: z.string().optional().describe('ETag (auto-fetched if omitted)'),
    },
    { title: 'delete-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const etag = params.etag || await graphClient.getEtag(`/planner/tasks/${taskId}`);
        await graphClient.delete(`/planner/tasks/${taskId}`, etag);
        return { content: [{ type: 'text', text: JSON.stringify({ success: true, message: 'Task deleted' }) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'assign-task',
    'Assign a user to a Planner task. Fetches current task, merges assignment, and updates.',
    {
      'task-id': z.string().describe('Task ID'),
      userId: z.string().describe('User ID to assign'),
    },
    { title: 'assign-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const assignments = task.assignments || {};
        assignments[params.userId] = {
          '@odata.type': '#microsoft.graph.plannerAssignment',
          orderHint: ' !',
        };
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { assignments }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'unassign-task',
    'Remove a user assignment from a Planner task.',
    {
      'task-id': z.string().describe('Task ID'),
      userId: z.string().describe('User ID to unassign'),
    },
    { title: 'unassign-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const assignments = { [params.userId]: null };
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { assignments }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );

  server.tool(
    'move-task',
    'Move a Planner task to a different bucket.',
    {
      'task-id': z.string().describe('Task ID'),
      bucketId: z.string().describe('Target bucket ID'),
    },
    { title: 'move-task', destructiveHint: true, openWorldHint: true },
    async (params) => {
      try {
        const taskId = params['task-id'];
        const task = await graphClient.get(`/planner/tasks/${taskId}`);
        const etag = task['@odata.etag'];
        const result = await graphClient.patch(`/planner/tasks/${taskId}`, { bucketId: params.bucketId }, etag);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] };
      } catch (error) {
        return { content: [{ type: 'text', text: JSON.stringify({ error: (error as Error).message }) }], isError: true };
      }
    }
  );
}
