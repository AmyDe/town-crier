import { useState, useCallback, useRef } from 'react';
import type { AuthorityId, PlanningApplicationSummary } from '../../domain/types';
import type { SearchRepository } from '../../domain/ports/search-repository';
import { ApiRequestError } from '../../api/client';

const PAGE_SIZE = 20;

interface SearchState {
  applications: readonly PlanningApplicationSummary[];
  total: number;
  page: number;
  isLoading: boolean;
  error: string | null;
  proGateRequired: boolean;
}

export function useSearch(repository: SearchRepository) {
  const [state, setState] = useState<SearchState>({
    applications: [],
    total: 0,
    page: 1,
    isLoading: false,
    error: null,
    proGateRequired: false,
  });

  const queryRef = useRef('');
  const authorityRef = useRef<AuthorityId | null>(null);

  const loadPage = useCallback(async (query: string, authorityId: AuthorityId, page: number) => {
    setState(prev => ({ ...prev, isLoading: true, error: null, proGateRequired: false }));
    try {
      const result = await repository.search(query, authorityId, page);
      setState({
        applications: result.applications,
        total: result.total,
        page: result.page,
        isLoading: false,
        error: null,
        proGateRequired: false,
      });
    } catch (err: unknown) {
      if (err instanceof ApiRequestError && err.status === 403) {
        setState(prev => ({
          ...prev,
          applications: [],
          isLoading: false,
          error: null,
          proGateRequired: true,
        }));
        return;
      }
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [repository]);

  const performSearch = useCallback((query: string, authorityId: AuthorityId) => {
    queryRef.current = query;
    authorityRef.current = authorityId;
    loadPage(query, authorityId, 1);
  }, [loadPage]);

  const totalPages = state.total > 0 ? Math.ceil(state.total / PAGE_SIZE) : 0;

  const goToNextPage = useCallback(() => {
    const next = state.page + 1;
    if (next <= totalPages && authorityRef.current !== null) {
      loadPage(queryRef.current, authorityRef.current, next);
    }
  }, [state.page, totalPages, loadPage]);

  const goToPreviousPage = useCallback(() => {
    const prev = state.page - 1;
    if (prev >= 1 && authorityRef.current !== null) {
      loadPage(queryRef.current, authorityRef.current, prev);
    }
  }, [state.page, loadPage]);

  return {
    applications: state.applications,
    page: state.page,
    totalPages,
    isLoading: state.isLoading,
    error: state.error,
    proGateRequired: state.proGateRequired,
    performSearch,
    goToNextPage,
    goToPreviousPage,
  };
}
