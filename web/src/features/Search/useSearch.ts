import { useState, useCallback, useRef, useEffect } from 'react';
import type { AuthorityId, PlanningApplicationSummary } from '../../domain/types';
import type { SearchRepository } from '../../domain/ports/search-repository';
import { ApiRequestError } from '../../api/client';
import { usePagination } from '../../hooks/usePagination';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

const PAGE_SIZE = 20;

interface SearchState {
  applications: readonly PlanningApplicationSummary[];
  isLoading: boolean;
  error: string | null;
  proGateRequired: boolean;
}

export function useSearch(repository: SearchRepository) {
  const [state, setState] = useState<SearchState>({
    applications: [],
    isLoading: false,
    error: null,
    proGateRequired: false,
  });

  const queryRef = useRef('');
  const authorityRef = useRef<AuthorityId | null>(null);
  const paginationRef = useRef<ReturnType<typeof usePagination>>(null!);

  const fetchPage = useCallback(async (query: string, authorityId: AuthorityId, page: number) => {
    setState(prev => ({ ...prev, isLoading: true, error: null, proGateRequired: false }));
    try {
      const result = await repository.search(query, authorityId, page);
      setState({
        applications: result.applications,
        isLoading: false,
        error: null,
        proGateRequired: false,
      });
      paginationRef.current.setTotal(result.total);
      paginationRef.current.setPage(result.page);
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
      const message = extractErrorMessage(err);
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [repository]);

  const loadPage = useCallback((page: number) => {
    if (authorityRef.current !== null) {
      fetchPage(queryRef.current, authorityRef.current, page);
    }
  }, [fetchPage]);

  const pagination = usePagination({ loadPage, pageSize: PAGE_SIZE });

  useEffect(() => {
    paginationRef.current = pagination;
  });

  const performSearch = useCallback((query: string, authorityId: AuthorityId) => {
    queryRef.current = query;
    authorityRef.current = authorityId;
    fetchPage(query, authorityId, 1);
  }, [fetchPage]);

  return {
    applications: state.applications,
    page: pagination.page,
    totalPages: pagination.totalPages,
    isLoading: state.isLoading,
    error: state.error,
    proGateRequired: state.proGateRequired,
    performSearch,
    goToNextPage: pagination.goToNextPage,
    goToPreviousPage: pagination.goToPreviousPage,
  };
}
