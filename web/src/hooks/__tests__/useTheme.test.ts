import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useTheme } from '../useTheme';

type Theme = 'light' | 'dark';

function stubMatchMedia(prefersDark: boolean): void {
  const mediaQueryList: MediaQueryList = {
    matches: prefersDark,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  };

  window.matchMedia = (() => mediaQueryList) as typeof window.matchMedia;
}

describe('useTheme', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia(false);
  });

  it('defaults to light when system prefers light and no localStorage value', () => {
    stubMatchMedia(false);

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('light' satisfies Theme);
  });

  it('defaults to dark when system prefers dark and no localStorage value', () => {
    stubMatchMedia(true);

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('dark' satisfies Theme);
  });

  it('uses localStorage value over system preference', () => {
    stubMatchMedia(true);
    window.localStorage.setItem('tc-theme', 'light');

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('light' satisfies Theme);
  });

  it('reads dark from localStorage when system prefers light', () => {
    stubMatchMedia(false);
    window.localStorage.setItem('tc-theme', 'dark');

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('dark' satisfies Theme);
  });

  it('toggleTheme switches from light to dark', () => {
    stubMatchMedia(false);

    const { result } = renderHook(() => useTheme());

    act(() => {
      result.current.toggleTheme();
    });

    expect(result.current.theme).toBe('dark' satisfies Theme);
  });

  it('toggleTheme switches from dark to light', () => {
    stubMatchMedia(true);

    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('dark' satisfies Theme);

    act(() => {
      result.current.toggleTheme();
    });

    expect(result.current.theme).toBe('light' satisfies Theme);
  });

  it('toggleTheme persists choice to localStorage', () => {
    stubMatchMedia(false);

    const { result } = renderHook(() => useTheme());

    act(() => {
      result.current.toggleTheme();
    });

    expect(window.localStorage.getItem('tc-theme')).toBe('dark');
  });

  it('sets data-theme attribute on document.documentElement on mount', () => {
    stubMatchMedia(true);

    renderHook(() => useTheme());

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('updates data-theme attribute when toggled', () => {
    stubMatchMedia(false);

    const { result } = renderHook(() => useTheme());
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    act(() => {
      result.current.toggleTheme();
    });

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });
});
