import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useSearch } from '../useSearch';
import { SpySearchPort } from './spies/spy-search-port';
import { aSearchResult, anotherSearchResult } from './fixtures/search-result.fixtures';

const aLocation = { lat: 51.5074, lon: -0.1278 };

describe('useSearch', () => {
  it('starts with no location set and the picker closed', () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    expect(result.current.location).toBeNull();
    expect(result.current.locationLabel).toBeNull();
    expect(result.current.isLocationPickerOpen).toBe(false);
  });

  it('does not call the port while the query is empty', async () => {
    const spy = new SpySearchPort();

    const { result } = renderHook(() => useSearch(spy));

    expect(result.current.query).toBe('');
    // Give the debounce window time to elapse — it must not fire.
    await new Promise((r) => setTimeout(r, 600));
    expect(spy.searchCalls).toHaveLength(0);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.hasSearched).toBe(false);
  });

  it('never calls the port while location is null, regardless of query text', async () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.setQuery('mill road');
    });

    // Give the debounce window time to elapse — it must not fire without a location.
    await new Promise((r) => setTimeout(r, 600));
    expect(spy.searchCalls).toHaveLength(0);
    expect(result.current.hasSearched).toBe(false);
  });

  it('changeLocation opens the picker', () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.changeLocation();
    });

    expect(result.current.isLocationPickerOpen).toBe(true);
  });

  it('closeLocationPicker closes the picker without touching the confirmed location', () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
    });
    act(() => {
      result.current.changeLocation();
    });
    act(() => {
      result.current.closeLocationPicker();
    });

    expect(result.current.isLocationPickerOpen).toBe(false);
    expect(result.current.location).toEqual(aLocation);
    expect(result.current.locationLabel).toBe('SW1A 1AA');
  });

  it('confirmLocation sets the location, label, and closes the picker', () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.changeLocation();
    });
    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
    });

    expect(result.current.location).toEqual(aLocation);
    expect(result.current.locationLabel).toBe('SW1A 1AA');
    expect(result.current.isLocationPickerOpen).toBe(false);
  });

  it('confirming a location triggers the first search using the already-typed query', async () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.setQuery('mill road');
    });
    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
    });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[0]).toEqual({ query: 'mill road', authority: null, location: aLocation });
  });

  it('re-runs the search with the new location when the location is changed mid-session', async () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.setQuery('mill road');
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
    });

    await waitFor(() => {
      expect(spy.searchCalls).toHaveLength(1);
    });

    const secondLocation = { lat: 52.2, lon: 0.1 };
    act(() => {
      result.current.confirmLocation(secondLocation, 'CB1 2AD');
    });

    await waitFor(() => {
      expect(spy.searchCalls).toHaveLength(2);
    });
    expect(spy.searchCalls[1]).toEqual({
      query: 'mill road',
      authority: null,
      location: secondLocation,
    });
  });

  it('debounces rapid typing into a single search call with the final query', async () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
    });

    act(() => {
      result.current.setQuery('m');
    });
    act(() => {
      result.current.setQuery('mi');
    });
    act(() => {
      result.current.setQuery('mill road');
    });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[0]).toEqual({ query: 'mill road', authority: null, location: aLocation });
  });

  it('populates results after a successful search', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [aSearchResult(), anotherSearchResult()], refineQuery: false };
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
      result.current.setQuery('mill road');
    });

    await waitFor(
      () => {
        expect(result.current.isLoading).toBe(false);
        expect(result.current.hasSearched).toBe(true);
      },
      { timeout: 2000 },
    );
    expect(result.current.results).toEqual([aSearchResult(), anotherSearchResult()]);
    expect(result.current.error).toBeNull();
    expect(result.current.refineQuery).toBe(false);
  });

  it('flags refineQuery when the match set was truncated', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [aSearchResult()], refineQuery: true };
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
      result.current.setQuery('a common word');
    });

    await waitFor(
      () => {
        expect(result.current.refineQuery).toBe(true);
      },
      { timeout: 2000 },
    );
  });

  it('surfaces an error message when the search fails', async () => {
    const spy = new SpySearchPort();
    spy.searchError = new Error('Request failed with status 400');
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
      result.current.setQuery('zz');
    });

    await waitFor(
      () => {
        expect(result.current.isLoading).toBe(false);
        expect(result.current.hasSearched).toBe(true);
      },
      { timeout: 2000 },
    );
    expect(result.current.error).toBe('Request failed with status 400');
    expect(result.current.results).toEqual([]);
  });

  it('passes the trimmed authority filter to the port, or null when blank', async () => {
    const spy = new SpySearchPort();
    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
      result.current.setAuthority('  cambridge  ');
      result.current.setQuery('mill road');
    });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[0]).toEqual({
      query: 'mill road',
      authority: 'cambridge',
      location: aLocation,
    });

    act(() => {
      result.current.setAuthority('');
      result.current.setQuery('mill road again');
    });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(2);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[1]).toEqual({
      query: 'mill road again',
      authority: null,
      location: aLocation,
    });
  });

  it('resets to idle and ignores a stale in-flight response when the query is cleared', async () => {
    let resolveSearch: ((outcome: { results: never[]; refineQuery: boolean }) => void) | null = null;
    const spy = new SpySearchPort();
    spy.search = (query: string, authority: string | null, location) => {
      spy.searchCalls.push({ query, authority, location });
      return new Promise((resolve) => {
        resolveSearch = resolve;
      });
    };

    const { result } = renderHook(() => useSearch(spy));

    act(() => {
      result.current.confirmLocation(aLocation, 'SW1A 1AA');
      result.current.setQuery('mill road');
    });

    await waitFor(() => {
      expect(spy.searchCalls).toHaveLength(1);
    });
    await waitFor(() => {
      expect(result.current.isLoading).toBe(true);
    });

    act(() => {
      result.current.setQuery('');
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.hasSearched).toBe(false);
    expect(result.current.results).toEqual([]);

    // The stale in-flight promise resolving afterwards must not repopulate results.
    act(() => {
      resolveSearch!({ results: [], refineQuery: false });
    });
    await new Promise((r) => setTimeout(r, 50));
    expect(result.current.hasSearched).toBe(false);
    expect(result.current.results).toEqual([]);
  });
});
