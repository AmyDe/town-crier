import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyMapPort } from './spies/spy-map-port';
import { aWatchZone, aSecondWatchZone, anApplication, aSecondApplication } from './fixtures/map.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useMapData', () => {
  it('fetches watch zones and applications per zone authority', async () => {
    const spy = new SpyMapPort();
    const zone1 = aWatchZone();
    const zone2 = aSecondWatchZone();
    spy.fetchWatchZonesResult = [zone1, zone2];

    const app1 = anApplication();
    const app2 = aSecondApplication();
    spy.fetchApplicationsByAuthorityResults.set(1, [app1]);
    spy.fetchApplicationsByAuthorityResults.set(2, [app2]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.zones).toEqual([zone1, zone2]);
    expect(result.current.applications).toEqual([app1, app2]);
    expect(result.current.error).toBeNull();
    expect(spy.fetchWatchZonesCalls).toBe(1);
    expect(spy.fetchApplicationsByAuthorityCalls).toContainEqual(asAuthorityId(1));
    expect(spy.fetchApplicationsByAuthorityCalls).toContainEqual(asAuthorityId(2));
  });

  it('sets error state when watch zone fetch fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchWatchZonesError = new Error('Network unavailable');

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.zones).toEqual([]);
    expect(result.current.applications).toEqual([]);
  });

  it('returns loading state while fetching', () => {
    const spy = new SpyMapPort();
    // Don't resolve the promise — keep it pending
    spy.fetchWatchZonesResult = [];

    const { result } = renderHook(() => useMapData(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.zones).toEqual([]);
    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('deduplicates authority IDs across zones', async () => {
    const spy = new SpyMapPort();
    const zone1 = aWatchZone({ authorityId: asAuthorityId(1) });
    const zone2 = aSecondWatchZone({ authorityId: asAuthorityId(1) });
    spy.fetchWatchZonesResult = [zone1, zone2];

    const app1 = anApplication();
    spy.fetchApplicationsByAuthorityResults.set(1, [app1]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Should only call fetchApplicationsByAuthority once for the shared authority
    expect(spy.fetchApplicationsByAuthorityCalls).toHaveLength(1);
    expect(spy.fetchApplicationsByAuthorityCalls[0]).toEqual(asAuthorityId(1));
  });
});
