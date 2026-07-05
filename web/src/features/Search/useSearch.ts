import { useState, useRef, useCallback, useEffect } from 'react';
import type { SearchPort } from '../../domain/ports/search-port';
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
 * ViewModel for the public `/search` page (#821 Phase 4). Debounces the query
 * box (and authority filter) before calling the anonymous `SearchPort`, and
 * guards against out-of-order responses — a slow response for a query the
 * user has since changed or cleared must never clobber newer state.
 */
export function useSearch(port: SearchPort) {
  const [query, setQueryState] = useState('');
  const [authority, setAuthority] = useState('');
  const [state, setState] = useState<SearchState>(IDLE_STATE);

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Incremented on every new search attempt (including a reset-to-idle);
  // a response is applied only if it is still the most recent attempt.
  const requestIdRef = useRef(0);

  const runSearch = useCallback(
    async (trimmedQuery: string, authorityFilter: string | null) => {
      const requestId = ++requestIdRef.current;
      setState((prev) => ({ ...prev, isLoading: true, error: null }));
      try {
        const outcome = await port.search(trimmedQuery, authorityFilter);
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

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const trimmedQuery = query.trim();
    if (trimmedQuery === '') {
      return;
    }

    const trimmedAuthority = authority.trim();
    debounceRef.current = setTimeout(() => {
      void runSearch(trimmedQuery, trimmedAuthority === '' ? null : trimmedAuthority);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, authority, runSearch]);

  return {
    query,
    setQuery,
    authority,
    setAuthority,
    ...state,
  };
}
