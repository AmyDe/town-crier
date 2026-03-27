import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useUserGroups } from '../useUserGroups';
import { SpyGroupsRepository } from './spies/spy-groups-repository';
import { ownerGroupSummary, memberGroupSummary } from './fixtures/group.fixtures';

describe('useUserGroups', () => {
  it('starts in loading state', () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useUserGroups(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.groups).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('populates groups on successful fetch', async () => {
    const spy = new SpyGroupsRepository();
    spy.listGroupsResult = [ownerGroupSummary(), memberGroupSummary()];

    const { result } = renderHook(() => useUserGroups(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.groups).toHaveLength(2);
    expect(result.current.groups[0]?.name).toBe('Mill Road Residents');
    expect(result.current.groups[1]?.name).toBe('Castle Hill Watch');
    expect(result.current.error).toBeNull();
    expect(spy.listGroupsCalls).toBe(1);
  });

  it('sets error on failed fetch', async () => {
    const spy = new SpyGroupsRepository();
    spy.listGroupsError = new Error('Network unavailable');

    const { result } = renderHook(() => useUserGroups(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.groups).toEqual([]);
  });

  it('exposes a refresh function that reloads groups', async () => {
    const spy = new SpyGroupsRepository();
    spy.listGroupsResult = [ownerGroupSummary()];

    const { result } = renderHook(() => useUserGroups(spy));

    await waitFor(() => {
      expect(result.current.groups).toHaveLength(1);
    });

    spy.listGroupsResult = [ownerGroupSummary(), memberGroupSummary()];

    await result.current.refresh();

    await waitFor(() => {
      expect(result.current.groups).toHaveLength(2);
    });

    expect(spy.listGroupsCalls).toBe(2);
  });
});
