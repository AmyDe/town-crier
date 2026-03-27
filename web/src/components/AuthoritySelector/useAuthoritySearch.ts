import { useState, useEffect, useRef, useCallback } from 'react';
import type { AuthorityListItem } from '../../domain/types';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';

const DEBOUNCE_MS = 250;
const MIN_QUERY_LENGTH = 2;

interface AuthoritySearchState {
  query: string;
  results: readonly AuthorityListItem[];
  isSearching: boolean;
}

export function useAuthoritySearch(port: AuthoritySearchPort) {
  const [state, setState] = useState<AuthoritySearchState>({
    query: '',
    results: [],
    isSearching: false,
  });
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const setQuery = useCallback((query: string) => {
    setState((prev) => ({ ...prev, query }));
  }, []);

  const clearResults = useCallback(() => {
    setState((prev) => ({ ...prev, results: [] }));
  }, []);

  useEffect(() => {
    if (timerRef.current !== undefined) {
      clearTimeout(timerRef.current);
    }

    if (state.query.length < MIN_QUERY_LENGTH) {
      setState((prev) => ({ ...prev, results: [], isSearching: false }));
      return;
    }

    setState((prev) => ({ ...prev, isSearching: true }));

    timerRef.current = setTimeout(async () => {
      try {
        const result = await port.search(state.query);
        setState((prev) => ({
          ...prev,
          results: result.authorities,
          isSearching: false,
        }));
      } catch {
        setState((prev) => ({ ...prev, results: [], isSearching: false }));
      }
    }, DEBOUNCE_MS);

    return () => {
      if (timerRef.current !== undefined) {
        clearTimeout(timerRef.current);
      }
    };
  }, [state.query, port]);

  return {
    query: state.query,
    results: state.results,
    isSearching: state.isSearching,
    setQuery,
    clearResults,
  };
}
