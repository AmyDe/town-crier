import { useState, useEffect, useCallback } from 'react';
import type { NotificationItem } from '../../domain/types';
import type { NotificationRepository } from '../../domain/ports/notification-repository';

const PAGE_SIZE = 20;

interface NotificationsState {
  notifications: readonly NotificationItem[];
  total: number;
  page: number;
  isLoading: boolean;
  error: string | null;
}

export function useNotifications(repository: NotificationRepository) {
  const [state, setState] = useState<NotificationsState>({
    notifications: [],
    total: 0,
    page: 1,
    isLoading: true,
    error: null,
  });

  const loadPage = useCallback(async (page: number) => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const result = await repository.list(page, PAGE_SIZE);
      setState({
        notifications: result.notifications,
        total: result.total,
        page: result.page,
        isLoading: false,
        error: null,
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [repository]);

  useEffect(() => {
    let cancelled = false;
    repository.list(1, PAGE_SIZE).then(result => {
      if (!cancelled) {
        setState({
          notifications: result.notifications,
          total: result.total,
          page: result.page,
          isLoading: false,
          error: null,
        });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'An error occurred';
        setState(prev => ({ ...prev, isLoading: false, error: message }));
      }
    });
    return () => { cancelled = true; };
  }, [repository]);

  const totalPages = state.total > 0 ? Math.ceil(state.total / PAGE_SIZE) : 0;

  const goToNextPage = useCallback(() => {
    const next = state.page + 1;
    if (next <= totalPages) {
      loadPage(next);
    }
  }, [state.page, totalPages, loadPage]);

  const goToPreviousPage = useCallback(() => {
    const prev = state.page - 1;
    if (prev >= 1) {
      loadPage(prev);
    }
  }, [state.page, loadPage]);

  return {
    notifications: state.notifications,
    page: state.page,
    totalPages,
    isLoading: state.isLoading,
    error: state.error,
    goToNextPage,
    goToPreviousPage,
  };
}
