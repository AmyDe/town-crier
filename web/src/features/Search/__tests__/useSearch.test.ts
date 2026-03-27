import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useSearch } from '../useSearch';
import { SpySearchRepository } from './spies/spy-search-repository';
import { asAuthorityId } from '../../../domain/types';
import { ApiRequestError } from '../../../api/client';
import { anApplication, aSecondApplication, searchResultPage } from './fixtures/search.fixtures';

const AUTHORITY_ID = asAuthorityId(101);

describe('useSearch', () => {
  it('starts with empty state and no loading', () => {
    const spy = new SpySearchRepository();

    const { result } = renderHook(() => useSearch(spy));

    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.proGateRequired).toBe(false);
    expect(result.current.page).toBe(1);
    expect(result.current.totalPages).toBe(0);
  });

  it('populates results after performing a search', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage();

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.searchCalls).toEqual([{ query: 'extension', authorityId: AUTHORITY_ID, page: 1 }]);
  });

  it('sets proGateRequired when search returns 403', async () => {
    const spy = new SpySearchRepository();
    spy.searchError = new ApiRequestError(403, 'Pro tier required');

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.proGateRequired).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.applications).toEqual([]);
  });

  it('sets error on non-403 failure', async () => {
    const spy = new SpySearchRepository();
    spy.searchError = new Error('Network unavailable');

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.proGateRequired).toBe(false);
    expect(result.current.applications).toEqual([]);
  });

  it('navigates to the next page', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage([anApplication()], 40, 1);

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    spy.searchResult = searchResultPage([aSecondApplication()], 40, 2);

    act(() => {
      result.current.goToNextPage();
    });

    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    expect(result.current.applications).toHaveLength(1);
    expect(spy.searchCalls[1]).toEqual({ query: 'extension', authorityId: AUTHORITY_ID, page: 2 });
  });

  it('navigates to the previous page', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage([anApplication()], 40, 1);

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Go to page 2
    spy.searchResult = searchResultPage([aSecondApplication()], 40, 2);
    act(() => {
      result.current.goToNextPage();
    });
    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    // Go back to page 1
    spy.searchResult = searchResultPage([anApplication()], 40, 1);
    act(() => {
      result.current.goToPreviousPage();
    });
    await waitFor(() => {
      expect(result.current.page).toBe(1);
    });

    expect(spy.searchCalls[2]).toEqual({ query: 'extension', authorityId: AUTHORITY_ID, page: 1 });
  });

  it('calculates totalPages correctly', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage([anApplication()], 45, 1);

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // 45 items / 20 per page = 3 pages (ceiling)
    expect(result.current.totalPages).toBe(3);
  });

  it('resets to page 1 on new search', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage([anApplication()], 40, 1);

    const { result } = renderHook(() => useSearch(spy));

    // First search
    act(() => {
      result.current.performSearch('extension', AUTHORITY_ID);
    });
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Navigate to page 2
    spy.searchResult = searchResultPage([aSecondApplication()], 40, 2);
    act(() => {
      result.current.goToNextPage();
    });
    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    // New search should reset to page 1
    spy.searchResult = searchResultPage([anApplication()], 10, 1);
    act(() => {
      result.current.performSearch('new query', AUTHORITY_ID);
    });
    await waitFor(() => {
      expect(result.current.page).toBe(1);
    });

    expect(spy.searchCalls[2]).toEqual({ query: 'new query', authorityId: AUTHORITY_ID, page: 1 });
  });

  it('returns empty state when search has no results', async () => {
    const spy = new SpySearchRepository();
    spy.searchResult = searchResultPage([], 0, 1);

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.performSearch('nonexistent', AUTHORITY_ID);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.applications).toEqual([]);
    expect(result.current.totalPages).toBe(0);
  });
});
