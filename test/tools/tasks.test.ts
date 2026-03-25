import { describe, it, expect } from 'vitest';

function buildAssignment(userId: string) {
  return {
    [userId]: {
      '@odata.type': '#microsoft.graph.plannerAssignment',
      orderHint: ' !',
    },
  };
}

describe('Task helpers', () => {
  it('builds correct assignment format', () => {
    const assignment = buildAssignment('user-123');
    expect(assignment).toEqual({
      'user-123': {
        '@odata.type': '#microsoft.graph.plannerAssignment',
        orderHint: ' !',
      },
    });
  });
});
