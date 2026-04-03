import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useUserAuthorities } from '../useUserAuthorities';
import { SpyUserAuthoritiesPort } from './spies/spy-user-authorities-port';
import { cornwallAuthority, bathAuthority } from './fixtures/authority.fixtures';

describe('useUserAuthorities', () => {
  it('fetches authorities on mount', async () => {
    const spy = new SpyUserAuthoritiesPort();
    spy.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];

    const { result } = renderHook(() => useUserAuthorities(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.authorities).toHaveLength(2);
    expect(spy.fetchMyAuthoritiesCalls).toBe(1);
  });

  it('starts in loading state', () => {
    const spy = new SpyUserAuthoritiesPort();

    const { result } = renderHook(() => useUserAuthorities(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.authorities).toEqual([]);
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyUserAuthoritiesPort();
    spy.fetchMyAuthoritiesError = new Error('Network error');

    const { result } = renderHook(() => useUserAuthorities(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).not.toBeNull();
    expect(result.current.error?.message).toBe('Network error');
    expect(result.current.authorities).toEqual([]);
  });
});
