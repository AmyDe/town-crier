import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplications } from '../useApplications';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import type { PlanningApplicationSummary } from '../../../domain/types';
import {
  undecidedApplication,
  approvedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import { cambridgeZone } from './fixtures/zone.fixtures';

describe('useApplications', () => {
  it('starts with no applications and no loading', () => {
    const spy = new SpyApplicationsBrowsePort();

    const { result } = renderHook(() => useApplications(spy));

    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.selectedZone).toBeNull();
    expect(spy.fetchByZoneCalls).toHaveLength(0);
  });

  it('fetches applications when zone is selected', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByZoneResult = [undecidedApplication(), approvedApplication()];
    const zone = cambridgeZone();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toHaveLength(2);
    expect(result.current.selectedZone).toEqual(zone);
    expect(spy.fetchByZoneCalls).toEqual([zone.id]);
  });

  it('sets loading to true while fetching', async () => {
    let resolvePromise: (value: readonly PlanningApplicationSummary[]) => void;
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByZoneOverride = () =>
      new Promise((resolve) => {
        resolvePromise = resolve;
      });
    const zone = cambridgeZone();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(true);
    });

    await act(async () => {
      resolvePromise!([undecidedApplication()]);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.applications).toHaveLength(1);
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByZoneError = new Error('Network unavailable');
    const zone = cambridgeZone();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
  });

  it('returns empty applications when zone has none', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByZoneResult = [];
    const zone = cambridgeZone();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(result.current.selectedZone).toEqual(zone);
  });

  it('clears previous error on new fetch', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByZoneError = new Error('First error');
    const zone = cambridgeZone();

    const { result } = renderHook(() => useApplications(spy));

    // First fetch fails
    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    // Second fetch succeeds
    spy.fetchByZoneError = null;
    spy.fetchByZoneResult = [undecidedApplication()];

    act(() => {
      result.current.selectZone(zone);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
      expect(result.current.applications).toHaveLength(1);
    });

    expect(result.current.error).toBeNull();
  });
});
