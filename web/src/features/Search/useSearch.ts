import { useState, useRef, useCallback, useEffect } from 'react';
import type { SearchLocation, SearchPort } from '../../domain/ports/search-port';
import type { SearchResult } from '../../domain/types';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

/** Debounce window between the last keystroke and firing the search call. */
const SEARCH_DEBOUNCE_MS = 400;

interface SearchState {
  readonly results: readonly SearchResult[];
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly refineQuery: boolean;
  /** True once a search has actually run — distinguishes "no query yet" from "zero results". */
  readonly hasSearched: boolean;
}

const IDLE_STATE: SearchState = {
  results: [],
  isLoading: false,
  error: null,
  refineQuery: false,
  hasSearched: false,
};

/**
 * ViewModel for the public `/search` page. A location must be confirmed before
 * any search fires, because the API 400s without `lat`/`lon`; a stale response
 * never overwrites newer state.
 */
export function useSearch(port: SearchPort) {
  const [query, setQueryState] = useState('');
  const [authority, setAuthority] = useState('');
  const [state, setState] = useState<SearchState>(IDLE_STATE);
  const [location, setLocation] = useState<SearchLocation | null>(null);
  const [locationLabel, setLocationLabel] = useState<string | null>(null);
  const [isLocationPickerOpen, setIsLocationPickerOpen] = useState(false);

  const requestIdRef = useRef(0);

  const runSearch = useCallback(
    async (trimmedQuery: string, authorityFilter: string | null, searchLocation: SearchLocation) => {
      const requestId = ++requestIdRef.current;
      setState((prev) => ({ ...prev, isLoading: true, error: null }));
      try {
        const outcome = await port.search(trimmedQuery, authorityFilter, searchLocation);
        if (requestIdRef.current !== requestId) return;
        setState({
          results: outcome.results,
          isLoading: false,
          error: null,
          refineQuery: outcome.refineQuery,
          hasSearched: true,
        });
      } catch (err: unknown) {
        if (requestIdRef.current !== requestId) return;
        setState({
          results: [],
          isLoading: false,
          error: extractErrorMessage(err, 'Search failed. Try a different search.'),
          refineQuery: false,
          hasSearched: true,
        });
      }
    },
    [port],
  );

  // Resets to idle synchronously rather than via an effect, so clearing the box
  // skips the debounce window and avoids a setState-in-effect cascading render.
  const setQuery = useCallback((value: string) => {
    setQueryState(value);
    if (value.trim() === '') {
      requestIdRef.current += 1; // invalidate any in-flight response
      setState(IDLE_STATE);
    }
  }, []);

  const changeLocation = useCallback(() => {
    setIsLocationPickerOpen(true);
  }, []);

  // Only exposed once a location exists; the first-time gate can't be dismissed.
  const closeLocationPicker = useCallback(() => {
    setIsLocationPickerOpen(false);
  }, []);

  const confirmLocation = useCallback((newLocation: SearchLocation, label: string) => {
    setLocation(newLocation);
    setLocationLabel(label);
    setIsLocationPickerOpen(false);
  }, []);

  useEffect(() => {
    const trimmedQuery = query.trim();
    if (trimmedQuery === '' || location === null) {
      return;
    }

    const trimmedAuthority = authority.trim();
    const timer = setTimeout(() => {
      void runSearch(trimmedQuery, trimmedAuthority === '' ? null : trimmedAuthority, location);
    }, SEARCH_DEBOUNCE_MS);

    return () => clearTimeout(timer);
  }, [query, authority, location, runSearch]);

  return {
    query,
    setQuery,
    authority,
    setAuthority,
    location,
    locationLabel,
    isLocationPickerOpen,
    changeLocation,
    closeLocationPicker,
    confirmLocation,
    ...state,
  };
}
