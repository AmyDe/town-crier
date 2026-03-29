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
});
