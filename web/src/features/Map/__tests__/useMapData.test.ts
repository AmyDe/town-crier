import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMapData } from '../useMapData';
import { SpyMapPort } from './spies/spy-map-port';
import { anAuthority, aSecondAuthority, anApplication, aSecondApplication } from './fixtures/map.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useMapData', () => {
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
});
