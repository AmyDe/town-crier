import { useCallback } from 'react';
import type { NotificationItem, NotificationsResult } from '../../domain/types';
import type { NotificationRepository } from '../../domain/ports/notification-repository';
import { usePaginatedFetch } from '../../hooks/usePaginatedFetch';

const PAGE_SIZE = 20;

export function useNotifications(repository: NotificationRepository) {
  const fetcher = useCallback(
    (page: number) => repository.list(page, PAGE_SIZE),
    [repository],
  );

  const result = usePaginatedFetch<NotificationsResult, NotificationItem>({
    fetcher,
    pageSize: PAGE_SIZE,
    getItems: (r) => r.notifications,
    getTotal: (r) => r.total,
    getPage: (r) => r.page,
    autoLoad: true,
  });

  return {
    notifications: result.items,
    page: result.page,
    totalPages: result.totalPages,
    isLoading: result.isLoading,
    error: result.error,
    goToNextPage: result.goToNextPage,
    goToPreviousPage: result.goToPreviousPage,
  };
}
