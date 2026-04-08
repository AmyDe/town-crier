import { useState, useEffect, useCallback, useRef } from 'react';
import type { NotificationItem } from '../../domain/types';
import type { NotificationRepository } from '../../domain/ports/notification-repository';
import { usePagination } from '../../hooks/usePagination';

const PAGE_SIZE = 20;

interface NotificationsState {
  notifications: readonly NotificationItem[];
  isLoading: boolean;
  error: string | null;
}

export function useNotifications(repository: NotificationRepository) {
  const [state, setState] = useState<NotificationsState>({
    notifications: [],
    isLoading: true,
    error: null,
  });

  const paginationRef = useRef<ReturnType<typeof usePagination>>(null!);

  const loadPage = useCallback(async (page: number) => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const result = await repository.list(page, PAGE_SIZE);
      setState({
        notifications: result.notifications,
        isLoading: false,
        error: null,
      });
      paginationRef.current.setTotal(result.total);
      paginationRef.current.setPage(result.page);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [repository]);

  const pagination = usePagination({ loadPage, pageSize: PAGE_SIZE });

  useEffect(() => {
    paginationRef.current = pagination;
  });

  useEffect(() => {
    let cancelled = false;
    repository.list(1, PAGE_SIZE).then(result => {
      if (!cancelled) {
        setState({
          notifications: result.notifications,
          isLoading: false,
          error: null,
        });
        paginationRef.current.setTotal(result.total);
        paginationRef.current.setPage(result.page);
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'An error occurred';
        setState(prev => ({ ...prev, isLoading: false, error: message }));
      }
    });
    return () => { cancelled = true; };
  }, [repository]);

  return {
    notifications: state.notifications,
    page: pagination.page,
    totalPages: pagination.totalPages,
    isLoading: state.isLoading,
    error: state.error,
    goToNextPage: pagination.goToNextPage,
    goToPreviousPage: pagination.goToPreviousPage,
  };
}
