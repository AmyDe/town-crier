import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useNotifications } from '../useNotifications';
import { SpyNotificationRepository } from './spies/spy-notification-repository';
import { aNotification, aSecondNotification, notificationsPage } from './fixtures/notification.fixtures';

describe('useNotifications', () => {
  it('loads the first page of notifications on mount', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage();

    const { result } = renderHook(() => useNotifications(spy));

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.notifications).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(result.current.page).toBe(1);
    expect(spy.listCalls).toEqual([{ page: 1, pageSize: 20 }]);
  });

  it('sets error on failed fetch', async () => {
    const spy = new SpyNotificationRepository();
    spy.listError = new Error('Network unavailable');

    const { result } = renderHook(() => useNotifications(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.notifications).toEqual([]);
  });

  it('navigates to the next page', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage(
      [aNotification()],
      40,
      1,
    );

    const { result } = renderHook(() => useNotifications(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    spy.listResult = notificationsPage(
      [aSecondNotification()],
      40,
      2,
    );

    act(() => {
      result.current.goToNextPage();
    });

    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    expect(result.current.notifications).toHaveLength(1);
    expect(result.current.notifications[0]?.applicationName).toBe('2026/0099');
  });

  it('navigates to the previous page', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage(
      [aNotification()],
      40,
      1,
    );

    const { result } = renderHook(() => useNotifications(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Go to page 2
    spy.listResult = notificationsPage(
      [aSecondNotification()],
      40,
      2,
    );

    act(() => {
      result.current.goToNextPage();
    });

    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    // Go back to page 1
    spy.listResult = notificationsPage(
      [aNotification()],
      40,
      1,
    );

    act(() => {
      result.current.goToPreviousPage();
    });

    await waitFor(() => {
      expect(result.current.page).toBe(1);
    });

    expect(result.current.notifications[0]?.applicationName).toBe('2026/0042');
  });

  it('calculates totalPages correctly', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage(
      [aNotification()],
      45,
      1,
    );

    const { result } = renderHook(() => useNotifications(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // 45 items / 20 per page = 3 pages (ceiling)
    expect(result.current.totalPages).toBe(3);
  });

  it('returns empty state when no notifications exist', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([], 0, 1);

    const { result } = renderHook(() => useNotifications(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.notifications).toEqual([]);
    expect(result.current.totalPages).toBe(0);
  });
});
