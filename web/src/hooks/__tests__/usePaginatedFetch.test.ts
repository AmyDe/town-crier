import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { usePaginatedFetch } from '../usePaginatedFetch';

interface TestItem {
  id: string;
  name: string;
}

interface TestResult {
  items: readonly TestItem[];
  total: number;
  page: number;
}

class SpyFetcher {
  calls: number[] = [];
  result: TestResult = { items: [], total: 0, page: 1 };
  error: Error | null = null;

  fetch = async (page: number): Promise<TestResult> => {
    this.calls.push(page);
    if (this.error) {
      throw this.error;
    }
    return this.result;
  };
}

function anItem(overrides?: Partial<TestItem>): TestItem {
  return { id: 'item-1', name: 'First Item', ...overrides };
}

function aSecondItem(overrides?: Partial<TestItem>): TestItem {
  return { id: 'item-2', name: 'Second Item', ...overrides };
}

describe('usePaginatedFetch', () => {
  it('returns initial empty state when autoLoad is false', () => {
    const spy = new SpyFetcher();

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
      }),
    );

    expect(result.current.items).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.page).toBe(1);
    expect(result.current.totalPages).toBe(0);
    expect(spy.calls).toEqual([]);
  });

  it('auto-loads first page on mount when autoLoad is true', async () => {
    const spy = new SpyFetcher();
    spy.result = { items: [anItem(), aSecondItem()], total: 2, page: 1 };

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
      }),
    );

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.items).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.calls).toEqual([1]);
  });

  it('loads a page via loadPage and populates items', async () => {
    const spy = new SpyFetcher();
    spy.result = { items: [anItem(), aSecondItem()], total: 2, page: 1 };

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
      }),
    );

    await act(async () => {
      result.current.loadPage(1);
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.items).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.calls).toEqual([1]);
  });

  it('sets error on failed fetch', async () => {
    const spy = new SpyFetcher();
    spy.error = new Error('Network unavailable');

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.items).toEqual([]);
  });

  it('navigates to the next page', async () => {
    const spy = new SpyFetcher();
    spy.result = { items: [anItem()], total: 40, page: 1 };

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    spy.result = { items: [aSecondItem()], total: 40, page: 2 };

    act(() => {
      result.current.goToNextPage();
    });

    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    expect(result.current.items).toHaveLength(1);
    expect(result.current.items[0]?.name).toBe('Second Item');
  });

  it('navigates to the previous page', async () => {
    const spy = new SpyFetcher();
    spy.result = { items: [anItem()], total: 40, page: 1 };

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Go to page 2
    spy.result = { items: [aSecondItem()], total: 40, page: 2 };
    act(() => {
      result.current.goToNextPage();
    });
    await waitFor(() => {
      expect(result.current.page).toBe(2);
    });

    // Go back to page 1
    spy.result = { items: [anItem()], total: 40, page: 1 };
    act(() => {
      result.current.goToPreviousPage();
    });
    await waitFor(() => {
      expect(result.current.page).toBe(1);
    });

    expect(result.current.items[0]?.name).toBe('First Item');
  });

  it('calculates totalPages correctly', async () => {
    const spy = new SpyFetcher();
    spy.result = { items: [anItem()], total: 45, page: 1 };

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // 45 items / 20 per page = 3 pages (ceiling)
    expect(result.current.totalPages).toBe(3);
  });

  it('calls onError callback when fetch fails', async () => {
    const spy = new SpyFetcher();
    const apiError = new Error('Network unavailable');
    spy.error = apiError;

    const errorsCaught: unknown[] = [];

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
        onError: (err) => { errorsCaught.push(err); },
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(errorsCaught).toHaveLength(1);
    expect(errorsCaught[0]).toBe(apiError);
  });

  it('does not set error when onError handles it and returns a string', async () => {
    const spy = new SpyFetcher();
    spy.error = new Error('Custom handled');

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
        onError: () => 'handled',
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // onError returned 'handled' which is not 'default', so error should be null
    expect(result.current.error).toBeNull();
  });

  it('sets default error when onError returns "default"', async () => {
    const spy = new SpyFetcher();
    spy.error = new Error('Fallthrough error');

    const { result } = renderHook(() =>
      usePaginatedFetch({
        fetcher: spy.fetch,
        pageSize: 20,
        getItems: (r: TestResult) => r.items,
        getTotal: (r: TestResult) => r.total,
        getPage: (r: TestResult) => r.page,
        autoLoad: true,
        onError: () => 'default',
      }),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Fallthrough error');
  });
});
