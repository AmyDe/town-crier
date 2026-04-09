import { useState, useCallback, useRef } from 'react';
import type { AuthorityId, PlanningApplicationSummary, SearchResult } from '../../domain/types';
import type { SearchRepository } from '../../domain/ports/search-repository';
import { ApiRequestError } from '../../api/client';
import { usePaginatedFetch } from '../../hooks/usePaginatedFetch';

const PAGE_SIZE = 20;

export function useSearch(repository: SearchRepository) {
  const [proGateRequired, setProGateRequired] = useState(false);

  const queryRef = useRef('');
  const authorityRef = useRef<AuthorityId | null>(null);

  const fetcher = useCallback(
    (page: number) => {
      if (authorityRef.current === null) {
        return Promise.resolve({ applications: [], total: 0, page: 1 } as SearchResult);
      }
      return repository.search(queryRef.current, authorityRef.current, page);
    },
    [repository],
  );

  const handleError = useCallback((err: unknown): 'default' | 'handled' => {
    if (err instanceof ApiRequestError && err.status === 403) {
      setProGateRequired(true);
      return 'handled';
    }
    return 'default';
  }, []);

  const result = usePaginatedFetch<SearchResult, PlanningApplicationSummary>({
    fetcher,
    pageSize: PAGE_SIZE,
    getItems: (r) => r.applications,
    getTotal: (r) => r.total,
    getPage: (r) => r.page,
    onError: handleError,
  });

  const performSearch = useCallback((query: string, authorityId: AuthorityId) => {
    queryRef.current = query;
    authorityRef.current = authorityId;
    setProGateRequired(false);
    result.loadPage(1);
  }, [result.loadPage]);

  return {
    applications: result.items,
    page: result.page,
    totalPages: result.totalPages,
    isLoading: result.isLoading,
    error: result.error,
    proGateRequired,
    performSearch,
    goToNextPage: result.goToNextPage,
    goToPreviousPage: result.goToPreviousPage,
  };
}
