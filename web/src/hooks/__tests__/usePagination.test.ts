import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { usePagination } from '../usePagination';

describe('usePagination', () => {
  it('calculates totalPages from total and pageSize', () => {
    const loadPage = (): void => {};

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setTotal(45);
    });

    // 45 / 20 = 2.25, ceil = 3
    expect(result.current.totalPages).toBe(3);
  });

  it('returns 0 totalPages when total is 0', () => {
    const loadPage = (): void => {};

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    expect(result.current.totalPages).toBe(0);
  });

  it('advances to next page when within bounds', () => {
    const pages: number[] = [];
    const loadPage = (page: number): void => { pages.push(page); };

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setTotal(40);
    });

    act(() => {
      result.current.goToNextPage();
    });

    expect(pages).toEqual([2]);
  });

  it('does not advance past the last page', () => {
    const pages: number[] = [];
    const loadPage = (page: number): void => { pages.push(page); };

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setTotal(20); // 1 page total
    });

    act(() => {
      result.current.goToNextPage();
    });

    expect(pages).toEqual([]);
  });

  it('goes to previous page when above page 1', () => {
    const pages: number[] = [];
    const loadPage = (page: number): void => { pages.push(page); };

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setTotal(60);
    });

    // Go to page 2
    act(() => {
      result.current.goToNextPage();
    });

    // Set page to 2 (simulating that loadPage succeeded)
    act(() => {
      result.current.setPage(2);
    });

    // Now go back
    act(() => {
      result.current.goToPreviousPage();
    });

    expect(pages).toEqual([2, 1]);
  });

  it('does not go below page 1', () => {
    const pages: number[] = [];
    const loadPage = (page: number): void => { pages.push(page); };

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setTotal(40);
    });

    act(() => {
      result.current.goToPreviousPage();
    });

    expect(pages).toEqual([]);
  });

  it('starts on page 1', () => {
    const loadPage = (): void => {};

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    expect(result.current.page).toBe(1);
  });

  it('updates page via setPage', () => {
    const loadPage = (): void => {};

    const { result } = renderHook(() => usePagination({ loadPage, pageSize: 20 }));

    act(() => {
      result.current.setPage(3);
    });

    expect(result.current.page).toBe(3);
  });
});
