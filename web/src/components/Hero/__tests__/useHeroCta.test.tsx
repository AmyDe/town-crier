import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { AuthProvider } from '../../../auth/auth-context';
import { SpyAuthPort } from '../../../auth/__tests__/spies/spy-auth-port';
import { useHeroCta } from '../useHeroCta';

function createWrapper(spy: SpyAuthPort) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <AuthProvider value={spy}>{children}</AuthProvider>;
  };
}

describe('useHeroCta', () => {
  it('returns a handleTryWebApp function', () => {
    const spy = new SpyAuthPort();

    const { result } = renderHook(() => useHeroCta(), {
      wrapper: createWrapper(spy),
    });

    expect(result.current.handleTryWebApp).toBeInstanceOf(Function);
  });

  it('calls loginWithRedirect when handleTryWebApp is invoked', async () => {
    const spy = new SpyAuthPort();

    const { result } = renderHook(() => useHeroCta(), {
      wrapper: createWrapper(spy),
    });

    await result.current.handleTryWebApp();

    expect(spy.loginWithRedirectCalls).toBe(1);
  });
});
