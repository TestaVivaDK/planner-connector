import { describe, it, expect } from 'vitest';

describe('endpoint-tools', () => {
  it('extracts path parameters from pathPattern', async () => {
    const pattern = '/planner/plans/{plan-id}/buckets';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual(['plan-id']);
  });

  it('extracts multiple path parameters', () => {
    const pattern = '/groups/{group-id}/planner/plans/{plan-id}';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual(['group-id', 'plan-id']);
  });

  it('extracts zero path parameters from parameterless path', () => {
    const pattern = '/me/planner/plans';
    const params = [...pattern.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]);
    expect(params).toEqual([]);
  });
});
