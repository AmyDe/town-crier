import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyMapPort } from './spies/spy-map-port';
import {
  aZone,
  aSecondZone,
  aBubbleCluster,
  aSinglePinCluster,
  anApplication,
} from './fixtures/map.fixtures';
import type { MapBounds } from '../../../domain/ports/map-port';
import { asWatchZoneId } from '../../../domain/types';

const BOUNDS: MapBounds = { west: -0.2, south: 51.4, east: 0.1, north: 51.6 };
const OTHER_BOUNDS: MapBounds = { west: 0.0, south: 52.0, east: 0.3, north: 52.3 };

async function loadedHook(spy: SpyMapPort) {
  const view = renderHook(() => useMapData(spy));
  await waitFor(() => {
    expect(view.result.current.isLoading).toBe(false);
  });
  return view;
}

describe('useMapData', () => {
  it('auto-selects the first zone after loading zones', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone(), aSecondZone()];

    const { result } = await loadedHook(spy);

    expect(result.current.selectedZone?.id).toBe(asWatchZoneId('zone-001'));
    expect(result.current.zones).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.fetchMyZonesCalls).toBe(1);
  });

  it('surfaces an error when the zone load fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesError = new Error('Network unavailable');

    const { result } = await loadedHook(spy);

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.selectedZone).toBeNull();
    expect(result.current.clusters).toEqual([]);
  });

  it('fetches clusters for the active zone and current viewport on a region change', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aBubbleCluster()];

    const { result } = await loadedHook(spy);

    act(() => {
      result.current.onRegionChange(BOUNDS, 13);
    });

    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(1);
    });
    const call = spy.fetchClustersCalls[0]!;
    expect(call.zoneId).toBe(asWatchZoneId('zone-001'));
    expect(call.bounds).toEqual(BOUNDS);
    expect(call.zoom).toBe(13);
    expect(call.status).toBeNull();
    await waitFor(() => {
      expect(result.current.clusters).toHaveLength(1);
    });
  });

  it('debounces rapid region changes into a single fetch with the last viewport', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];

    const { result } = await loadedHook(spy);

    act(() => {
      result.current.onRegionChange(BOUNDS, 11);
      result.current.onRegionChange(OTHER_BOUNDS, 15);
    });

    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(1);
    });
    // Give any stray debounced call time to fire — it must not.
    await new Promise((r) => setTimeout(r, 400));
    expect(spy.fetchClustersCalls).toHaveLength(1);
    expect(spy.fetchClustersCalls[0]!.bounds).toEqual(OTHER_BOUNDS);
    expect(spy.fetchClustersCalls[0]!.zoom).toBe(15);
  });

  it('queries only the active zone, never draining other zones', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone(), aSecondZone()];

    const { result } = await loadedHook(spy);

    act(() => {
      result.current.onRegionChange(BOUNDS, 12);
    });

    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(1);
    });
    expect(spy.fetchClustersCalls.every((c) => c.zoneId === asWatchZoneId('zone-001'))).toBe(true);
    expect(spy.fetchMyZonesCalls).toBe(1);
  });

  it('refetches clusters server-side with status= when the status filter changes', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];

    const { result } = await loadedHook(spy);

    act(() => {
      result.current.onRegionChange(BOUNDS, 13);
    });
    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(1);
    });

    act(() => {
      result.current.setStatusFilter('Permitted');
    });

    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(2);
    });
    const last = spy.fetchClustersCalls[1]!;
    expect(last.status).toBe('Permitted');
    expect(last.bounds).toEqual(BOUNDS);
    expect(result.current.selectedStatusFilter).toBe('Permitted');
  });

  it('clears the status filter and requeries when the active zone changes', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone(), aSecondZone()];
    spy.fetchClustersResultsByZone.set('zone-002', [aBubbleCluster()]);

    const { result } = await loadedHook(spy);

    act(() => {
      result.current.onRegionChange(BOUNDS, 13);
    });
    await waitFor(() => {
      expect(spy.fetchClustersCalls).toHaveLength(1);
    });
    act(() => {
      result.current.setStatusFilter('Permitted');
    });
    await waitFor(() => {
      expect(result.current.selectedStatusFilter).toBe('Permitted');
    });

    act(() => {
      result.current.selectZone(aSecondZone());
    });

    expect(result.current.selectedZone?.id).toBe(asWatchZoneId('zone-002'));
    expect(result.current.selectedStatusFilter).toBeNull();
    await waitFor(() => {
      expect(
        spy.fetchClustersCalls.some(
          (c) => c.zoneId === asWatchZoneId('zone-002') && c.status === null,
        ),
      ).toBe(true);
    });
  });

  it('point-reads the application for a single-member cell', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    const app = anApplication();
    spy.fetchApplicationByMemberResult = app;

    const { result } = await loadedHook(spy);

    const cluster = aSinglePinCluster();
    let resolved: Awaited<ReturnType<typeof result.current.resolveMember>> = null;
    await act(async () => {
      resolved = await result.current.resolveMember(cluster.member!);
    });

    expect(resolved).toEqual(app);
    expect(spy.fetchApplicationByMemberCalls).toEqual([cluster.member]);
  });

  it('resolveMember returns null on a point-read failure', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationByMemberError = new Error('boom');

    const { result } = await loadedHook(spy);

    const cluster = aSinglePinCluster();
    let resolved: Awaited<ReturnType<typeof result.current.resolveMember>> = anApplication();
    await act(async () => {
      resolved = await result.current.resolveMember(cluster.member!);
    });

    expect(resolved).toBeNull();
  });
});
