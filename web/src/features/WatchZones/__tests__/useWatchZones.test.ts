import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useWatchZones } from '../useWatchZones';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, aSecondWatchZone } from './fixtures/watch-zone.fixtures';

describe('useWatchZones', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('loads zones on mount', async () => {
    spy.listResult = [aWatchZone(), aSecondWatchZone()];

    const { result } = renderHook(() => useWatchZones(spy));

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.zones).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.listCalls).toBe(1);
  });

  it('sets error when fetch fails', async () => {
    spy.listError = new Error('Network unavailable');

    const { result } = renderHook(() => useWatchZones(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.zones).toEqual([]);
  });

  it('deletes a zone and refreshes the list', async () => {
    spy.listResult = [aWatchZone(), aSecondWatchZone()];

    const { result } = renderHook(() => useWatchZones(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // After delete, list returns only one zone
    spy.listResult = [aSecondWatchZone()];

    await act(async () => {
      await result.current.deleteZone('zone-1');
    });

    expect(spy.deleteCalls).toEqual(['zone-1']);
    expect(result.current.zones).toHaveLength(1);
    expect(spy.listCalls).toBe(2);
  });

  it('sets error when delete fails', async () => {
    spy.listResult = [aWatchZone()];

    const { result } = renderHook(() => useWatchZones(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    spy.deleteError = new Error('Delete failed');

    await act(async () => {
      await result.current.deleteZone('zone-1');
    });

    expect(result.current.error).toBe('Delete failed');
    // Zones should remain unchanged
    expect(result.current.zones).toHaveLength(1);
  });

  it('returns empty state when no zones exist', async () => {
    spy.listResult = [];

    const { result } = renderHook(() => useWatchZones(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.zones).toEqual([]);
  });
});
