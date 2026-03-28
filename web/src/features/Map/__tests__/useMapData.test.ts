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
});
