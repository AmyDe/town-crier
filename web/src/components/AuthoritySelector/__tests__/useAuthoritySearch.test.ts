import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { useAuthoritySearch } from '../useAuthoritySearch';
import { SpyAuthoritySearchPort } from './spies/spy-authority-search-port';
import { twoAuthorityResults } from './fixtures/authority.fixtures';

describe('useAuthoritySearch', () => {
  it('starts with empty results and no loading state', () => {
    const spy = new SpyAuthoritySearchPort();

    const { result } = renderHook(() => useAuthoritySearch(spy));

    expect(result.current.results).toEqual([]);
    expect(result.current.isSearching).toBe(false);
    expect(result.current.query).toBe('');
  });

  it('searches after setting a query with sufficient length', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();

    const { result } = renderHook(() => useAuthoritySearch(spy));

    act(() => {
      result.current.setQuery('cam');
    });

    await waitFor(() => {
      expect(result.current.results).toHaveLength(2);
    });

    expect(spy.searchCalls).toContain('cam');
  });

  it('does not search for queries shorter than 2 characters', async () => {
    const spy = new SpyAuthoritySearchPort();

    const { result } = renderHook(() => useAuthoritySearch(spy));

    act(() => {
      result.current.setQuery('c');
    });

    // Wait a tick to ensure no search fires
    await new Promise((r) => setTimeout(r, 50));

    expect(spy.searchCalls).toHaveLength(0);
    expect(result.current.results).toEqual([]);
  });

  it('clears results when query is emptied', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();

    const { result } = renderHook(() => useAuthoritySearch(spy));

    act(() => {
      result.current.setQuery('cam');
    });

    await waitFor(() => {
      expect(result.current.results).toHaveLength(2);
    });

    act(() => {
      result.current.setQuery('');
    });

    await waitFor(() => {
      expect(result.current.results).toEqual([]);
    });
  });

  it('debounces search calls', async () => {
    vi.useFakeTimers();
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();

    const { result } = renderHook(() => useAuthoritySearch(spy));

    act(() => {
      result.current.setQuery('ca');
      result.current.setQuery('cam');
      result.current.setQuery('camb');
    });

    // Before debounce fires, no search should have happened
    expect(spy.searchCalls).toHaveLength(0);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    // Only the final query should have been searched
    expect(spy.searchCalls).toEqual(['camb']);

    vi.useRealTimers();
  });
});
