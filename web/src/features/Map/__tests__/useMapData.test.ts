import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { SpyMapApplicationsPort } from './spies/spy-applications-browse-port';
import { aWatchZone, aSecondWatchZone, anApplication, aSecondApplication } from './fixtures/map.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useMapData', () => {
  let zoneSpy: SpyWatchZoneRepository;
  let appsSpy: SpyMapApplicationsPort;

  beforeEach(() => {
    zoneSpy = new SpyWatchZoneRepository();
    appsSpy = new SpyMapApplicationsPort();
  });

  it('fetches watch zones and then applications per zone authority', async () => {
    const zone1 = aWatchZone();
    const zone2 = aSecondWatchZone();
    zoneSpy.listResult = [zone1, zone2];

    const app1 = anApplication();
    const app2 = aSecondApplication();
    appsSpy.fetchByAuthorityResults.set(1, [app1, app2]);
    appsSpy.fetchByAuthorityResults.set(2, []);

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(zoneSpy.listCalls).toBe(1);
    expect(appsSpy.fetchByAuthorityCalls).toContainEqual(asAuthorityId(1));
    expect(appsSpy.fetchByAuthorityCalls).toContainEqual(asAuthorityId(2));
    expect(result.current.applications).toHaveLength(2);
    expect(result.current.error).toBeNull();
  });

  it('centers on the first watch zone', async () => {
    const zone = aWatchZone({ latitude: 52.2053, longitude: 0.1218 });
    zoneSpy.listResult = [zone];
    appsSpy.fetchByAuthorityResults.set(1, []);

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.center).toEqual({ lat: 52.2053, lng: 0.1218 });
  });

  it('defaults center to London when no zones exist', async () => {
    zoneSpy.listResult = [];

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.center).toEqual({ lat: 51.505, lng: -0.09 });
    expect(result.current.applications).toEqual([]);
  });

  it('deduplicates applications across zones with the same authority', async () => {
    const zone1 = aWatchZone({ authorityId: asAuthorityId(1) });
    const zone2 = aSecondWatchZone({ authorityId: asAuthorityId(1) });
    zoneSpy.listResult = [zone1, zone2];

    const app = anApplication();
    appsSpy.fetchByAuthorityResults.set(1, [app]);

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Should only fetch once for the same authority
    const auth1Calls = appsSpy.fetchByAuthorityCalls.filter(
      (id) => (id as number) === 1,
    );
    expect(auth1Calls).toHaveLength(1);
    expect(result.current.applications).toHaveLength(1);
  });

  it('sets error when watch zone fetch fails', async () => {
    zoneSpy.listError = new Error('Network unavailable');

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });

  it('sets error when applications fetch fails', async () => {
    zoneSpy.listResult = [aWatchZone()];
    appsSpy.fetchByAuthorityError = new Error('Applications unavailable');

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Applications unavailable');
  });

  it('filters out applications without coordinates', async () => {
    zoneSpy.listResult = [aWatchZone()];

    const withCoords = anApplication({ latitude: 52.2053, longitude: 0.1218 });
    const withoutCoords = aSecondApplication({ latitude: null, longitude: null });
    appsSpy.fetchByAuthorityResults.set(1, [withCoords, withoutCoords]);

    const { result } = renderHook(() => useMapData(zoneSpy, appsSpy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toHaveLength(1);
    expect(result.current.applications[0]?.uid).toBe(withCoords.uid);
  });
});
