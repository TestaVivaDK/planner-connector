import { describe, it, expect, vi, beforeEach } from 'vitest';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const mockAuthManager = {
  getToken: vi.fn().mockResolvedValue('test-token'),
};

import GraphClient from '../src/graph-client.js';

describe('GraphClient', () => {
  let client: GraphClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GraphClient(mockAuthManager as any);
  });

  describe('get', () => {
    it('sends GET request with auth header', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ id: '123', title: 'Test' }),
      });

      const result = await client.get('/planner/tasks/123');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://graph.microsoft.com/v1.0/planner/tasks/123',
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            Authorization: 'Bearer test-token',
          }),
        })
      );
      expect(result).toEqual({ id: '123', title: 'Test' });
    });

    it('appends query parameters', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ value: [] }),
      });

      await client.get('/planner/plans/abc/tasks', { $top: '10', $filter: "status eq 'active'" });
      const url = mockFetch.mock.calls[0][0];
      expect(url).toContain('$top=10');
      expect(url).toContain('$filter=');
    });
  });

  describe('patch with ETag', () => {
    it('sends If-Match header with ETag', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => '',
      });

      await client.patch('/planner/tasks/123', { title: 'Updated' }, 'W/"etag123"');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://graph.microsoft.com/v1.0/planner/tasks/123',
        expect.objectContaining({
          method: 'PATCH',
          headers: expect.objectContaining({
            'If-Match': 'W/"etag123"',
          }),
        })
      );
    });
  });

  describe('getEtag', () => {
    it('extracts @odata.etag from response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ '@odata.etag': 'W/"abc"', id: '123' }),
      });

      const etag = await client.getEtag('/planner/tasks/123');
      expect(etag).toBe('W/"abc"');
    });
  });

  describe('throttle handling', () => {
    it('retries once on 429 with Retry-After', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 429,
          headers: { get: (name: string) => name === 'Retry-After' ? '1' : null },
          text: async () => JSON.stringify({ error: { code: 'TooManyRequests', message: 'Throttled' } }),
        })
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          text: async () => JSON.stringify({ id: '123' }),
        });

      const result = await client.get('/planner/tasks/123');
      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(result).toEqual({ id: '123' });
    });
  });

  describe('error handling', () => {
    it('throws on 412 Precondition Failed with clear message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 412,
        headers: { get: () => null },
        text: async () => JSON.stringify({ error: { code: 'PreconditionFailed', message: 'ETag mismatch' } }),
      });

      await expect(client.patch('/planner/tasks/123', {}, 'W/"old"')).rejects.toThrow(
        /modified by another user/i
      );
    });

    it('throws on generic Graph API error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: { get: () => null },
        text: async () => JSON.stringify({ error: { code: 'Request_ResourceNotFound', message: 'Not found' } }),
      });

      await expect(client.get('/planner/tasks/nonexistent')).rejects.toThrow(/Not found/);
    });
  });
});
