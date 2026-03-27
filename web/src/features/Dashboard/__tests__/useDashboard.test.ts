import { renderHook, waitFor } from '@testing-library/react';
import { useDashboard } from '../useDashboard';
import { SpyDashboardPort } from './spies/spy-dashboard-port';
import {
  cambridgeZone,
  oxfordZone,
  recentApplication,
  anotherRecentApplication,
} from './fixtures/dashboard.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useDashboard', () => {
  it('loads watch zones on mount', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [cambridgeZone(), oxfordZone()];

    const { result } = renderHook(() => useDashboard(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.zones).toHaveLength(2);
    expect(result.current.zones[0]?.name).toBe('Home - Cambridge');
    expect(result.current.zones[1]?.name).toBe('Office - Oxford');
    expect(spy.fetchWatchZonesCalls).toBe(1);
  });

  it('fetches recent applications for each unique authority after loading zones', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [cambridgeZone(), oxfordZone()];
    spy.fetchRecentApplicationsResults.set(asAuthorityId(42), [recentApplication()]);
    spy.fetchRecentApplicationsResults.set(asAuthorityId(99), [anotherRecentApplication()]);

    const { result } = renderHook(() => useDashboard(spy));

    await waitFor(() => {
      expect(result.current.recentApplications).toHaveLength(2);
    });

    expect(spy.fetchRecentApplicationsCalls).toHaveLength(2);
    expect(spy.fetchRecentApplicationsCalls).toContain(asAuthorityId(42));
    expect(spy.fetchRecentApplicationsCalls).toContain(asAuthorityId(99));
  });

  it('deduplicates authority IDs when multiple zones share an authority', async () => {
    const spy = new SpyDashboardPort();
    const sharedAuthority = asAuthorityId(42);
    spy.fetchWatchZonesResult = [
      cambridgeZone({ authorityId: sharedAuthority }),
      oxfordZone({ authorityId: sharedAuthority }),
    ];
    spy.fetchRecentApplicationsResults.set(sharedAuthority, [recentApplication()]);

    const { result } = renderHook(() => useDashboard(spy));

    await waitFor(() => {
      expect(result.current.recentApplications).toHaveLength(1);
    });

    expect(spy.fetchRecentApplicationsCalls).toHaveLength(1);
    expect(spy.fetchRecentApplicationsCalls[0]).toBe(sharedAuthority);
  });

  it('sets error when watch zones fetch fails', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZones = async () => {
      throw new Error('Network unavailable');
    };

    const { result } = renderHook(() => useDashboard(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.zones).toHaveLength(0);
    expect(result.current.recentApplications).toHaveLength(0);
  });

  it('returns empty applications when no zones exist', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [];

    const { result } = renderHook(() => useDashboard(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.zones).toHaveLength(0);
    expect(result.current.recentApplications).toHaveLength(0);
    expect(spy.fetchRecentApplicationsCalls).toHaveLength(0);
  });
});
