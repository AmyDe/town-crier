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
 * ViewModel for the public `/search` page (#821 Phase 4; mandatory location
 * gate GH#863 / tc-rrv7i). Debounces the query box (and authority filter)
 * before calling the anonymous `SearchPort`, and guards against out-of-order
 * responses — a slow response for a query the user has since changed or
 * cleared must never clobber newer state.
 *
 * A location must be resolved (postcode, "use my location", or a map tap —
 * `confirmLocation`) before the debounced effect is ever allowed to fire,
 * regardless of query text: the API 400s without `lat`/`lon`, and results are
 * ranked nearest-first from this point. `changeLocation` opens (or reopens)
 * the inline picker without clearing the previously confirmed location, so
 * `SearchPage` can prefill it for a "Change" flow.
 */
export function useSearch(port: SearchPort) {
  const [query, setQueryState] = useState('');
  const [authority, setAuthority] = useState('');
  const [state, setState] = useState<SearchState>(IDLE_STATE);
  const [location, setLocation] = useState<SearchLocation | null>(null);
  const [locationLabel, setLocationLabel] = useState<string | null>(null);
  const [isLocationPickerOpen, setIsLocationPickerOpen] = useState(false);

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Incremented on every new search attempt (including a reset-to-idle);
  // a response is applied only if it is still the most recent attempt.
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

  // setQuery resets to idle immediately (synchronously, as part of handling the
  // user's own input event) whenever the box becomes blank — never via an
  // effect, so clearing the box doesn't wait on the debounce window and never
  // triggers a setState-in-effect cascading render.
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

  const confirmLocation = useCallback((newLocation: SearchLocation, label: string) => {
    setLocation(newLocation);
    setLocationLabel(label);
    setIsLocationPickerOpen(false);
  }, []);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const trimmedQuery = query.trim();
    if (trimmedQuery === '' || location === null) {
      return;
    }

    const trimmedAuthority = authority.trim();
    debounceRef.current = setTimeout(() => {
      void runSearch(trimmedQuery, trimmedAuthority === '' ? null : trimmedAuthority, location);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
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
    confirmLocation,
    ...state,
  };
}
