import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyMapPort } from './spies/spy-map-port';
import { anAuthority, aSecondAuthority, anApplication, aSecondApplication, aSavedApplication } from './fixtures/map.fixtures';
import { asAuthorityId, asApplicationUid } from '../../../domain/types';

describe('useMapData', () => {
  it('fetches saved application UIDs alongside applications', async () => {
    const spy = new SpyMapPort();
    const auth = anAuthority();
    const app = anApplication();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(1, [app]);
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
    spy.fetchMyAuthoritiesResult = [anAuthority()];
    spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.savedUids.size).toBe(0);
  });

  it('fetches authorities and applications per authority', async () => {
    const spy = new SpyMapPort();
    const auth1 = anAuthority();
    const auth2 = aSecondAuthority();
    spy.fetchMyAuthoritiesResult = [auth1, auth2];

    const app1 = anApplication();
    const app2 = aSecondApplication();
    spy.fetchApplicationsByAuthorityResults.set(1, [app1]);
    spy.fetchApplicationsByAuthorityResults.set(2, [app2]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toEqual([app1, app2]);
    expect(result.current.error).toBeNull();
    expect(spy.fetchMyAuthoritiesCalls).toBe(1);
    expect(spy.fetchApplicationsByAuthorityCalls).toContainEqual(asAuthorityId(1));
    expect(spy.fetchApplicationsByAuthorityCalls).toContainEqual(asAuthorityId(2));
  });

  it('sets error state when authority fetch fails', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyAuthoritiesError = new Error('Network unavailable');

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });

  it('returns loading state while fetching', () => {
    const spy = new SpyMapPort();
    spy.fetchMyAuthoritiesResult = [];

    const { result } = renderHook(() => useMapData(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('deduplicates authority IDs', async () => {
    const spy = new SpyMapPort();
    const auth1 = anAuthority({ id: asAuthorityId(1) });
    const auth2 = aSecondAuthority({ id: asAuthorityId(1) });
    spy.fetchMyAuthoritiesResult = [auth1, auth2];

    const app1 = anApplication();
    spy.fetchApplicationsByAuthorityResults.set(1, [app1]);

    const { result } = renderHook(() => useMapData(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(spy.fetchApplicationsByAuthorityCalls).toHaveLength(1);
    expect(spy.fetchApplicationsByAuthorityCalls[0]).toEqual(asAuthorityId(1));
  });

  it('adds uid to savedUids on save', async () => {
    const spy = new SpyMapPort();
    spy.fetchMyAuthoritiesResult = [anAuthority()];
    spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
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
    spy.fetchMyAuthoritiesResult = [anAuthority()];
    spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
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
    spy.fetchMyAuthoritiesResult = [anAuthority()];
    spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
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
    spy.fetchMyAuthoritiesResult = [anAuthority()];
    spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
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
});
