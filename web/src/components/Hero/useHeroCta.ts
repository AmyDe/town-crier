import { useCallback } from 'react';
import { useAuth } from '../../auth/auth-context';

interface HeroCtaState {
  readonly handleTryWebApp: () => Promise<void>;
}

export function useHeroCta(): HeroCtaState {
  const { loginWithRedirect } = useAuth();

  const handleTryWebApp = useCallback(async () => {
    await loginWithRedirect();
  }, [loginWithRedirect]);

  return { handleTryWebApp };
}
