import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useFetchData } from '../useFetchData';

describe('useFetchData', () => {
  it('returns data on successful fetch', async () => {
    const fetcher = async () => ({ name: 'Alice' });

    const { result } = renderHook(() => useFetchData(fetcher, []));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toEqual({ name: 'Alice' });
    expect(result.current.error).toBeNull();
  });

  it('sets error on fetch failure with Error instance', async () => {
    const fetcher = async () => {
      throw new Error('Network unavailable');
    };

    const { result } = renderHook(() => useFetchData<string>(fetcher, []));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.data).toBeNull();
  });

  it('sets fallback error message for non-Error throws', async () => {
    const fetcher = async () => {
      throw 'something went wrong';
    };

    const { result } = renderHook(() => useFetchData<string>(fetcher, []));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('An error occurred');
    expect(result.current.data).toBeNull();
  });

  it('does not update state after unmount', async () => {
    let resolvePromise: (value: string) => void;
    const fetcher = () =>
      new Promise<string>((resolve) => {
        resolvePromise = resolve;
      });

    const { result, unmount } = renderHook(() => useFetchData(fetcher, []));

    expect(result.current.isLoading).toBe(true);

    unmount();

    resolvePromise!('late data');

    await new Promise((r) => setTimeout(r, 50));

    // No state update occurs after unmount.
    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeNull();
  });

  it('refresh re-fetches and updates data', async () => {
    let callCount = 0;
    const fetcher = async () => {
      callCount += 1;
      return { count: callCount };
    };

    const { result } = renderHook(() => useFetchData(fetcher, []));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toEqual({ count: 1 });

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => {
      expect(result.current.data).toEqual({ count: 2 });
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('skips fetch when enabled is false', async () => {
    let callCount = 0;
    const fetcher = async () => {
      callCount += 1;
      return 'data';
    };

    const { result } = renderHook(() =>
      useFetchData(fetcher, [], { enabled: false }),
    );

    expect(result.current.isLoading).toBe(false);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await new Promise((r) => setTimeout(r, 50));

    expect(callCount).toBe(0);
  });

  it('refetches when deps change', async () => {
    let callCount = 0;
    const fetcher = async () => {
      callCount += 1;
      return `result-${callCount}`;
    };

    const { result, rerender } = renderHook(
      ({ id }: { id: string }) => useFetchData(fetcher, [id]),
      { initialProps: { id: 'a' } },
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toBe('result-1');

    rerender({ id: 'b' });

    await waitFor(() => {
      expect(result.current.data).toBe('result-2');
    });

    expect(result.current.isLoading).toBe(false);
  });
});
