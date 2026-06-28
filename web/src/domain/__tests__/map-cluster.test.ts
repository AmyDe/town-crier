import { describe, it, expect } from 'vitest';
import {
  type MapCluster,
  type ClusterMember,
  clusterIsSingleMember,
  clusterMemberStatus,
} from '../types';

function aBubble(overrides?: Partial<MapCluster>): MapCluster {
  return {
    latitude: 52.2,
    longitude: 0.12,
    count: 7,
    statusCounts: { Undecided: 4, Permitted: 3 },
    member: null,
    ...overrides,
  };
}

function aSinglePin(overrides?: Partial<MapCluster>): MapCluster {
  const member: ClusterMember = { authority: '42', name: '22/1234/FUL' };
  return {
    latitude: 52.2,
    longitude: 0.12,
    count: 1,
    statusCounts: { Permitted: 1 },
    member,
    ...overrides,
  };
}

describe('clusterIsSingleMember', () => {
  it('is true when count is exactly 1', () => {
    expect(clusterIsSingleMember(aSinglePin())).toBe(true);
  });

  it('is false when count exceeds 1', () => {
    expect(clusterIsSingleMember(aBubble())).toBe(false);
  });
});

describe('clusterMemberStatus', () => {
  it('returns the lone status for a single-member cell', () => {
    expect(clusterMemberStatus(aSinglePin())).toBe('Permitted');
  });

  it('returns null for a multi-member cell', () => {
    expect(clusterMemberStatus(aBubble())).toBeNull();
  });

  it('returns null when a single-member cell has no status counts', () => {
    expect(clusterMemberStatus(aSinglePin({ statusCounts: {} }))).toBeNull();
  });
});
