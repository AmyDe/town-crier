import { useState, useCallback, useRef, useEffect } from 'react';
import { usePagination } from './usePagination';
import { extractErrorMessage } from '../utils/extractErrorMessage';

/**
 * Return value from onError:
 * - 'default': fall through to the default error handling (sets error message)
 * - 'handled' or any other string: error was handled by the callback, do not set error
 */
type OnErrorResult = 'default' | 'handled' | string;

interface UsePaginatedFetchOptions<TResult> {
  /** Async function that fetches a page of data */
  fetcher: (page: number) => Promise<TResult>;
  /** Number of items per page */
  pageSize: number;
  /** Extract the items array from the fetch result */
  getItems: (result: TResult) => readonly unknown[];
  /** Extract the total count from the fetch result */
  getTotal: (result: TResult) => number;
  /** Extract the current page number from the fetch result */
  getPage: (result: TResult) => number;
  /** Whether to automatically load the first page on mount */
  autoLoad?: boolean;
  /** Optional error handler. Return 'default' to fall through to default error handling. */
  onError?: (err: unknown) => OnErrorResult | void;
}

interface UsePaginatedFetchResult<TItem> {
  items: readonly TItem[];
  page: number;
  totalPages: number;
  isLoading: boolean;
  error: string | null;
  loadPage: (page: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
}

interface PaginatedState<TItem> {
  items: readonly TItem[];
  isLoading: boolean;
  error: string | null;
}

export function usePaginatedFetch<TResult, TItem = unknown>(
  options: UsePaginatedFetchOptions<TResult> & { getItems: (result: TResult) => readonly TItem[] },
): UsePaginatedFetchResult<TItem> {
  const { fetcher, pageSize, getItems, getTotal, getPage, autoLoad = false, onError } = options;

  const [state, setState] = useState<PaginatedState<TItem>>({
    items: [],
    isLoading: autoLoad,
    error: null,
  });

  const paginationRef = useRef<ReturnType<typeof usePagination>>(null!);

  const fetchPage = useCallback(async (page: number) => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const result = await fetcher(page);
      setState({
        items: getItems(result),
        isLoading: false,
        error: null,
      });
      paginationRef.current.setTotal(getTotal(result));
      paginationRef.current.setPage(getPage(result));
    } catch (err: unknown) {
      if (onError) {
        const onErrorResult = onError(err);
        if (onErrorResult !== 'default') {
          setState(prev => ({
            ...prev,
            isLoading: false,
          }));
          return;
        }
      }
      const message = extractErrorMessage(err);
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [fetcher, getItems, getTotal, getPage, onError]);

  const loadPageForPagination = useCallback((page: number) => {
    fetchPage(page);
  }, [fetchPage]);

  const pagination = usePagination({ loadPage: loadPageForPagination, pageSize });

  useEffect(() => {
    paginationRef.current = pagination;
  });

  useEffect(() => {
    if (!autoLoad) return;
    let cancelled = false;
    (async () => {
      try {
        const result = await fetcher(1);
        if (!cancelled) {
          setState({
            items: getItems(result),
            isLoading: false,
            error: null,
          });
          paginationRef.current.setTotal(getTotal(result));
          paginationRef.current.setPage(getPage(result));
        }
      } catch (err: unknown) {
        if (!cancelled) {
          if (onError) {
            const onErrorResult = onError(err);
            if (onErrorResult !== 'default') {
              setState(prev => ({
                ...prev,
                isLoading: false,
              }));
              return;
            }
          }
          const message = extractErrorMessage(err);
          setState(prev => ({ ...prev, isLoading: false, error: message }));
        }
      }
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoLoad]);

  return {
    items: state.items,
    page: pagination.page,
    totalPages: pagination.totalPages,
    isLoading: state.isLoading,
    error: state.error,
    loadPage: fetchPage,
    goToNextPage: pagination.goToNextPage,
    goToPreviousPage: pagination.goToPreviousPage,
  };
}
