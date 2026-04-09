import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplications } from '../useApplications';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import type { PlanningApplicationSummary } from '../../../domain/types';
import {
  undecidedApplication,
  approvedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import {
  cambridgeAuthority,
} from '../../../components/AuthoritySelector/__tests__/fixtures/authority.fixtures';

describe('useApplications', () => {
  it('starts with no applications and no loading', () => {
    const spy = new SpyApplicationsBrowsePort();

    const { result } = renderHook(() => useApplications(spy));

    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.selectedAuthority).toBeNull();
    expect(spy.fetchByAuthorityCalls).toHaveLength(0);
  });

  it('fetches applications when authority is selected', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByAuthorityResult = [undecidedApplication(), approvedApplication()];
    const authority = cambridgeAuthority();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectAuthority(authority);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toHaveLength(2);
    expect(result.current.selectedAuthority).toEqual(authority);
    expect(spy.fetchByAuthorityCalls).toEqual([authority.id]);
  });

  it('sets loading to true while fetching', async () => {
    let resolvePromise: (value: readonly PlanningApplicationSummary[]) => void;
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByAuthorityOverride = () =>
      new Promise((resolve) => {
        resolvePromise = resolve;
      });
    const authority = cambridgeAuthority();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectAuthority(authority);
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
    spy.fetchByAuthorityError = new Error('Network unavailable');
    const authority = cambridgeAuthority();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectAuthority(authority);
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
  });

  it('returns empty applications when authority has none', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByAuthorityResult = [];
    const authority = cambridgeAuthority();

    const { result } = renderHook(() => useApplications(spy));

    act(() => {
      result.current.selectAuthority(authority);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(result.current.selectedAuthority).toEqual(authority);
  });

  it('clears previous error on new fetch', async () => {
    const spy = new SpyApplicationsBrowsePort();
    spy.fetchByAuthorityError = new Error('First error');
    const authority = cambridgeAuthority();

    const { result } = renderHook(() => useApplications(spy));

    // First fetch fails
    act(() => {
      result.current.selectAuthority(authority);
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    // Second fetch succeeds
    spy.fetchByAuthorityError = null;
    spy.fetchByAuthorityResult = [undecidedApplication()];

    act(() => {
      result.current.selectAuthority(authority);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
      expect(result.current.applications).toHaveLength(1);
    });

    expect(result.current.error).toBeNull();
  });
});
