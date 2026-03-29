import { renderHook, waitFor } from '@testing-library/react';
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

    // Resolve after unmount — state should not update
    resolvePromise!('late data');

    // Give microtasks a chance to flush
    await new Promise((r) => setTimeout(r, 50));

    // After unmount, the last captured state should still show isLoading: true
    // (no state update occurred)
    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeNull();
  });
});
