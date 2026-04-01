import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { AuthProvider } from '../../../auth/auth-context';
import { SpyAuthPort } from '../../../auth/__tests__/spies/spy-auth-port';
import { useNavbarAuth } from '../useNavbarAuth';

function createWrapper(spy: SpyAuthPort) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <AuthProvider value={spy}>{children}</AuthProvider>;
  };
}

describe('useNavbarAuth', () => {
  it('returns isAuthenticated true and no handleSignIn when user is authenticated', () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = true;

    const { result } = renderHook(() => useNavbarAuth(), {
      wrapper: createWrapper(spy),
    });

    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.handleSignIn).toBeUndefined();
  });

  it('returns isAuthenticated false and a handleSignIn function when user is not authenticated', () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = false;

    const { result } = renderHook(() => useNavbarAuth(), {
      wrapper: createWrapper(spy),
    });

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.handleSignIn).toBeInstanceOf(Function);
  });

  it('calls loginWithRedirect when handleSignIn is invoked', async () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = false;

    const { result } = renderHook(() => useNavbarAuth(), {
      wrapper: createWrapper(spy),
    });

    await result.current.handleSignIn!();

    expect(spy.loginWithRedirectCalls).toBe(1);
  });
});
