import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyMapPort } from './spies/spy-map-port';
import { aZone, aSecondZone, anApplication, aSecondApplication, aSavedApplication } from './fixtures/map.fixtures';
import { asWatchZoneId, asApplicationUid } from '../../../domain/types';

describe('useMapData', () => {
  it('fetches saved application UIDs alongside applications', async () => {
    const spy = new SpyMapPort();
    const zone = aZone();
    const app = anApplication();
    spy.fetchMyZonesResult = [zone];
    spy.fetchApplicationsByZoneResults.set('zone-001', [app]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
    expect(result.current.savedUids.size).toBe(1);
    expect(spy.fetchSavedApplicationsCalls).toBe(1);
  });

  it('returns empty savedUids when no applications are saved', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.savedUids.size).toBe(0);
  });

  it('fetches zones and applications per zone', async () => {
    const spy = new SpyMapPort();
    const zone1 = aZone();
    const zone2 = aSecondZone();
    spy.fetchMyZonesResult = [zone1, zone2];

    const app1 = anApplication();
    const app2 = aSecondApplication();
    spy.fetchApplicationsByZoneResults.set('zone-001', [app1]);
    spy.fetchApplicationsByZoneResults.set('zone-002', [app2]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toEqual([app1, app2]);
    expect(result.current.error).toBeNull();
    expect(spy.fetchMyZonesCalls).toBe(1);
    expect(spy.fetchApplicationsByZoneCalls).toContainEqual(asWatchZoneId('zone-001'));
    expect(spy.fetchApplicationsByZoneCalls).toContainEqual(asWatchZoneId('zone-002'));
  });

  it('sets error state when zone fetch fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesError = new Error('Network unavailable');

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });

  it('returns loading state while fetching', () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [];

    const { result } = renderHook(() => useMapData(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('deduplicates applications across overlapping zones', async () => {
    const spy = new SpyMapPort();
    const zone1 = aZone();
    const zone2 = aSecondZone();
    spy.fetchMyZonesResult = [zone1, zone2];

    const sharedApp = anApplication();
    spy.fetchApplicationsByZoneResults.set('zone-001', [sharedApp]);
    spy.fetchApplicationsByZoneResults.set('zone-002', [sharedApp]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toHaveLength(1);
    expect(spy.fetchApplicationsByZoneCalls).toHaveLength(2);
  });

  it('adds uid to savedUids on save', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.saveApplication(asApplicationUid('app-001'));
    });

    expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
    expect(spy.saveApplicationCalls).toEqual([asApplicationUid('app-001')]);
  });

  it('removes uid from savedUids on unsave', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.unsaveApplication(asApplicationUid('app-001'));
    });

    expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(false);
    expect(spy.unsaveApplicationCalls).toEqual([asApplicationUid('app-001')]);
  });

  it('reverts savedUids when save fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [];
    spy.saveApplicationError = new Error('Server error');

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.saveApplication(asApplicationUid('app-001'));
    });

    expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(false);
  });

  it('reverts savedUids when unsave fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];
    spy.unsaveApplicationError = new Error('Server error');

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.unsaveApplication(asApplicationUid('app-001'));
    });

    expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
  });

  it('shows uid as saved after save-unsave-save toggle sequence', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const uid = asApplicationUid('app-001');

    await act(async () => {
      await result.current.saveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(true);

    await act(async () => {
      await result.current.unsaveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(false);

    await act(async () => {
      await result.current.saveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(true);
  });

  it('shows uid as unsaved after unsave-save-unsave toggle sequence on initially saved item', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const uid = asApplicationUid('app-001');

    await act(async () => {
      await result.current.unsaveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(false);

    await act(async () => {
      await result.current.saveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(true);

    await act(async () => {
      await result.current.unsaveApplication(uid);
    });
    expect(result.current.savedUids.has(uid)).toBe(false);
  });

  it('handles save and unsave of different UIDs independently', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchApplicationsByZoneResults.set('zone-001', [
      anApplication(),
      aSecondApplication(),
    ]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const uidA = asApplicationUid('app-001');
    const uidB = asApplicationUid('app-002');

    // uidA starts saved, uidB starts unsaved
    expect(result.current.savedUids.has(uidA)).toBe(true);
    expect(result.current.savedUids.has(uidB)).toBe(false);

    // unsave A and save B simultaneously
    await act(async () => {
      await result.current.unsaveApplication(uidA);
    });
    await act(async () => {
      await result.current.saveApplication(uidB);
    });

    expect(result.current.savedUids.has(uidA)).toBe(false);
    expect(result.current.savedUids.has(uidB)).toBe(true);

    expect(spy.unsaveApplicationCalls).toEqual([uidA]);
    expect(spy.saveApplicationCalls).toEqual([uidB]);
  });
});
