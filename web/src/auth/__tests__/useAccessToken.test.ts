import { renderHook } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useAccessToken } from '../useAccessToken';

const mockGetAccessTokenSilently = vi.fn<() => Promise<string | undefined>>();

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({
    getAccessTokenSilently: mockGetAccessTokenSilently,
  }),
}));

describe('useAccessToken', () => {
  beforeEach(() => {
    mockGetAccessTokenSilently.mockReset();
  });

  it('resolves with the token from Auth0', async () => {
    mockGetAccessTokenSilently.mockResolvedValue('token-123');
    const { result } = renderHook(() => useAccessToken());

    await expect(result.current()).resolves.toBe('token-123');
  });

  it('rejects when Auth0 returns no token', async () => {
    mockGetAccessTokenSilently.mockResolvedValue(undefined);
    const { result } = renderHook(() => useAccessToken());

    await expect(result.current()).rejects.toThrow('No access token');
  });

  it('propagates Auth0 errors', async () => {
    mockGetAccessTokenSilently.mockRejectedValue(new Error('login_required'));
    const { result } = renderHook(() => useAccessToken());

    await expect(result.current()).rejects.toThrow('login_required');
  });

  it('returns a stable function across renders', () => {
    const { result, rerender } = renderHook(() => useAccessToken());
    const first = result.current;

    rerender();

    expect(result.current).toBe(first);
  });
});
