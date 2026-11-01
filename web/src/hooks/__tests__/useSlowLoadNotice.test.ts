import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { useSlowLoadNotice } from '../useSlowLoadNotice';

describe('useSlowLoadNotice', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('is false before the delay and true after it', () => {
    const { result } = renderHook(() => useSlowLoadNotice(true, true));

    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(2999);
    });
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(true);
  });

  it('is false again after loading ends', () => {
    const { result, rerender } = renderHook(
      ({ loading }) => useSlowLoadNotice(loading, true),
      { initialProps: { loading: true } },
    );
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(result.current).toBe(true);

    rerender({ loading: false });

    expect(result.current).toBe(false);
  });

  it('never shows for a small zone', () => {
    const { result } = renderHook(() => useSlowLoadNotice(true, false));

    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    expect(result.current).toBe(false);
  });

  it('never shows when the load finishes before the delay', () => {
    const { result, rerender } = renderHook(
      ({ loading }) => useSlowLoadNotice(loading, true),
      { initialProps: { loading: true } },
    );
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    rerender({ loading: false });
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current).toBe(false);
  });

  it('starts a fresh delay when a new load begins after the previous one ended', () => {
    const { result, rerender } = renderHook(
      ({ loading }) => useSlowLoadNotice(loading, true),
      { initialProps: { loading: true } },
    );
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    rerender({ loading: false });
    rerender({ loading: true });
    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current).toBe(true);
  });

  it('restarts the delay when the zone changes from small to large mid-load', () => {
    const { result, rerender } = renderHook(
      ({ large }) => useSlowLoadNotice(true, large),
      { initialProps: { large: false } },
    );
    act(() => {
      vi.advanceTimersByTime(2500);
    });
    rerender({ large: true });
    act(() => {
      vi.advanceTimersByTime(2999);
    });
    expect(result.current).toBe(false);

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(true);
  });

  it('honours a custom delay', () => {
    const { result } = renderHook(() => useSlowLoadNotice(true, true, 500));

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(result.current).toBe(true);
  });

  it('clears its timer on unmount', () => {
    const { unmount } = renderHook(() => useSlowLoadNotice(true, true));

    unmount();

    expect(vi.getTimerCount()).toBe(0);
  });
});
